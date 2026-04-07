# P2P Chat — Plan 2: Redis Layer

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Redis to the Go server for presence (online/offline), typing indicators, rate limiting, and cross-instance pub/sub message routing — enabling horizontal scaling across multiple server instances.

**Architecture:** Redis 7 accessed via `go-redis/v9`. Each connected user's presence stored as a Redis key with TTL refreshed by WebSocket heartbeats. Typing state stored as ephemeral Redis keys (TTL 5s). Cross-instance routing: when sender and receiver are on different server instances, the sending instance publishes to `chat:user:{receiver_id}` channel; each instance subscribes for all its connected users. Rate limiting via Redis atomic INCR with TTL (sliding window per user).

**Prerequisite:** Plan 1 complete and passing.

**Tech Stack:** Redis 7 (docker-compose), `github.com/redis/go-redis/v9`

---

## File Map

```
server/
├── go.mod                         # add go-redis/v9
├── config/
│   └── config.go                  # add RedisURL field
├── presence/
│   ├── presence.go                # SetOnline, SetOffline, IsOnline, GetPresence (batch)
│   ├── presence_test.go
│   ├── typing.go                  # SetTyping, fan-out typing events to group/chat members
│   └── typing_test.go
├── ratelimit/
│   ├── limiter.go                 # Allow(userID) bool — Redis INCR sliding window
│   └── limiter_test.go
├── pubsub/
│   ├── router.go                  # Publisher + Subscriber — cross-instance routing
│   └── router_test.go
├── ws/
│   └── handler.go                 # modified: call presence.SetOnline on connect, SetOffline on disconnect
└── main.go                        # modified: init Redis, wire presence + pubsub into dispatcher
```

---

## Task 1: Add Redis to docker-compose + Config

**Files:**
- Modify: `docker-compose.yml`
- Modify: `server/config/config.go`
- Modify: `.env.example`

- [ ] **Step 1: Add Redis service to docker-compose.yml**

```yaml
version: "3.9"
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: chat
      POSTGRES_PASSWORD: chat
      POSTGRES_DB: chatdb
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    command: redis-server --save "" --appendonly no

volumes:
  pgdata:
```

- [ ] **Step 2: Update `server/config/config.go` to add RedisURL**

```go
package config

import "os"

type Config struct {
	Port        string
	DatabaseURL string
	RedisURL    string
	JWTSecret   string
	Env         string
}

func Load() Config {
	return Config{
		Port:        getEnv("PORT", "8080"),
		DatabaseURL: mustEnv("DATABASE_URL"),
		RedisURL:    getEnv("REDIS_URL", "redis://localhost:6379"),
		JWTSecret:   mustEnv("JWT_SECRET"),
		Env:         getEnv("ENV", "development"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic("required env var not set: " + key)
	}
	return v
}
```

- [ ] **Step 3: Update `.env.example`**

```
PORT=8080
DATABASE_URL=postgres://chat:chat@localhost:5432/chatdb?sslmode=disable
REDIS_URL=redis://localhost:6379
JWT_SECRET=change_me_to_a_random_32_char_string
ENV=development
```

- [ ] **Step 4: Add go-redis dependency**

```bash
cd server
go get github.com/redis/go-redis/v9@v9.5.1
```

- [ ] **Step 5: Start Redis and verify**

```bash
docker-compose up -d redis
docker-compose exec redis redis-cli ping
```

Expected: `PONG`

- [ ] **Step 6: Commit**

```bash
git add docker-compose.yml server/config/config.go .env.example server/go.mod server/go.sum
git commit -m "feat: add Redis to docker-compose and config"
```

---

## Task 2: Presence (Online/Offline)

**Files:**
- Create: `server/presence/presence.go`
- Create: `server/presence/presence_test.go`

- [ ] **Step 1: Write failing tests**

Create `server/presence/presence_test.go`:

```go
package presence_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/samarth080/peer-chat/presence"
	"github.com/stretchr/testify/require"
)

func testRedis(t *testing.T) *redis.Client {
	t.Helper()
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	require.NoError(t, rdb.Ping(context.Background()).Err())
	t.Cleanup(func() { rdb.Close() })
	return rdb
}

func TestSetAndIsOnline(t *testing.T) {
	rdb := testRedis(t)
	ctx := context.Background()
	id := uuid.New()

	presence.SetOnline(ctx, rdb, id)
	require.True(t, presence.IsOnline(ctx, rdb, id))

	presence.SetOffline(ctx, rdb, id)
	require.False(t, presence.IsOnline(ctx, rdb, id))
}

func TestSetOnline_TTLRefresh(t *testing.T) {
	rdb := testRedis(t)
	ctx := context.Background()
	id := uuid.New()

	presence.SetOnline(ctx, rdb, id)
	ttl := rdb.TTL(ctx, "presence:"+id.String()).Val()
	require.Greater(t, ttl, 25*time.Second)
	require.LessOrEqual(t, ttl, 31*time.Second)
}

func TestGetPresenceBatch(t *testing.T) {
	rdb := testRedis(t)
	ctx := context.Background()

	onlineID := uuid.New()
	offlineID := uuid.New()
	presence.SetOnline(ctx, rdb, onlineID)

	results := presence.GetPresenceBatch(ctx, rdb, []uuid.UUID{onlineID, offlineID})
	require.True(t, results[onlineID])
	require.False(t, results[offlineID])

	presence.SetOffline(ctx, rdb, onlineID)
}
```

- [ ] **Step 2: Run to confirm compile failure**

```bash
go test ./presence/... -v
```

Expected: compile error — `presence` package not found.

- [ ] **Step 3: Create `server/presence/presence.go`**

```go
package presence

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const ttl = 30 * time.Second

func key(userID uuid.UUID) string {
	return fmt.Sprintf("presence:%s", userID)
}

// SetOnline marks a user as online with a 30s TTL (refreshed by heartbeat)
func SetOnline(ctx context.Context, rdb *redis.Client, userID uuid.UUID) {
	rdb.Set(ctx, key(userID), "online", ttl)
}

// SetOffline removes a user's presence key immediately
func SetOffline(ctx context.Context, rdb *redis.Client, userID uuid.UUID) {
	rdb.Del(ctx, key(userID))
}

// IsOnline returns true if the user has an active presence key
func IsOnline(ctx context.Context, rdb *redis.Client, userID uuid.UUID) bool {
	val, err := rdb.Exists(ctx, key(userID)).Result()
	return err == nil && val > 0
}

// GetPresenceBatch returns a map of userID -> isOnline for a slice of user IDs
func GetPresenceBatch(ctx context.Context, rdb *redis.Client, userIDs []uuid.UUID) map[uuid.UUID]bool {
	if len(userIDs) == 0 {
		return nil
	}

	keys := make([]string, len(userIDs))
	for i, id := range userIDs {
		keys[i] = key(id)
	}

	vals, err := rdb.MGet(ctx, keys...).Result()
	result := make(map[uuid.UUID]bool, len(userIDs))
	for i, id := range userIDs {
		if err == nil && vals[i] != nil {
			result[id] = true
		} else {
			result[id] = false
		}
	}
	return result
}
```

- [ ] **Step 4: Run tests to confirm they pass**

```bash
go test ./presence/... -run TestSet -v
go test ./presence/... -run TestGetPresenceBatch -v
```

Expected: `PASS` for all presence tests.

- [ ] **Step 5: Commit**

```bash
git add server/presence/presence.go server/presence/presence_test.go
git commit -m "feat: Redis presence — SetOnline, SetOffline, batch query"
```

---

## Task 3: Typing Indicators

**Files:**
- Create: `server/presence/typing.go`
- Create: `server/presence/typing_test.go`

- [ ] **Step 1: Write failing test**

Create `server/presence/typing_test.go`:

```go
package presence_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/samarth080/peer-chat/presence"
	"github.com/samarth080/peer-chat/ws"
	"github.com/stretchr/testify/require"
)

func TestSetTyping_FanOut(t *testing.T) {
	rdb := testRedis(t)
	ctx := context.Background()

	senderID := uuid.New()
	receiverID := uuid.New()
	chatID := uuid.New()

	hub := ws.NewHub()
	recvClient := ws.NewTestClient(receiverID, "bob")
	hub.Register(recvClient)

	presence.SetTyping(ctx, rdb, hub, chatID, senderID, "alice", []uuid.UUID{senderID, receiverID})

	msg := <-recvClient.Send()
	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(msg, &out))
	require.Equal(t, "typing", out["type"])
	require.Equal(t, "alice", out["from"])
}
```

- [ ] **Step 2: Run to confirm compile failure**

```bash
go test ./presence/... -run TestSetTyping -v
```

Expected: compile error — `presence.SetTyping` not defined.

- [ ] **Step 3: Write `server/presence/typing.go`**

```go
package presence

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/samarth080/peer-chat/ws"
)

const typingTTL = 5 * time.Second

func typingKey(chatID, userID uuid.UUID) string {
	return fmt.Sprintf("typing:%s:%s", chatID, userID)
}

// SetTyping records that userID is typing in chatID and fans out to online members
func SetTyping(ctx context.Context, rdb *redis.Client, hub *ws.Hub, chatID, senderID uuid.UUID, senderName string, memberIDs []uuid.UUID) {
	rdb.Set(ctx, typingKey(chatID, senderID), "1", typingTTL)

	payload, _ := json.Marshal(map[string]interface{}{
		"type":    "typing",
		"from":    senderName,
		"from_id": senderID,
		"chat_id": chatID,
	})

	for _, memberID := range memberIDs {
		if memberID != senderID {
			hub.Send(memberID, payload)
		}
	}
}
```

- [ ] **Step 4: Run tests to confirm they pass**

```bash
go test ./presence/... -v
```

Expected: `PASS` for all presence + typing tests.

- [ ] **Step 5: Commit**

```bash
git add server/presence/typing.go server/presence/typing_test.go
git commit -m "feat: typing indicators — Redis TTL + hub fan-out"
```

---

## Task 4: Rate Limiting

**Files:**
- Create: `server/ratelimit/limiter.go`
- Create: `server/ratelimit/limiter_test.go`

- [ ] **Step 1: Write failing test**

Create `server/ratelimit/limiter_test.go`:

```go
package ratelimit_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/samarth080/peer-chat/ratelimit"
	"github.com/stretchr/testify/require"
)

func testRedis(t *testing.T) *redis.Client {
	t.Helper()
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	require.NoError(t, rdb.Ping(context.Background()).Err())
	t.Cleanup(func() { rdb.Close() })
	return rdb
}

func TestAllow_UnderLimit(t *testing.T) {
	rdb := testRedis(t)
	ctx := context.Background()
	id := uuid.New()
	limiter := ratelimit.New(rdb, 5, 60)

	for i := 0; i < 5; i++ {
		require.True(t, limiter.Allow(ctx, id), "expected allow on call %d", i+1)
	}

	// cleanup
	rdb.Del(ctx, "rate:"+id.String())
}

func TestAllow_OverLimit(t *testing.T) {
	rdb := testRedis(t)
	ctx := context.Background()
	id := uuid.New()
	limiter := ratelimit.New(rdb, 3, 60)

	for i := 0; i < 3; i++ {
		limiter.Allow(ctx, id)
	}
	require.False(t, limiter.Allow(ctx, id))

	rdb.Del(ctx, "rate:"+id.String())
}
```

- [ ] **Step 2: Run to confirm compile failure**

```bash
go test ./ratelimit/... -v
```

Expected: compile error — `ratelimit` package not defined.

- [ ] **Step 3: Create `server/ratelimit/limiter.go`**

```go
package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type Limiter struct {
	rdb     *redis.Client
	max     int64
	windowS int
}

func New(rdb *redis.Client, max int64, windowSeconds int) *Limiter {
	return &Limiter{rdb: rdb, max: max, windowS: windowSeconds}
}

func (l *Limiter) Allow(ctx context.Context, userID uuid.UUID) bool {
	key := fmt.Sprintf("rate:%s", userID)
	pipe := l.rdb.Pipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, time.Duration(l.windowS)*time.Second)
	pipe.Exec(ctx)

	return incr.Val() <= l.max
}
```

- [ ] **Step 4: Run tests to confirm they pass**

```bash
go test ./ratelimit/... -v
```

Expected: `PASS`.

- [ ] **Step 5: Commit**

```bash
git add server/ratelimit/
git commit -m "feat: Redis rate limiter — sliding window per user"
```

---

## Task 5: Cross-Instance Pub/Sub Router

**Files:**
- Create: `server/pubsub/router.go`
- Create: `server/pubsub/router_test.go`

- [ ] **Step 1: Write failing test**

Create `server/pubsub/router_test.go`:

```go
package pubsub_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/samarth080/peer-chat/pubsub"
	"github.com/samarth080/peer-chat/ws"
	"github.com/stretchr/testify/require"
)

func testRedis(t *testing.T) *redis.Client {
	t.Helper()
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	require.NoError(t, rdb.Ping(context.Background()).Err())
	t.Cleanup(func() { rdb.Close() })
	return rdb
}

func TestPublishAndDeliver(t *testing.T) {
	rdb := testRedis(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	hub := ws.NewHub()
	receiverID := uuid.New()
	client := ws.NewTestClient(receiverID, "alice")
	hub.Register(client)

	router := pubsub.NewRouter(rdb, hub)
	router.Subscribe(ctx, receiverID)

	msg := []byte(`{"type":"message","body":"hello from other instance"}`)
	pubsub.Publish(ctx, rdb, receiverID, msg)

	select {
	case received := <-client.Send():
		require.Equal(t, msg, received)
	case <-ctx.Done():
		t.Fatal("timed out waiting for message delivery via pub/sub")
	}

	router.Unsubscribe(receiverID)
}
```

- [ ] **Step 2: Run to confirm compile failure**

```bash
go test ./pubsub/... -v
```

Expected: compile error — `pubsub` package not defined.

- [ ] **Step 3: Write `server/pubsub/router.go`**

```go
package pubsub

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/samarth080/peer-chat/ws"
)

func channelName(userID uuid.UUID) string {
	return fmt.Sprintf("chat:user:%s", userID)
}

// Publish sends a message to a user's Redis pub/sub channel
func Publish(ctx context.Context, rdb *redis.Client, userID uuid.UUID, data []byte) {
	rdb.Publish(ctx, channelName(userID), data)
}

// Router subscribes to Redis channels and delivers messages to local hub clients
type Router struct {
	rdb  *redis.Client
	hub  *ws.Hub
	mu   sync.Mutex
	subs map[uuid.UUID]*redis.PubSub
}

func NewRouter(rdb *redis.Client, hub *ws.Hub) *Router {
	return &Router{
		rdb:  rdb,
		hub:  hub,
		subs: make(map[uuid.UUID]*redis.PubSub),
	}
}

// Subscribe starts listening on the user's channel and forwards to hub
func (r *Router) Subscribe(ctx context.Context, userID uuid.UUID) {
	sub := r.rdb.Subscribe(ctx, channelName(userID))

	r.mu.Lock()
	r.subs[userID] = sub
	r.mu.Unlock()

	go func() {
		ch := sub.Channel()
		for msg := range ch {
			r.hub.Send(userID, []byte(msg.Payload))
		}
		log.Printf("pubsub: subscription ended for user %s", userID)
	}()
}

// Unsubscribe closes the Redis subscription for a user
func (r *Router) Unsubscribe(userID uuid.UUID) {
	r.mu.Lock()
	sub, ok := r.subs[userID]
	if ok {
		delete(r.subs, userID)
	}
	r.mu.Unlock()

	if ok {
		sub.Close()
	}
}
```

- [ ] **Step 4: Run tests to confirm they pass**

```bash
go test ./pubsub/... -v
```

Expected: `PASS`.

- [ ] **Step 5: Commit**

```bash
git add server/pubsub/
git commit -m "feat: Redis pub/sub router — cross-instance message routing"
```

---

## Task 6: Wire Redis into main.go + ws/handler.go

**Files:**
- Modify: `server/ws/handler.go`
- Modify: `server/main.go`

- [ ] **Step 1: Update `server/ws/handler.go` to accept presence + pubsub hooks**

Replace the `ServeWS` function to accept callbacks for connect/disconnect events:

```go
package ws

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// ConnectHook is called after a client connects (use for presence, pubsub subscribe)
type ConnectHook func(userID uuid.UUID)

// DisconnectHook is called after a client disconnects (use for presence, pubsub unsubscribe)
type DisconnectHook func(userID uuid.UUID)

// ServeWS handles GET /ws — requires JWTAuth middleware
func ServeWS(hub *Hub, dispatcher Dispatcher, onConnect, onDisconnect ConnectHook) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := c.MustGet("user_id").(uuid.UUID)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user context"})
			return
		}
		username := c.MustGet("username").(string)

		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("websocket upgrade failed: %v", err)
			return
		}

		client := newRealClient(userID, username, conn)
		hub.Register(client)

		if onConnect != nil {
			onConnect(userID)
		}

		go client.writePump(conn)
		go func() {
			client.readPump(conn, hub, dispatcher)
			if onDisconnect != nil {
				onDisconnect(userID)
			}
		}()
	}
}
```

- [ ] **Step 2: Update `server/main.go` to wire Redis presence + pubsub**

```go
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
		if err := chat.HandleDM(env.Ctx, d.pool, d.hub, d.rdb, d.router, env.SenderID, env.SenderName, env.Data); err != nil {
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
		presence.SetTyping(env.Ctx, d.rdb, d.hub, chatID, env.SenderID, env.SenderName, memberIDs)
	case "ping":
		presence.SetOnline(env.Ctx, d.rdb, env.SenderID)
		d.hub.Send(env.SenderID, []byte(`{"type":"pong"}`))
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
		presence.SetOnline(ctx, rdb, userID)
		router.Subscribe(ctx, userID)
	}
	onDisconnect := func(userID uuid.UUID) {
		presence.SetOffline(ctx, rdb, userID)
		router.Unsubscribe(userID)
	}

	r.GET("/ws", middleware.JWTAuth(cfg.JWTSecret), ws.ServeWS(hub, dispatcher, onConnect, onDisconnect))

	addr := ":" + cfg.Port
	log.Printf("server starting on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
```

- [ ] **Step 3: Update `chat/dm.go` to accept Redis router for cross-instance delivery**

Replace the `HandleDM` function signature to accept `rdb` and `router`:

```go
// HandleDM processes a direct message — inserts to DB, delivers locally or via pub/sub
func HandleDM(ctx context.Context, pool *pgxpool.Pool, hub *ws.Hub, rdb *redis.Client, router Router, senderID uuid.UUID, senderName string, raw []byte) error {
	var msg dmInbound
	if err := json.Unmarshal(raw, &msg); err != nil {
		return fmt.Errorf("invalid dm payload: %w", err)
	}

	receiverID, err := uuid.Parse(msg.To)
	if err != nil {
		return fmt.Errorf("invalid receiver id: %w", err)
	}

	chatID, err := db.FindOrCreateChat(ctx, pool, senderID, receiverID)
	if err != nil {
		return fmt.Errorf("find/create chat: %w", err)
	}

	stored, err := db.InsertMessage(ctx, pool, chatID, senderID, msg.Body)
	if err != nil {
		return fmt.Errorf("insert message: %w", err)
	}

	outbound, _ := json.Marshal(map[string]interface{}{
		"type":       "message",
		"from":       senderName,
		"from_id":    senderID,
		"body":       stored.Body,
		"message_id": stored.ID,
		"timestamp":  stored.CreatedAt,
	})

	// Try local delivery first; fall back to pub/sub for cross-instance
	if !hub.Send(receiverID, outbound) {
		router.Publish(ctx, receiverID, outbound)
	}

	ack, _ := json.Marshal(map[string]interface{}{
		"type":       "sent",
		"message_id": stored.ID,
		"id":         msg.ID,
		"status":     "sent",
	})
	hub.Send(senderID, ack)

	return nil
}
```

Also add the `Router` interface to `chat/dm.go` to avoid import cycle:

```go
// Router abstracts pub/sub delivery for cross-instance routing
type Router interface {
	Publish(ctx context.Context, userID uuid.UUID, data []byte)
}
```

And update `pubsub/router.go` to add a `Publish` method on `*Router` so it satisfies `chat.Router`:

```go
// Publish satisfies chat.Router interface
func (r *Router) Publish(ctx context.Context, userID uuid.UUID, data []byte) {
	Publish(ctx, r.rdb, userID, data)
}
```

- [ ] **Step 4: Build to confirm no compile errors**

```bash
cd server
go build ./...
```

Expected: clean build, no errors.

- [ ] **Step 5: Run full test suite**

```bash
go test ./... -v -count=1
```

Expected: all tests `PASS`.

- [ ] **Step 6: Manual smoke test — presence + typing**

```bash
# Start services
docker-compose up -d

# Set env and start server
set -a && source .env && set +a
go run ./server/main.go &

# Register two users
TOKEN_A=$(curl -s -X POST localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"alice2","password":"pass123"}' | jq -r .token)

TOKEN_B=$(curl -s -X POST localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"bob2","password":"pass123"}' | jq -r .token)

# Connect both — in two separate terminals:
wscat -c "ws://localhost:8080/ws?token=$TOKEN_A"
wscat -c "ws://localhost:8080/ws?token=$TOKEN_B"

# From alice's connection, send ping:
{"type":"ping"}
# Expected: {"type":"pong"}

# Check Redis presence:
docker-compose exec redis redis-cli keys "presence:*"
# Expected: one key per connected user
```

- [ ] **Step 7: Commit**

```bash
git add server/
git commit -m "feat: wire Redis presence, typing, pub/sub, rate limiting into server"
```

---

## What Plan 2 Delivers

After this plan, you have:
- Real-time presence (green dot online/offline) via Redis TTL + heartbeat
- Typing indicators (ephemeral, auto-expire 5s, no stop event needed)
- Rate limiting: 30 messages/60s per user via Redis INCR
- Cross-instance routing: two server instances can deliver messages to each other's connected users via Redis pub/sub
- `/health` endpoint checks both PostgreSQL and Redis

**Next:** Plan 3 adds WebRTC signaling, React frontend, Go CLI, and Fly.io deployment.
