# realtime-chat-go

A production-grade real-time chat backend built with **Go**, **PostgreSQL**, and **Redis**. Features WebSocket-based messaging, JWT authentication, Redis pub/sub for horizontal scaling, WebRTC signaling, presence tracking, typing indicators, and rate limiting.

---

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Server | Go 1.22, Gin, Gorilla WebSocket |
| Auth | JWT (HS256), bcrypt |
| Database | PostgreSQL 16, pgx/v5 |
| Cache / Pub-Sub | Redis 7, go-redis/v9 |
| Frontend | React 18, Vite, Tailwind CSS *(in progress)* |
| CLI Client | Go *(in progress)* |
| Deploy | Fly.io *(in progress)* |

---

## Features

### Authentication
- `POST /auth/register` — bcrypt password hashing (cost 12), returns JWT
- `POST /auth/login` — verifies hash, returns JWT
- JWT validated on every WebSocket upgrade — sender identity never trusted from client payload

### WebSocket Messaging
- Goroutine-per-connection model with a mutex-protected hub
- Direct messages with find-or-create chat, DB persistence, delivery ack
- Group messaging with membership checks and fan-out (skipping sender)
- Group management: create, add member, leave

### Redis Layer
- **Presence** — `SET presence:{user_id} online EX 30`, refreshed by heartbeat ping every 20s
- **Typing indicators** — ephemeral Redis key with 5s TTL, fans out to chat members
- **Rate limiting** — atomic INCR + ExpireNX (fixed window, 30 msg/60s per user)
- **Cross-instance pub/sub** — messages routed via `chat:user:{id}` channels when sender and receiver are on different server instances

### Delivery Pipeline
1. Client sends `{"type":"message","to":"<user_id>","body":"...","id":"<client-uuid>"}`
2. JWT middleware extracts sender identity
3. Rate limit checked via Redis
4. Message inserted into PostgreSQL
5. Local delivery attempted via hub; falls back to Redis pub/sub if receiver is on another instance
6. Sender receives `{"type":"sent","message_id":"..."}` ack

### Database Schema
- UUID primary keys throughout (no enumeration attacks)
- `messages.status` — `sent | delivered | read` with CHECK constraint
- Composite index on `(chat_id, created_at DESC)` for efficient pagination
- `ON DELETE CASCADE` throughout — no orphan rows

---

## Project Structure

```
realtime-chat-go/
├── server/
│   ├── main.go              # Entry point, router, dispatcher
│   ├── auth/                # JWT + bcrypt, register/login handlers
│   ├── ws/                  # WebSocket hub, client, handler
│   ├── chat/                # DM + group message handlers
│   ├── presence/            # Online/offline + typing indicators
│   ├── ratelimit/           # Redis sliding window rate limiter
│   ├── pubsub/              # Cross-instance pub/sub router
│   ├── db/                  # PostgreSQL queries (pgx)
│   ├── middleware/          # JWT validation middleware
│   └── config/              # Environment config
├── migrations/              # PostgreSQL schema (golang-migrate)
├── docker-compose.yml       # Local dev: server + postgres + redis
├── Dockerfile               # Multi-stage Go build
└── .env.example
```

---

## Local Development

**Prerequisites:** Docker, Go 1.22+

```bash
# Start PostgreSQL + Redis
docker-compose up -d

# Copy env and fill in values
cp .env.example .env

# Run migrations
migrate -path migrations -database "$DATABASE_URL" up

# Start server
cd server && go run .
```

**Environment variables:**
```
PORT=8080
DATABASE_URL=postgres://chat:chat@localhost:5432/chatdb?sslmode=disable
REDIS_URL=redis://localhost:6379
JWT_SECRET=<random 32-byte hex>
ENV=development
```

---

## Deploy to Fly.io

**Prerequisites:** [flyctl](https://fly.io/docs/hands-on/install-flyctl/) installed, logged in (`flyctl auth login`)

```bash
# Create app (name must be globally unique)
flyctl apps create realtime-chat-go

# Provision managed Postgres
flyctl postgres create --name realtime-chat-db --region iad --initial-cluster-size 1 --vm-size shared-cpu-1x --volume-size 1
flyctl postgres attach realtime-chat-db --app realtime-chat-go

# Create Upstash Redis (free tier)
flyctl redis create --name realtime-chat-redis --region iad --plan free
# Copy the redis URL from output, then:
flyctl secrets set REDIS_URL=<redis-url> --app realtime-chat-go
flyctl secrets set JWT_SECRET=$(openssl rand -hex 32) --app realtime-chat-go
flyctl secrets set ENV=production --app realtime-chat-go

# Run migrations
migrate -path migrations -database "$DATABASE_URL" up

# Deploy
flyctl deploy
```

**Frontend (Netlify):**
```bash
cd web
npm run build
netlify deploy --prod --dir dist
```
Set `VITE_WS_URL=wss://realtime-chat-go.fly.dev` in Netlify environment variables.

---

## WebSocket Protocol

Connect: `GET /ws?token=<jwt>`

**Client → Server:**
```json
{"type": "message",       "to": "<user_id>",  "body": "hello", "id": "<client-uuid>"}
{"type": "group_message", "to": "<group_id>", "body": "hello", "id": "<client-uuid>"}
{"type": "typing",        "chat_id": "<id>",  "members": ["<id1>", "<id2>"]}
{"type": "ack",           "message_id": "<uuid>"}
{"type": "ping"}
```

**Server → Client:**
```json
{"type": "message",   "from": "<user_id>", "body": "...", "message_id": "<uuid>", "timestamp": "..."}
{"type": "delivered", "message_id": "<uuid>"}
{"type": "typing",    "from": "<username>", "chat_id": "<uuid>"}
{"type": "presence",  "user_id": "<uuid>", "status": "online|offline"}
{"type": "pong"}
```

---

## What This Demonstrates

- **Go concurrency** — goroutine-per-connection, channel-based hub, no mutex on the hot read path
- **Distributed systems** — Redis pub/sub for cross-instance routing; adding server instances scales horizontally
- **Real-time architecture** — fan-out delivery, offline queue, delivery status tracking
- **WebRTC** — signaling server for SDP/ICE relay, P2P DataChannel *(in progress)*
- **Production readiness** — JWT auth, bcrypt, rate limiting, Docker, health checks
- **Database design** — UUID PKs, composite indexes, cascading deletes, type-safe pgx queries
