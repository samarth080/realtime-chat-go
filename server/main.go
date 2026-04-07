package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	_ "github.com/redis/go-redis/v9"
	"github.com/samarth080/peer-chat/auth"
	"github.com/samarth080/peer-chat/chat"
	"github.com/samarth080/peer-chat/config"
	"github.com/samarth080/peer-chat/db"
	"github.com/samarth080/peer-chat/middleware"
	"github.com/samarth080/peer-chat/ws"
)

// MessageDispatcher routes inbound WebSocket messages to the correct handler
type MessageDispatcher struct {
	pool *db.Pool
	hub  *ws.Hub
}

func (d *MessageDispatcher) Dispatch(env ws.InboundEnvelope) {
	var base struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(env.Data, &base); err != nil {
		return
	}

	switch base.Type {
	case "message":
		if err := chat.HandleDM(env.Ctx, d.pool, d.hub, nil, env.SenderID, env.SenderName, env.Data); err != nil {
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
	case "ping":
		d.hub.Send(env.SenderID, []byte(`{"type":"pong"}`))
	default:
		log.Printf("unknown message type: %s", base.Type)
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

	hub := ws.NewHub()
	dispatcher := &MessageDispatcher{pool: pool, hub: hub}
	authHandler := auth.NewHandler(pool, cfg.JWTSecret)

	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		if err := pool.Ping(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unhealthy", "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.POST("/auth/register", authHandler.Register)
	r.POST("/auth/login", authHandler.Login)

	r.GET("/ws", middleware.JWTAuth(cfg.JWTSecret), ws.ServeWS(hub, dispatcher, nil, nil))

	addr := ":" + cfg.Port
	log.Printf("server starting on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
