# P2P Chat — Plan 1: Go Server Foundation

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a production-grade Go WebSocket chat server with JWT auth, PostgreSQL persistence, real-time DM + group chat — runnable locally via docker-compose, tested, and deployable as a standalone binary.

**Architecture:** Single Go binary using `gin` for HTTP/REST, `gorilla/websocket` for WebSocket connections, a mutex-guarded `Hub` for client registry and fan-out delivery, and `pgxpool` for async PostgreSQL queries. A single dispatch goroutine in `main.go` routes inbound WebSocket messages to the correct handler — avoiding circular imports between `ws` and `chat` packages. Redis (presence, pub/sub, rate limiting) is Plan 2.

**Tech Stack:** Go 1.22, `gin` v1.9, `gorilla/websocket` v1.5, `pgx/v5` (pgxpool), `golang-jwt/jwt/v5`, `bcrypt`, `testify` v1.9, PostgreSQL 16 (docker-compose for local dev)

---

## File Map

```
server/
├── go.mod
├── go.sum
├── main.go                        # entry point, router, dependency wiring, dispatch goroutine
├── config/
│   └── config.go                  # load env vars with defaults
├── db/
│   ├── db.go                      # pgxpool setup
│   ├── users.go                   # user CRUD queries + User struct
│   ├── messages.go                # message queries + Message struct, FindOrCreateChat
│   └── groups.go                  # group queries + Group/GroupMember/GroupMessage structs
├── auth/
│   ├── service.go                 # HashPassword, CheckPassword, GenerateToken, ParseToken
│   ├── service_test.go
│   ├── handler.go                 # POST /auth/register, POST /auth/login
│   └── handler_test.go
├── middleware/
│   ├── auth.go                    # JWTAuth gin middleware (header + query param)
│   └── auth_test.go
├── ws/
│   ├── hub.go                     # Hub struct, mutex-protected client map, Send, Register, Unregister
│   ├── hub_test.go
│   ├── client.go                  # Client struct, readPump, writePump
│   └── handler.go                 # ServeWS: upgrade + register + launch pumps
└── chat/
    ├── dm.go                      # HandleDM: DB insert + hub fan-out + ack
    ├── dm_test.go
    ├── group.go                   # HandleGroupMessage, HandleCreateGroup, HandleAddMember, HandleLeaveGroup, HandleGroupHistory
    └── group_test.go

migrations/
├── 000001_init.up.sql
└── 000001_init.down.sql

docker-compose.yml
.env.example
```

---

## Task 1: Project Scaffold

**Files:**
- Create: `server/go.mod`
- Create: `docker-compose.yml`
- Create: `.env.example`
- Create: `server/config/config.go`

- [ ] **Step 1: Create repo root structure**

```bash
cd /path/to/peer-to-peer-chat   # your project root
mkdir -p server/config server/db server/auth server/middleware server/ws server/chat
mkdir -p migrations
```

- [ ] **Step 2: Initialize Go module**

```bash
cd server
go mod init github.com/samarth080/peer-chat
```

- [ ] **Step 3: Add dependencies**

```bash
go get github.com/gin-gonic/gin@v1.9.1
go get github.com/gorilla/websocket@v1.5.1
go get github.com/golang-jwt/jwt/v5@v5.2.1
go get github.com/google/uuid@v1.6.0
go get github.com/jackc/pgx/v5@v5.5.5
go get github.com/stretchr/testify@v1.9.0
go get golang.org/x/crypto@v0.21.0
```

- [ ] **Step 4: Write `server/config/config.go`**

```go
package config

import "os"

type Config struct {
	Port        string
	DatabaseURL string
	JWTSecret   string
	Env         string
}

func Load() Config {
	return Config{
		Port:        getEnv("PORT", "8080"),
		DatabaseURL: mustEnv("DATABASE_URL"),
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

- [ ] **Step 5: Write `docker-compose.yml`**

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

volumes:
  pgdata:
```

- [ ] **Step 6: Write `.env.example`**

```
PORT=8080
DATABASE_URL=postgres://chat:chat@localhost:5432/chatdb?sslmode=disable
JWT_SECRET=change_me_to_a_random_32_char_string
ENV=development
```

- [ ] **Step 7: Start postgres and verify**

```bash
docker-compose up -d postgres
docker-compose ps
```

Expected output: postgres container status `Up`.

- [ ] **Step 8: Commit**

```bash
cd ..  # back to repo root
git add server/go.mod server/go.sum server/config/ docker-compose.yml .env.example
git commit -m "feat: project scaffold — go module, config, docker-compose"
```

---

## Task 2: Database Migrations

**Files:**
- Create: `migrations/000001_init.up.sql`
- Create: `migrations/000001_init.down.sql`

- [ ] **Step 1: Write `migrations/000001_init.up.sql`**

```sql
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    username      TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE chats (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE chat_members (
    chat_id UUID NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (chat_id, user_id)
);

CREATE TABLE messages (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    chat_id    UUID NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    sender_id  UUID NOT NULL REFERENCES users(id),
    body       TEXT NOT NULL,
    status     TEXT NOT NULL DEFAULT 'sent',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_messages_chat_time ON messages(chat_id, created_at DESC);
CREATE INDEX idx_messages_sender    ON messages(sender_id);

CREATE TABLE groups (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name       TEXT UNIQUE NOT NULL,
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE group_members (
    group_id  UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id   UUID NOT NULL REFERENCES users(id)  ON DELETE CASCADE,
    role      TEXT NOT NULL DEFAULT 'member',
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (group_id, user_id)
);

CREATE TABLE group_messages (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    group_id   UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    sender_id  UUID NOT NULL REFERENCES users(id),
    body       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_group_messages_group_time ON group_messages(group_id, created_at DESC);

CREATE TABLE group_read_receipts (
    message_id UUID NOT NULL REFERENCES group_messages(id) ON DELETE CASCADE,
    reader_id  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    read_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (message_id, reader_id)
);

CREATE TABLE read_receipts (
    message_id UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    reader_id  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    read_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (message_id, reader_id)
);
```

- [ ] **Step 2: Write `migrations/000001_init.down.sql`**

```sql
DROP TABLE IF EXISTS read_receipts;
DROP TABLE IF EXISTS group_read_receipts;
DROP TABLE IF EXISTS group_messages;
DROP TABLE IF EXISTS group_members;
DROP TABLE IF EXISTS groups;
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS chat_members;
DROP TABLE IF EXISTS chats;
DROP TABLE IF EXISTS users;
DROP EXTENSION IF EXISTS "uuid-ossp";
```

- [ ] **Step 3: Apply migrations**

```bash
# Install golang-migrate CLI
brew install golang-migrate  # macOS
# or: go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

migrate -path migrations -database "postgres://chat:chat@localhost:5432/chatdb?sslmode=disable" up
```

Expected output:
```
1/u 000001_init (Xms)
```

- [ ] **Step 4: Verify schema in psql**

```bash
docker exec -it $(docker-compose ps -q postgres) psql -U chat -d chatdb -c "\dt"
```

Expected: 8 tables listed (users, chats, chat_members, messages, groups, group_members, group_messages, group_read_receipts, read_receipts).

- [ ] **Step 5: Commit**

```bash
git add migrations/
git commit -m "feat: PostgreSQL schema migrations"
```

---

## Task 3: DB Connection Pool + User Queries

**Files:**
- Create: `server/db/db.go`
- Create: `server/db/users.go`

- [ ] **Step 1: Write failing test for CreateUser**

Create `server/db/users_test.go`:

```go
package db_test

import (
	"context"
	"os"
	"testing"

	"github.com/samarth080/peer-chat/db"
	"github.com/stretchr/testify/require"
)

func testPool(t *testing.T) *db.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = "postgres://chat:chat@localhost:5432/chatdb?sslmode=disable"
	}
	pool, err := db.NewPool(context.Background(), url)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })
	return pool
}

func TestCreateUser(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	user, err := db.CreateUser(ctx, pool, "alice_test_"+t.Name(), "hashed")
	require.NoError(t, err)
	require.Equal(t, "alice_test_"+t.Name(), user.Username)
	require.NotEmpty(t, user.ID)

	// cleanup
	_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", user.ID)
}

func TestGetUserByUsername(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	created, _ := db.CreateUser(ctx, pool, "bob_test_"+t.Name(), "hashed")
	t.Cleanup(func() { pool.Exec(ctx, "DELETE FROM users WHERE id = $1", created.ID) })

	found, err := db.GetUserByUsername(ctx, pool, "bob_test_"+t.Name())
	require.NoError(t, err)
	require.Equal(t, created.ID, found.ID)
}
```

- [ ] **Step 2: Run test to confirm it fails**

```bash
cd server
go test ./db/... -run TestCreateUser -v
```

Expected: compile error — `db.NewPool`, `db.CreateUser` not defined yet.

- [ ] **Step 3: Write `server/db/db.go`**

```go
package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Pool = pgxpool.Pool

func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}
```

- [ ] **Step 4: Write `server/db/users.go`**

```go
package db

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	ID           uuid.UUID
	Username     string
	PasswordHash string
	CreatedAt    time.Time
}

func CreateUser(ctx context.Context, pool *pgxpool.Pool, username, passwordHash string) (User, error) {
	var u User
	err := pool.QueryRow(ctx,
		`INSERT INTO users (username, password_hash)
		 VALUES ($1, $2)
		 RETURNING id, username, password_hash, created_at`,
		username, passwordHash,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt)
	return u, err
}

func GetUserByUsername(ctx context.Context, pool *pgxpool.Pool, username string) (User, error) {
	var u User
	err := pool.QueryRow(ctx,
		`SELECT id, username, password_hash, created_at
		 FROM users WHERE username = $1`,
		username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt)
	if err == pgx.ErrNoRows {
		return User{}, pgx.ErrNoRows
	}
	return u, err
}

func GetUserByID(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) (User, error) {
	var u User
	err := pool.QueryRow(ctx,
		`SELECT id, username, password_hash, created_at
		 FROM users WHERE id = $1`,
		id,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt)
	if err == pgx.ErrNoRows {
		return User{}, pgx.ErrNoRows
	}
	return u, err
}
```

- [ ] **Step 5: Run tests to confirm they pass**

```bash
go test ./db/... -v
```

Expected: `PASS` for `TestCreateUser` and `TestGetUserByUsername`.

- [ ] **Step 6: Commit**

```bash
git add server/db/
git commit -m "feat: DB pool + user queries"
```

---

## Task 4: Auth Service (bcrypt + JWT)

**Files:**
- Create: `server/auth/service.go`
- Create: `server/auth/service_test.go`

- [ ] **Step 1: Write failing tests**

Create `server/auth/service_test.go`:

```go
package auth_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/samarth080/peer-chat/auth"
	"github.com/stretchr/testify/require"
)

func TestHashAndCheck(t *testing.T) {
	hash, err := auth.HashPassword("secret123")
	require.NoError(t, err)
	require.NotEmpty(t, hash)

	require.NoError(t, auth.CheckPassword(hash, "secret123"))
	require.Error(t, auth.CheckPassword(hash, "wrongpassword"))
}

func TestGenerateAndParseToken(t *testing.T) {
	id := uuid.New()
	secret := "testsecret"

	token, err := auth.GenerateToken(id, "alice", secret)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	claims, err := auth.ParseToken(token, secret)
	require.NoError(t, err)
	require.Equal(t, id, claims.UserID)
	require.Equal(t, "alice", claims.Username)
}

func TestParseToken_WrongSecret(t *testing.T) {
	id := uuid.New()
	token, _ := auth.GenerateToken(id, "alice", "secret1")

	_, err := auth.ParseToken(token, "secret2")
	require.Error(t, err)
}
```

- [ ] **Step 2: Run test to confirm it fails**

```bash
go test ./auth/... -run TestHashAndCheck -v
```

Expected: compile error — `auth.HashPassword` not defined.

- [ ] **Step 3: Write `server/auth/service.go`**

```go
package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type Claims struct {
	UserID   uuid.UUID `json:"user_id"`
	Username string    `json:"username"`
	jwt.RegisteredClaims
}

func HashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	return string(b), err
}

func CheckPassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

func GenerateToken(userID uuid.UUID, username, secret string) (string, error) {
	claims := Claims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

func ParseToken(tokenStr, secret string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}
```

- [ ] **Step 4: Run tests to confirm they pass**

```bash
go test ./auth/... -v
```

Expected: `PASS` for all 3 tests.

- [ ] **Step 5: Commit**

```bash
git add server/auth/service.go server/auth/service_test.go
git commit -m "feat: auth service — bcrypt + JWT"
```

---

## Task 5: Auth HTTP Handlers

**Files:**
- Create: `server/auth/handler.go`
- Create: `server/auth/handler_test.go`

- [ ] **Step 1: Write failing handler tests**

Create `server/auth/handler_test.go`:

```go
package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/samarth080/peer-chat/auth"
	"github.com/samarth080/peer-chat/db"
	"github.com/stretchr/testify/require"
)

func testHandler(t *testing.T) (*auth.Handler, func()) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = "postgres://chat:chat@localhost:5432/chatdb?sslmode=disable"
	}
	pool, err := db.NewPool(context.Background(), url)
	require.NoError(t, err)
	h := auth.NewHandler(pool, "testsecret")
	return h, func() { pool.Close() }
}

func TestRegister(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, cleanup := testHandler(t)
	defer cleanup()

	r := gin.New()
	r.POST("/auth/register", h.Register)

	body, _ := json.Marshal(map[string]string{"username": "testuser_" + t.Name(), "password": "password123"})
	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotEmpty(t, resp["token"])
	require.NotEmpty(t, resp["user_id"])
}

func TestRegister_DuplicateUsername(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, cleanup := testHandler(t)
	defer cleanup()

	r := gin.New()
	r.POST("/auth/register", h.Register)

	username := "dupuser_" + t.Name()
	body, _ := json.Marshal(map[string]string{"username": username, "password": "pass123"})

	req1 := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	require.Equal(t, http.StatusCreated, w1.Code)

	body2, _ := json.Marshal(map[string]string{"username": username, "password": "pass123"})
	req2 := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	require.Equal(t, http.StatusConflict, w2.Code)
}

func TestLogin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, cleanup := testHandler(t)
	defer cleanup()

	r := gin.New()
	r.POST("/auth/register", h.Register)
	r.POST("/auth/login", h.Login)

	username := "loginuser_" + t.Name()
	regBody, _ := json.Marshal(map[string]string{"username": username, "password": "mypassword"})
	regReq := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(regBody))
	regReq.Header.Set("Content-Type", "application/json")
	httptest.NewRecorder()
	r.ServeHTTP(httptest.NewRecorder(), regReq)

	loginBody, _ := json.Marshal(map[string]string{"username": username, "password": "mypassword"})
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotEmpty(t, resp["token"])
}

func TestLogin_WrongPassword(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, cleanup := testHandler(t)
	defer cleanup()

	r := gin.New()
	r.POST("/auth/register", h.Register)
	r.POST("/auth/login", h.Login)

	username := "badlogin_" + t.Name()
	regBody, _ := json.Marshal(map[string]string{"username": username, "password": "correct"})
	regReq := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(regBody))
	regReq.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(httptest.NewRecorder(), regReq)

	loginBody, _ := json.Marshal(map[string]string{"username": username, "password": "wrong"})
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)
}
```

- [ ] **Step 2: Run to confirm compile failure**

```bash
go test ./auth/... -run TestRegister -v
```

Expected: compile error — `auth.NewHandler`, `auth.Handler` not defined.

- [ ] **Step 3: Write `server/auth/handler.go`**

```go
package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/samarth080/peer-chat/db"
)

type Handler struct {
	pool      *pgxpool.Pool
	jwtSecret string
}

func NewHandler(pool *pgxpool.Pool, jwtSecret string) *Handler {
	return &Handler{pool: pool, jwtSecret: jwtSecret}
}

type registerRequest struct {
	Username string `json:"username" binding:"required,min=2,max=30"`
	Password string `json:"password" binding:"required,min=6"`
}

func (h *Handler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	hash, err := HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}
	user, err := db.CreateUser(c.Request.Context(), h.pool, req.Username, hash)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "username already taken"})
		return
	}
	token, err := GenerateToken(user.ID, user.Username, h.jwtSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"token":    token,
		"user_id":  user.ID,
		"username": user.Username,
	})
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	user, err := db.GetUserByUsername(c.Request.Context(), h.pool, req.Username)
	if err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}
	if err := CheckPassword(user.PasswordHash, req.Password); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	token, err := GenerateToken(user.ID, user.Username, h.jwtSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"token":    token,
		"user_id":  user.ID,
		"username": user.Username,
	})
}
```

- [ ] **Step 4: Run tests to confirm they pass**

```bash
go test ./auth/... -v
```

Expected: `PASS` for all 4 handler tests + 3 service tests.

- [ ] **Step 5: Commit**

```bash
git add server/auth/handler.go server/auth/handler_test.go
git commit -m "feat: auth HTTP handlers — register + login"
```

---

## Task 6: JWT Middleware

**Files:**
- Create: `server/middleware/auth.go`
- Create: `server/middleware/auth_test.go`

- [ ] **Step 1: Write failing middleware test**

Create `server/middleware/auth_test.go`:

```go
package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/samarth080/peer-chat/auth"
	"github.com/samarth080/peer-chat/middleware"
	"github.com/stretchr/testify/require"
)

func setupRouter(secret string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/protected", middleware.JWTAuth(secret), func(c *gin.Context) {
		userID, _ := c.Get("user_id")
		username, _ := c.Get("username")
		c.JSON(http.StatusOK, gin.H{"user_id": userID, "username": username})
	})
	return r
}

func TestJWTAuth_ValidToken(t *testing.T) {
	secret := "testsecret"
	id := uuid.New()
	token, _ := auth.GenerateToken(id, "alice", secret)

	r := setupRouter(secret)
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestJWTAuth_TokenInQueryParam(t *testing.T) {
	secret := "testsecret"
	id := uuid.New()
	token, _ := auth.GenerateToken(id, "alice", secret)

	r := setupRouter(secret)
	req := httptest.NewRequest(http.MethodGet, "/protected?token="+token, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestJWTAuth_MissingToken(t *testing.T) {
	r := setupRouter("secret")
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestJWTAuth_InvalidToken(t *testing.T) {
	r := setupRouter("secret")
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer notavalidtoken")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code)
}
```

- [ ] **Step 2: Run to confirm compile failure**

```bash
go test ./middleware/... -v
```

Expected: compile error — `middleware.JWTAuth` not defined.

- [ ] **Step 3: Write `server/middleware/auth.go`**

```go
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/samarth080/peer-chat/auth"
)

func JWTAuth(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var tokenStr string

		if header := c.GetHeader("Authorization"); header != "" {
			parts := strings.SplitN(header, " ", 2)
			if len(parts) == 2 && parts[0] == "Bearer" {
				tokenStr = parts[1]
			}
		}

		if tokenStr == "" {
			tokenStr = c.Query("token")
		}

		if tokenStr == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
			return
		}

		claims, err := auth.ParseToken(tokenStr, jwtSecret)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Next()
	}
}
```

- [ ] **Step 4: Run tests to confirm they pass**

```bash
go test ./middleware/... -v
```

Expected: `PASS` for all 4 tests.

- [ ] **Step 5: Commit**

```bash
git add server/middleware/
git commit -m "feat: JWT middleware — header + query param support"
```

---

## Task 7: WebSocket Hub + Client

**Files:**
- Create: `server/ws/hub.go`
- Create: `server/ws/hub_test.go`
- Create: `server/ws/client.go`
- Create: `server/ws/handler.go`

- [ ] **Step 1: Write failing hub tests**

Create `server/ws/hub_test.go`:

```go
package ws_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/samarth080/peer-chat/ws"
	"github.com/stretchr/testify/require"
)

func TestHub_RegisterAndSend(t *testing.T) {
	hub := ws.NewHub()

	id := uuid.New()
	client := ws.NewTestClient(id, "alice")
	hub.Register(client)

	msg := []byte(`{"type":"test"}`)
	delivered := hub.Send(id, msg)
	require.True(t, delivered)

	received := <-client.Send
	require.Equal(t, msg, received)
}

func TestHub_SendToOfflineUser(t *testing.T) {
	hub := ws.NewHub()
	id := uuid.New()

	delivered := hub.Send(id, []byte(`{}`))
	require.False(t, delivered)
}

func TestHub_Unregister(t *testing.T) {
	hub := ws.NewHub()
	id := uuid.New()
	client := ws.NewTestClient(id, "alice")
	hub.Register(client)
	hub.Unregister(client)

	delivered := hub.Send(id, []byte(`{}`))
	require.False(t, delivered)
}

func TestHub_IsOnline(t *testing.T) {
	hub := ws.NewHub()
	id := uuid.New()
	require.False(t, hub.IsOnline(id))

	client := ws.NewTestClient(id, "alice")
	hub.Register(client)
	require.True(t, hub.IsOnline(id))

	hub.Unregister(client)
	require.False(t, hub.IsOnline(id))
}
```

- [ ] **Step 2: Run to confirm compile failure**

```bash
go test ./ws/... -v
```

Expected: compile error — `ws.NewHub`, `ws.NewTestClient` not defined.

- [ ] **Step 3: Write `server/ws/hub.go`**

```go
package ws

import (
	"sync"

	"github.com/google/uuid"
)

// Client interface used by Hub — allows test clients without real WebSocket connections
type Client struct {
	UserID   uuid.UUID
	Username string
	send     chan []byte
	// conn is set by real WebSocket connections; nil for test clients
}

// Send channel exposed for testing
func (c *Client) Send() chan []byte { return c.send }

// NewTestClient creates a Client with a buffered channel, for use in tests only
func NewTestClient(userID uuid.UUID, username string) *Client {
	return &Client{
		UserID:   userID,
		Username: username,
		send:     make(chan []byte, 256),
	}
}

type Hub struct {
	mu      sync.RWMutex
	clients map[uuid.UUID]*Client
}

func NewHub() *Hub {
	return &Hub{clients: make(map[uuid.UUID]*Client)}
}

func (h *Hub) Register(c *Client) {
	h.mu.Lock()
	h.clients[c.UserID] = c
	h.mu.Unlock()
}

func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if existing, ok := h.clients[c.UserID]; ok && existing == c {
		delete(h.clients, c.UserID)
		close(c.send)
	}
}

// Send delivers data to a connected client. Returns true if delivered.
func (h *Hub) Send(toID uuid.UUID, data []byte) bool {
	h.mu.RLock()
	client, ok := h.clients[toID]
	h.mu.RUnlock()
	if !ok {
		return false
	}
	select {
	case client.send <- data:
		return true
	default:
		// channel full — drop connection
		h.Unregister(client)
		return false
	}
}

func (h *Hub) IsOnline(userID uuid.UUID) bool {
	h.mu.RLock()
	_, ok := h.clients[userID]
	h.mu.RUnlock()
	return ok
}
```

- [ ] **Step 4: Run hub tests to confirm they pass**

```bash
go test ./ws/... -run TestHub -v
```

Expected: `PASS` for all 4 hub tests.

- [ ] **Step 5: Write `server/ws/client.go`**

```go
package ws

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 4096
)

// InboundEnvelope carries a raw message with its sender's identity
type InboundEnvelope struct {
	SenderID   uuid.UUID
	SenderName string
	Data       []byte
	Ctx        context.Context
}

// Dispatcher is implemented by main.go to route inbound messages
type Dispatcher interface {
	Dispatch(env InboundEnvelope)
}

// newRealClient creates a Client backed by a real WebSocket connection
func newRealClient(userID uuid.UUID, username string, conn *websocket.Conn) *Client {
	return &Client{
		UserID:   userID,
		Username: username,
		send:     make(chan []byte, 256),
	}
}

// readPump reads messages from WebSocket and sends to dispatcher
func (c *Client) readPump(conn *websocket.Conn, hub *Hub, dispatcher Dispatcher) {
	defer func() {
		hub.Unregister(c)
		conn.Close()
	}()

	conn.SetReadLimit(maxMessageSize)
	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("websocket error: %v", err)
			}
			break
		}
		dispatcher.Dispatch(InboundEnvelope{
			SenderID:   c.UserID,
			SenderName: c.Username,
			Data:       data,
			Ctx:        context.Background(),
		})
	}
}

// writePump writes queued messages to WebSocket
func (c *Client) writePump(conn *websocket.Conn) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}

		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
```

- [ ] **Step 6: Write `server/ws/handler.go`**

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
	CheckOrigin: func(r *http.Request) bool {
		return true // tighten in production
	},
}

// ServeWS handles the WebSocket upgrade for GET /ws
// Requires JWTAuth middleware to have set user_id and username in context
func ServeWS(hub *Hub, dispatcher Dispatcher) gin.HandlerFunc {
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

		go client.writePump(conn)
		go client.readPump(conn, hub, dispatcher)
	}
}
```

- [ ] **Step 7: Commit**

```bash
git add server/ws/
git commit -m "feat: WebSocket hub + client (readPump/writePump)"
```

---

## Task 8: Message DB Queries

**Files:**
- Create: `server/db/messages.go`
- Create: `server/db/messages_test.go`

- [ ] **Step 1: Write failing test**

Create `server/db/messages_test.go`:

```go
package db_test

import (
	"context"
	"testing"

	"github.com/samarth080/peer-chat/db"
	"github.com/stretchr/testify/require"
)

func TestFindOrCreateChat(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	userA, _ := db.CreateUser(ctx, pool, "chatA_"+t.Name(), "h")
	userB, _ := db.CreateUser(ctx, pool, "chatB_"+t.Name(), "h")
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM users WHERE id IN ($1,$2)", userA.ID, userB.ID)
	})

	chatID1, err := db.FindOrCreateChat(ctx, pool, userA.ID, userB.ID)
	require.NoError(t, err)
	require.NotEmpty(t, chatID1)

	// calling again returns same chat
	chatID2, err := db.FindOrCreateChat(ctx, pool, userA.ID, userB.ID)
	require.NoError(t, err)
	require.Equal(t, chatID1, chatID2)
}

func TestInsertAndGetMessages(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	userA, _ := db.CreateUser(ctx, pool, "msgA_"+t.Name(), "h")
	userB, _ := db.CreateUser(ctx, pool, "msgB_"+t.Name(), "h")
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM users WHERE id IN ($1,$2)", userA.ID, userB.ID)
	})

	chatID, _ := db.FindOrCreateChat(ctx, pool, userA.ID, userB.ID)

	msg, err := db.InsertMessage(ctx, pool, chatID, userA.ID, "hello world")
	require.NoError(t, err)
	require.Equal(t, "hello world", msg.Body)
	require.Equal(t, "sent", msg.Status)
}
```

- [ ] **Step 2: Run to confirm compile failure**

```bash
go test ./db/... -run TestFindOrCreateChat -v
```

Expected: compile error — `db.FindOrCreateChat`, `db.InsertMessage` not defined.

- [ ] **Step 3: Write `server/db/messages.go`**

```go
package db

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Message struct {
	ID        uuid.UUID
	ChatID    uuid.UUID
	SenderID  uuid.UUID
	Body      string
	Status    string
	CreatedAt time.Time
}

// FindOrCreateChat returns an existing DM chat between two users, or creates one
func FindOrCreateChat(ctx context.Context, pool *pgxpool.Pool, userA, userB uuid.UUID) (uuid.UUID, error) {
	var chatID uuid.UUID
	err := pool.QueryRow(ctx, `
		SELECT cm1.chat_id
		FROM chat_members cm1
		JOIN chat_members cm2 ON cm1.chat_id = cm2.chat_id
		WHERE cm1.user_id = $1 AND cm2.user_id = $2
		LIMIT 1
	`, userA, userB).Scan(&chatID)

	if err == nil {
		return chatID, nil
	}
	if err != pgx.ErrNoRows {
		return uuid.Nil, err
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback(ctx)

	if err := tx.QueryRow(ctx,
		`INSERT INTO chats DEFAULT VALUES RETURNING id`,
	).Scan(&chatID); err != nil {
		return uuid.Nil, err
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO chat_members (chat_id, user_id) VALUES ($1, $2), ($1, $3)`,
		chatID, userA, userB,
	); err != nil {
		return uuid.Nil, err
	}

	return chatID, tx.Commit(ctx)
}

func InsertMessage(ctx context.Context, pool *pgxpool.Pool, chatID, senderID uuid.UUID, body string) (Message, error) {
	var m Message
	err := pool.QueryRow(ctx, `
		INSERT INTO messages (chat_id, sender_id, body)
		VALUES ($1, $2, $3)
		RETURNING id, chat_id, sender_id, body, status, created_at
	`, chatID, senderID, body).Scan(
		&m.ID, &m.ChatID, &m.SenderID, &m.Body, &m.Status, &m.CreatedAt,
	)
	return m, err
}

func GetUndeliveredMessages(ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID) ([]Message, error) {
	rows, err := pool.Query(ctx, `
		SELECT m.id, m.chat_id, m.sender_id, m.body, m.status, m.created_at
		FROM messages m
		JOIN chat_members cm ON cm.chat_id = m.chat_id
		WHERE cm.user_id = $1 AND m.sender_id != $1 AND m.status = 'sent'
		ORDER BY m.created_at ASC
		LIMIT 100
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.ChatID, &m.SenderID, &m.Body, &m.Status, &m.CreatedAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

func UpdateMessageStatus(ctx context.Context, pool *pgxpool.Pool, messageID uuid.UUID, status string) error {
	_, err := pool.Exec(ctx, `UPDATE messages SET status = $1 WHERE id = $2`, status, messageID)
	return err
}

func GetChatHistory(ctx context.Context, pool *pgxpool.Pool, chatID uuid.UUID, limit, offset int) ([]Message, error) {
	rows, err := pool.Query(ctx, `
		SELECT id, chat_id, sender_id, body, status, created_at
		FROM messages
		WHERE chat_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, chatID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.ChatID, &m.SenderID, &m.Body, &m.Status, &m.CreatedAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}
```

- [ ] **Step 4: Run tests to confirm they pass**

```bash
go test ./db/... -v
```

Expected: `PASS` for all db tests.

- [ ] **Step 5: Commit**

```bash
git add server/db/messages.go server/db/messages_test.go
git commit -m "feat: message DB queries — insert, history, undelivered"
```

---

## Task 9: DM Message Handler

**Files:**
- Create: `server/chat/dm.go`
- Create: `server/chat/dm_test.go`

- [ ] **Step 1: Write failing test**

Create `server/chat/dm_test.go`:

```go
package chat_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/samarth080/peer-chat/chat"
	"github.com/samarth080/peer-chat/db"
	"github.com/samarth080/peer-chat/ws"
	"github.com/stretchr/testify/require"
	"os"
)

func testPool(t *testing.T) *db.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = "postgres://chat:chat@localhost:5432/chatdb?sslmode=disable"
	}
	pool, err := db.NewPool(context.Background(), url)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })
	return pool
}

func TestHandleDM_DeliverToOnlineReceiver(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	sender, _ := db.CreateUser(ctx, pool, "dm_sender_"+t.Name(), "h")
	receiver, _ := db.CreateUser(ctx, pool, "dm_recv_"+t.Name(), "h")
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM users WHERE id IN ($1,$2)", sender.ID, receiver.ID)
	})

	hub := ws.NewHub()
	recvClient := ws.NewTestClient(receiver.ID, receiver.Username)
	senderClient := ws.NewTestClient(sender.ID, sender.Username)
	hub.Register(recvClient)
	hub.Register(senderClient)

	raw, _ := json.Marshal(map[string]string{
		"type": "message",
		"to":   receiver.ID.String(),
		"body": "hello there",
		"id":   uuid.New().String(),
	})

	err := chat.HandleDM(ctx, pool, hub, sender.ID, sender.Username, raw)
	require.NoError(t, err)

	// receiver gets the message
	msg := <-recvClient.Send()
	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(msg, &out))
	require.Equal(t, "message", out["type"])
	require.Equal(t, "hello there", out["body"])

	// sender gets ack
	ack := <-senderClient.Send()
	var ackOut map[string]interface{}
	require.NoError(t, json.Unmarshal(ack, &ackOut))
	require.Equal(t, "sent", ackOut["type"])
}
```

- [ ] **Step 2: Run to confirm compile failure**

```bash
go test ./chat/... -run TestHandleDM -v
```

Expected: compile error — `chat.HandleDM` not defined.

- [ ] **Step 3: Write `server/chat/dm.go`**

```go
package chat

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/samarth080/peer-chat/db"
	"github.com/samarth080/peer-chat/ws"
)

type dmInbound struct {
	Type string `json:"type"`
	To   string `json:"to"`
	Body string `json:"body"`
	ID   string `json:"id"` // client-generated correlation ID
}

// HandleDM processes a direct message. Inserts to DB, fans out to receiver, acks sender.
func HandleDM(ctx context.Context, pool *pgxpool.Pool, hub *ws.Hub, senderID uuid.UUID, senderName string, raw []byte) error {
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
	hub.Send(receiverID, outbound)

	ack, _ := json.Marshal(map[string]interface{}{
		"type":       "sent",
		"message_id": stored.ID,
		"id":         msg.ID,
		"status":     "sent",
	})
	hub.Send(senderID, ack)

	return nil
}

// HandleAck processes a delivery acknowledgement from receiver
func HandleAck(ctx context.Context, pool *pgxpool.Pool, hub *ws.Hub, senderID uuid.UUID, raw []byte) error {
	var payload struct {
		MessageID string `json:"message_id"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return err
	}
	msgID, err := uuid.Parse(payload.MessageID)
	if err != nil {
		return err
	}
	return db.UpdateMessageStatus(ctx, pool, msgID, "delivered")
}
```

- [ ] **Step 4: Run tests to confirm they pass**

```bash
go test ./chat/... -run TestHandleDM -v
```

Expected: `PASS`.

- [ ] **Step 5: Commit**

```bash
git add server/chat/dm.go server/chat/dm_test.go
git commit -m "feat: DM handler — DB insert, fan-out, ack"
```

---

## Task 10: Group DB Queries + Group Message Handler

**Files:**
- Create: `server/db/groups.go`
- Create: `server/chat/group.go`
- Create: `server/chat/group_test.go`

- [ ] **Step 1: Write `server/db/groups.go`**

```go
package db

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Group struct {
	ID        uuid.UUID
	Name      string
	CreatedBy uuid.UUID
	CreatedAt time.Time
}

type GroupMessage struct {
	ID        uuid.UUID
	GroupID   uuid.UUID
	SenderID  uuid.UUID
	Body      string
	CreatedAt time.Time
}

type GroupMemberInfo struct {
	UserID   uuid.UUID
	Username string
	Role     string
	JoinedAt time.Time
}

func CreateGroup(ctx context.Context, pool *pgxpool.Pool, name string, creatorID uuid.UUID) (Group, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return Group{}, err
	}
	defer tx.Rollback(ctx)

	var g Group
	if err := tx.QueryRow(ctx,
		`INSERT INTO groups (name, created_by) VALUES ($1, $2)
		 RETURNING id, name, created_by, created_at`,
		name, creatorID,
	).Scan(&g.ID, &g.Name, &g.CreatedBy, &g.CreatedAt); err != nil {
		return Group{}, err
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO group_members (group_id, user_id, role) VALUES ($1, $2, 'admin')`,
		g.ID, creatorID,
	); err != nil {
		return Group{}, err
	}

	return g, tx.Commit(ctx)
}

func GetGroupByName(ctx context.Context, pool *pgxpool.Pool, name string) (Group, error) {
	var g Group
	err := pool.QueryRow(ctx,
		`SELECT id, name, created_by, created_at FROM groups WHERE name = $1`, name,
	).Scan(&g.ID, &g.Name, &g.CreatedBy, &g.CreatedAt)
	if err == pgx.ErrNoRows {
		return Group{}, pgx.ErrNoRows
	}
	return g, err
}

func IsGroupMember(ctx context.Context, pool *pgxpool.Pool, groupID, userID uuid.UUID) (bool, error) {
	var exists bool
	err := pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM group_members WHERE group_id = $1 AND user_id = $2)`,
		groupID, userID,
	).Scan(&exists)
	return exists, err
}

func AddGroupMember(ctx context.Context, pool *pgxpool.Pool, groupID, userID uuid.UUID, role string) error {
	_, err := pool.Exec(ctx,
		`INSERT INTO group_members (group_id, user_id, role) VALUES ($1, $2, $3)
		 ON CONFLICT (group_id, user_id) DO NOTHING`,
		groupID, userID, role,
	)
	return err
}

func RemoveGroupMember(ctx context.Context, pool *pgxpool.Pool, groupID, userID uuid.UUID) error {
	_, err := pool.Exec(ctx,
		`DELETE FROM group_members WHERE group_id = $1 AND user_id = $2`,
		groupID, userID,
	)
	return err
}

func GetGroupMembers(ctx context.Context, pool *pgxpool.Pool, groupID uuid.UUID) ([]GroupMemberInfo, error) {
	rows, err := pool.Query(ctx, `
		SELECT gm.user_id, u.username, gm.role, gm.joined_at
		FROM group_members gm
		JOIN users u ON gm.user_id = u.id
		WHERE gm.group_id = $1
		ORDER BY gm.joined_at ASC
	`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []GroupMemberInfo
	for rows.Next() {
		var m GroupMemberInfo
		if err := rows.Scan(&m.UserID, &m.Username, &m.Role, &m.JoinedAt); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

func InsertGroupMessage(ctx context.Context, pool *pgxpool.Pool, groupID, senderID uuid.UUID, body string) (GroupMessage, error) {
	var m GroupMessage
	err := pool.QueryRow(ctx, `
		INSERT INTO group_messages (group_id, sender_id, body)
		VALUES ($1, $2, $3)
		RETURNING id, group_id, sender_id, body, created_at
	`, groupID, senderID, body).Scan(
		&m.ID, &m.GroupID, &m.SenderID, &m.Body, &m.CreatedAt,
	)
	return m, err
}

func GetGroupMessages(ctx context.Context, pool *pgxpool.Pool, groupID uuid.UUID, limit, offset int) ([]GroupMessage, error) {
	rows, err := pool.Query(ctx, `
		SELECT id, group_id, sender_id, body, created_at
		FROM group_messages
		WHERE group_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, groupID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []GroupMessage
	for rows.Next() {
		var m GroupMessage
		if err := rows.Scan(&m.ID, &m.GroupID, &m.SenderID, &m.Body, &m.CreatedAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

func ListUserGroups(ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID) ([]Group, error) {
	rows, err := pool.Query(ctx, `
		SELECT g.id, g.name, g.created_by, g.created_at
		FROM groups g
		JOIN group_members gm ON g.id = gm.group_id
		WHERE gm.user_id = $1
		ORDER BY g.created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []Group
	for rows.Next() {
		var g Group
		if err := rows.Scan(&g.ID, &g.Name, &g.CreatedBy, &g.CreatedAt); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}
```

- [ ] **Step 2: Write failing group message handler test**

Create `server/chat/group_test.go`:

```go
package chat_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/samarth080/peer-chat/chat"
	"github.com/samarth080/peer-chat/db"
	"github.com/samarth080/peer-chat/ws"
	"github.com/stretchr/testify/require"
)

func TestHandleGroupMessage_FanOut(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	creator, _ := db.CreateUser(ctx, pool, "gcreator_"+t.Name(), "h")
	member, _ := db.CreateUser(ctx, pool, "gmember_"+t.Name(), "h")
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM users WHERE id IN ($1,$2)", creator.ID, member.ID)
	})

	group, err := db.CreateGroup(ctx, pool, "testgroup_"+t.Name(), creator.ID)
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM groups WHERE id = $1", group.ID)
	})

	require.NoError(t, db.AddGroupMember(ctx, pool, group.ID, member.ID, "member"))

	hub := ws.NewHub()
	creatorClient := ws.NewTestClient(creator.ID, creator.Username)
	memberClient := ws.NewTestClient(member.ID, member.Username)
	hub.Register(creatorClient)
	hub.Register(memberClient)

	raw, _ := json.Marshal(map[string]string{
		"type":     "group_message",
		"group_id": group.ID.String(),
		"body":     "hello group",
	})

	err = chat.HandleGroupMessage(ctx, pool, hub, creator.ID, creator.Username, raw)
	require.NoError(t, err)

	// member receives the fan-out
	msg := <-memberClient.Send()
	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(msg, &out))
	require.Equal(t, "group_message", out["type"])
	require.Equal(t, "hello group", out["body"])
}
```

- [ ] **Step 3: Run to confirm compile failure**

```bash
go test ./chat/... -run TestHandleGroupMessage -v
```

Expected: compile error — `chat.HandleGroupMessage` not defined.

- [ ] **Step 4: Write `server/chat/group.go`**

```go
package chat

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/samarth080/peer-chat/db"
	"github.com/samarth080/peer-chat/ws"
)

type groupMsgInbound struct {
	Type    string `json:"type"`
	GroupID string `json:"group_id"`
	Body    string `json:"body"`
}

// HandleGroupMessage stores a group message and fans out to all online members
func HandleGroupMessage(ctx context.Context, pool *pgxpool.Pool, hub *ws.Hub, senderID uuid.UUID, senderName string, raw []byte) error {
	var msg groupMsgInbound
	if err := json.Unmarshal(raw, &msg); err != nil {
		return fmt.Errorf("invalid group_message payload: %w", err)
	}

	groupID, err := uuid.Parse(msg.GroupID)
	if err != nil {
		return fmt.Errorf("invalid group_id: %w", err)
	}

	isMember, err := db.IsGroupMember(ctx, pool, groupID, senderID)
	if err != nil {
		return err
	}
	if !isMember {
		sendError(hub, senderID, "not a member of this group")
		return nil
	}

	stored, err := db.InsertGroupMessage(ctx, pool, groupID, senderID, msg.Body)
	if err != nil {
		return fmt.Errorf("insert group message: %w", err)
	}

	outbound, _ := json.Marshal(map[string]interface{}{
		"type":       "group_message",
		"group_id":   groupID,
		"from":       senderName,
		"from_id":    senderID,
		"body":       stored.Body,
		"message_id": stored.ID,
		"timestamp":  stored.CreatedAt,
	})

	members, err := db.GetGroupMembers(ctx, pool, groupID)
	if err != nil {
		return err
	}
	for _, m := range members {
		if m.UserID != senderID {
			hub.Send(m.UserID, outbound)
		}
	}

	// ack to sender
	ack, _ := json.Marshal(map[string]interface{}{
		"type":       "group_sent",
		"message_id": stored.ID,
		"status":     "sent",
	})
	hub.Send(senderID, ack)

	return nil
}

// HandleCreateGroup creates a new group and acks the creator
func HandleCreateGroup(ctx context.Context, pool *pgxpool.Pool, hub *ws.Hub, creatorID uuid.UUID, raw []byte) error {
	var payload struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return err
	}

	group, err := db.CreateGroup(ctx, pool, payload.Name, creatorID)
	if err != nil {
		sendError(hub, creatorID, "group name already taken")
		return nil
	}

	resp, _ := json.Marshal(map[string]interface{}{
		"type":     "group_created",
		"group_id": group.ID,
		"name":     group.Name,
	})
	hub.Send(creatorID, resp)
	return nil
}

// HandleAddMember adds a user to a group (any member can add)
func HandleAddMember(ctx context.Context, pool *pgxpool.Pool, hub *ws.Hub, adderID uuid.UUID, raw []byte) error {
	var payload struct {
		GroupID string `json:"group_id"`
		UserID  string `json:"user_id"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return err
	}

	groupID, err := uuid.Parse(payload.GroupID)
	if err != nil {
		return err
	}
	userID, err := uuid.Parse(payload.UserID)
	if err != nil {
		return err
	}

	isMember, err := db.IsGroupMember(ctx, pool, groupID, adderID)
	if err != nil {
		return err
	}
	if !isMember {
		sendError(hub, adderID, "you are not a member of this group")
		return nil
	}

	if err := db.AddGroupMember(ctx, pool, groupID, userID, "member"); err != nil {
		sendError(hub, adderID, "failed to add member")
		return nil
	}

	resp, _ := json.Marshal(map[string]interface{}{
		"type":     "member_added",
		"group_id": groupID,
		"user_id":  userID,
	})
	hub.Send(adderID, resp)
	return nil
}

// HandleLeaveGroup removes the requesting user from a group
func HandleLeaveGroup(ctx context.Context, pool *pgxpool.Pool, hub *ws.Hub, userID uuid.UUID, raw []byte) error {
	var payload struct {
		GroupID string `json:"group_id"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return err
	}

	groupID, err := uuid.Parse(payload.GroupID)
	if err != nil {
		return err
	}

	if err := db.RemoveGroupMember(ctx, pool, groupID, userID); err != nil {
		return err
	}

	resp, _ := json.Marshal(map[string]interface{}{
		"type":     "left_group",
		"group_id": groupID,
	})
	hub.Send(userID, resp)
	return nil
}

func sendError(hub *ws.Hub, userID uuid.UUID, msg string) {
	data, _ := json.Marshal(map[string]string{"type": "error", "message": msg})
	hub.Send(userID, data)
}
```

- [ ] **Step 5: Run all tests**

```bash
go test ./... -v
```

Expected: `PASS` for all tests across all packages.

- [ ] **Step 6: Commit**

```bash
git add server/db/groups.go server/chat/group.go server/chat/group_test.go
git commit -m "feat: group DB queries + group message handler"
```

---

## Task 11: main.go — Wire Everything Together

**Files:**
- Create: `server/main.go`

- [ ] **Step 1: Write `server/main.go`**

```go
package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
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
		if err := chat.HandleDM(env.Ctx, d.pool, d.hub, env.SenderID, env.SenderName, env.Data); err != nil {
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

	r.GET("/ws", middleware.JWTAuth(cfg.JWTSecret), ws.ServeWS(hub, dispatcher))

	addr := ":" + cfg.Port
	log.Printf("server starting on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
```

- [ ] **Step 2: Build the binary to confirm it compiles**

```bash
cd server
go build ./...
```

Expected: no output (clean build). Binary not needed yet.

- [ ] **Step 3: Set up local env and run**

```bash
cp .env.example .env
# Edit .env — set JWT_SECRET to any 32+ char string
# DATABASE_URL should already be: postgres://chat:chat@localhost:5432/chatdb?sslmode=disable

# Ensure docker-compose is up
docker-compose up -d postgres

# Source env and run
set -a && source .env && set +a
go run ./server/main.go
```

Expected output:
```
[GIN-debug] POST   /auth/register
[GIN-debug] POST   /auth/login
[GIN-debug] GET    /ws
[GIN-debug] GET    /health
server starting on :8080
```

- [ ] **Step 4: Smoke test with curl**

```bash
# Register
curl -s -X POST localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"alice","password":"password123"}' | jq .

# Expected:
# { "token": "eyJ...", "user_id": "...", "username": "alice" }

# Health check
curl -s localhost:8080/health | jq .
# Expected: { "status": "ok" }
```

- [ ] **Step 5: Write Dockerfile**

Create `Dockerfile` in repo root:

```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY server/ .
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o chat-server .

FROM gcr.io/distroless/static-debian12
COPY --from=builder /app/chat-server /chat-server
EXPOSE 8080
CMD ["/chat-server"]
```

- [ ] **Step 6: Test Docker build**

```bash
docker build -t peer-chat-server .
```

Expected: image builds successfully.

- [ ] **Step 7: Commit**

```bash
git add server/main.go Dockerfile
git commit -m "feat: main.go — wire all handlers, dispatch goroutine, health check"
```

---

## Task 12: Run Full Test Suite + Verify

- [ ] **Step 1: Run all tests**

```bash
cd server
go test ./... -v -count=1
```

Expected: all tests `PASS`. Note the `-count=1` flag disables test caching so tests actually run.

- [ ] **Step 2: Run tests with race detector**

```bash
go test ./... -race -count=1
```

Expected: no race conditions reported.

- [ ] **Step 3: Verify WebSocket with wscat**

```bash
npm install -g wscat  # one-time

# Get a token first
TOKEN=$(curl -s -X POST localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"wstest","password":"pass123"}' | jq -r .token)

wscat -c "ws://localhost:8080/ws?token=$TOKEN"
# Type: {"type":"ping"}
# Expected response: {"type":"pong"}
```

- [ ] **Step 4: Final commit**

```bash
git add .
git commit -m "chore: Plan 1 complete — Go server foundation"
```

---

## What Plan 1 Delivers

After this plan, you have:
- `POST /auth/register` and `POST /auth/login` with bcrypt + JWT
- `GET /ws?token=<jwt>` — authenticated WebSocket connections
- Real-time DM delivery to online users, DB persistence for offline
- Group chat with member management + fan-out
- Message delivery status (`sent → delivered`)
- `GET /health` endpoint
- Full test suite (unit + integration)
- Docker image that builds

**Next:** Plan 2 adds Redis for presence, typing indicators, cross-instance pub/sub, and rate limiting.
