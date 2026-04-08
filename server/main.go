package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/samarth080/peer-chat/auth"
	"github.com/samarth080/peer-chat/chat"
	"github.com/samarth080/peer-chat/config"
	"github.com/samarth080/peer-chat/db"
	"github.com/samarth080/peer-chat/middleware"
	"github.com/samarth080/peer-chat/presence"
	"github.com/samarth080/peer-chat/pubsub"
	"github.com/samarth080/peer-chat/ratelimit"
	"github.com/samarth080/peer-chat/ws"
)

type MessageDispatcher struct {
	pool    *db.Pool
	hub     *ws.Hub
	rdb     *redis.Client
	limiter *ratelimit.Limiter
	router  *pubsub.Router
}

func (d *MessageDispatcher) Dispatch(env ws.InboundEnvelope) {
	var base struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(env.Data, &base); err != nil {
		return
	}

	// Rate limit all message-sending actions
	if base.Type == "message" || base.Type == "group_message" {
		if !d.limiter.Allow(env.Ctx, env.SenderID) {
			data, _ := json.Marshal(map[string]string{"type": "error", "code": "rate_limited"})
			d.hub.Send(env.SenderID, data)
			return
		}
	}

	switch base.Type {
	case "message":
		if err := chat.HandleDM(env.Ctx, d.pool, d.hub, d.router, env.SenderID, env.SenderName, env.Data); err != nil {
			log.Printf("HandleDM error: %v", err)
		}
	case "group_message":
		if err := chat.HandleGroupMessage(env.Ctx, d.pool, d.hub, env.SenderID, env.SenderName, env.Data); err != nil {
			log.Printf("HandleGroupMessage error: %v", err)
		}
	case "create_group":
		if err := chat.HandleCreateGroup(env.Ctx, d.pool, d.hub, env.SenderID, env.Data); err != nil {
			log.Printf("HandleCreateGroup error: %v", err)
		}
	case "add_member":
		if err := chat.HandleAddMember(env.Ctx, d.pool, d.hub, env.SenderID, env.Data); err != nil {
			log.Printf("HandleAddMember error: %v", err)
		}
	case "leave_group":
		if err := chat.HandleLeaveGroup(env.Ctx, d.pool, d.hub, env.SenderID, env.Data); err != nil {
			log.Printf("HandleLeaveGroup error: %v", err)
		}
	case "ack":
		if err := chat.HandleAck(env.Ctx, d.pool, d.hub, env.SenderID, env.Data); err != nil {
			log.Printf("HandleAck error: %v", err)
		}
	case "typing":
		var payload struct {
			ChatID  string   `json:"chat_id"`
			Members []string `json:"members"`
		}
		json.Unmarshal(env.Data, &payload)
		chatID, _ := uuid.Parse(payload.ChatID)
		var memberIDs []uuid.UUID
		for _, m := range payload.Members {
			if id, err := uuid.Parse(m); err == nil {
				memberIDs = append(memberIDs, id)
			}
		}
		if err := presence.SetTyping(env.Ctx, d.rdb, d.hub, chatID, env.SenderID, env.SenderName, memberIDs); err != nil {
			log.Printf("SetTyping error: %v", err)
		}
	case "ping":
		if err := presence.SetOnline(env.Ctx, d.rdb, env.SenderID); err != nil {
			log.Printf("SetOnline error: %v", err)
		}
		d.hub.Send(env.SenderID, []byte(`{"type":"pong"}`))
	case "webrtc_offer", "webrtc_answer", "ice_candidate":
		// Relay WebRTC signaling messages directly to the target peer
		var payload struct {
			To string `json:"to"`
		}
		if err := json.Unmarshal(env.Data, &payload); err != nil {
			return
		}
		targetID, err := uuid.Parse(payload.To)
		if err != nil {
			return
		}
		// Rewrite "to" field to "from" so receiver knows who sent it
		var raw map[string]interface{}
		json.Unmarshal(env.Data, &raw)
		raw["from"] = env.SenderID
		raw["from_name"] = env.SenderName
		delete(raw, "to")
		out, _ := json.Marshal(raw)
		// Try local hub first, then pub/sub for cross-instance
		if !d.hub.Send(targetID, out) {
			if err := d.router.Publish(env.Ctx, targetID, out); err != nil {
				log.Printf("webrtc relay pub/sub error: %v", err)
			}
		}
	}
}

func main() {
	cfg := config.Load()
	ctx := context.Background()

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	opt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		log.Fatalf("invalid REDIS_URL: %v", err)
	}
	rdb := redis.NewClient(opt)
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("failed to connect to redis: %v", err)
	}
	defer rdb.Close()

	hub := ws.NewHub()
	router := pubsub.NewRouter(rdb, hub)
	limiter := ratelimit.New(rdb, 30, 60)
	dispatcher := &MessageDispatcher{pool: pool, hub: hub, rdb: rdb, limiter: limiter, router: router}
	authHandler := auth.NewHandler(pool, cfg.JWTSecret)

	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()

	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	r.GET("/health", func(c *gin.Context) {
		if err := pool.Ping(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unhealthy", "db": err.Error()})
			return
		}
		if err := rdb.Ping(c.Request.Context()).Err(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unhealthy", "redis": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.POST("/auth/register", authHandler.Register)
	r.POST("/auth/login", authHandler.Login)

	onConnect := func(userID uuid.UUID) {
		if err := presence.SetOnline(ctx, rdb, userID); err != nil {
			log.Printf("presence.SetOnline: %v", err)
		}
		router.Subscribe(ctx, userID)
	}
	onDisconnect := func(userID uuid.UUID) {
		if err := presence.SetOffline(ctx, rdb, userID); err != nil {
			log.Printf("presence.SetOffline: %v", err)
		}
		router.Unsubscribe(userID)
	}

	r.GET("/ws", middleware.JWTAuth(cfg.JWTSecret), ws.ServeWS(hub, dispatcher, onConnect, onDisconnect))

	addr := ":" + cfg.Port
	log.Printf("server starting on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
