# realtime-chat-go

A production-grade real-time chat application built with **Go**, **PostgreSQL**, and **Redis**. Features WebSocket-based messaging, JWT authentication, Redis pub/sub for horizontal scaling, WebRTC signaling, presence tracking, typing indicators, and rate limiting.

**Live demo:** [https://p2p-chat-app.netlify.app](https://p2p-chat-app.netlify.app)

---

## Author

**Samarth Chatli**
GitHub: [@samarth080](https://github.com/samarth080)

---

## License

Copyright (c) 2026 Samarth Chatli. All rights reserved.

This project and its source code are proprietary. No part of this software may be reproduced, distributed, modified, sublicensed, sold, or used in any form without the prior written permission of the author. See [LICENSE](./LICENSE) for full terms.

---

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Server | Go 1.23, Gin, Gorilla WebSocket |
| Auth | JWT (HS256), bcrypt |
| Database | PostgreSQL 16, pgx/v5 |
| Cache / Pub-Sub | Redis 7, go-redis/v9 |
| Frontend | React 18, TypeScript, Vite, Tailwind CSS, Zustand |
| Deploy | Render (server) · Supabase (DB) · Upstash (Redis) · Netlify (frontend) |

---

## Features

### Authentication
- `POST /auth/register` — bcrypt password hashing (cost 12), returns JWT
- `POST /auth/login` — verifies hash, returns JWT
- JWT validated on every WebSocket upgrade — sender identity never trusted from client payload

### Direct Messaging
- Find-or-create DM chat between two users
- Messages persisted in PostgreSQL with `sent | delivered | read` status
- Delivery ack sent back to sender on success
- Offline queue: pending messages delivered on reconnect
- Duplicate prevention: status updated to `delivered` immediately on successful local hub delivery

### Group Messaging
- Create groups, add members, leave group
- Fan-out delivery to all online members (skipping sender)
- Membership enforced server-side

### Redis Layer
- **Presence** — `SET presence:{user_id} online EX 30`, refreshed by heartbeat ping every 20s
- **Typing indicators** — ephemeral Redis key with 5s TTL, fans out to chat members
- **Rate limiting** — atomic INCR + ExpireNX (fixed window, 30 msg/60s per user)
- **Cross-instance pub/sub** — messages routed via `chat:user:{id}` channels when sender and receiver are on different server instances

### WebRTC Signaling
- SDP offer/answer and ICE candidate relay between peers
- Rewritten `from` field so receiver knows the caller identity
- Falls back to Redis pub/sub for cross-instance relay

### Frontend
- React + TypeScript SPA with Zustand for global state
- Persistent state via `zustand/middleware` — contacts and messages survive page refresh
- Auto-reconnecting WebSocket with exponential-style backoff
- Copy-your-ID button for sharing with contacts
- Typing indicators and presence badges

### Delivery Pipeline
1. Client sends `{"type":"message","to":"<user_id>","body":"...","id":"<client-uuid>"}`
2. JWT middleware extracts sender identity
3. Rate limit checked via Redis
4. Message inserted into PostgreSQL (`status = sent`)
5. Local delivery attempted via hub; if successful → status updated to `delivered`
6. If receiver not on this instance → published to Redis pub/sub
7. On receiver reconnect → pending `sent` messages re-delivered
8. Sender receives `{"type":"sent","message_id":"..."}` ack

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
│   ├── ratelimit/           # Redis fixed-window rate limiter
│   ├── pubsub/              # Cross-instance pub/sub router
│   ├── db/                  # PostgreSQL queries (pgx)
│   ├── middleware/          # JWT validation middleware
│   └── config/              # Environment config
├── web/                     # React + TypeScript frontend
│   ├── src/
│   │   ├── pages/           # ChatPage, LoginPage, RegisterPage
│   │   ├── components/      # ChatWindow, ContactList, GroupPanel
│   │   ├── hooks/           # useWebSocket (auto-reconnect, message dispatch)
│   │   ├── store.ts         # Zustand store with persist middleware
│   │   └── api.ts           # REST helpers (register, login)
│   └── public/_redirects    # Netlify SPA routing
├── migrations/              # PostgreSQL schema (golang-migrate)
├── docker-compose.yml       # Local dev: server + postgres + redis
├── Dockerfile               # Multi-stage Go build (distroless runtime)
├── render.yaml              # Render deployment config
└── .env.example
```

---

## Local Development

**Prerequisites:** Docker, Go 1.23+, Node 18+

```bash
# Start PostgreSQL + Redis
docker-compose up -d

# Copy env and fill in values
cp .env.example .env

# Run migrations
migrate -path migrations -database "$DATABASE_URL" up

# Start server
cd server && go run .

# Start frontend (separate terminal)
cd web && npm install && npm run dev
```

**Server environment variables:**
```
PORT=8080
DATABASE_URL=postgres://chat:chat@localhost:5432/chatdb?sslmode=disable
REDIS_URL=redis://localhost:6379
JWT_SECRET=<random 32-byte hex>
ENV=development
```

**Frontend environment variables (`web/.env.local`):**
```
VITE_WS_URL=ws://localhost:8080
VITE_API_URL=http://localhost:8080
```

---

## Deploy (Free Tier — No Credit Card Required)

This project is deployed using four free-tier services:

| Service | Purpose | URL |
|---------|---------|-----|
| [Render](https://render.com) | Go server hosting | Free web service |
| [Supabase](https://supabase.com) | PostgreSQL database | Free tier |
| [Upstash](https://upstash.com) | Redis | Free tier |
| [Netlify](https://netlify.com) | React frontend | Free tier |

### 1. Database — Supabase

1. Create a project at [supabase.com](https://supabase.com)
2. Go to **Project Settings → Database → Connect → Session pooler** (use the IPv4-compatible URL)
3. Run migrations against the Supabase URL:
   ```bash
   migrate -path migrations -database "postgres://postgres.xxxxx:password@aws-x-region.pooler.supabase.com:5432/postgres" up
   ```

### 2. Redis — Upstash

1. Create a Redis database at [upstash.com](https://upstash.com)
2. Copy the `rediss://` TLS URL (not the redis-cli command)

### 3. Server — Render

1. Connect your GitHub repo at [render.com](https://render.com)
2. Create a new **Web Service** with Docker runtime (or use `render.yaml`)
3. Set environment variables:
   ```
   DATABASE_URL=<supabase session pooler url>
   REDIS_URL=<upstash rediss:// url>
   JWT_SECRET=<random 32-byte hex>
   ENV=production
   PORT=10000
   ```

### 4. Frontend — Netlify

1. Connect your GitHub repo at [netlify.com](https://netlify.com)
2. **Base directory:** `web`
3. **Build command:** `npm run build`
4. **Publish directory:** `web/dist`
5. Set environment variables:
   ```
   VITE_WS_URL=wss://<your-render-app>.onrender.com
   VITE_API_URL=https://<your-render-app>.onrender.com
   ```

> **Note:** Render free tier spins down after 15 minutes of inactivity. The first request after spin-down takes ~30–50 seconds to respond while the instance cold-starts.

---

## WebSocket Protocol

Connect: `GET /ws?token=<jwt>`

**Client → Server:**
```json
{"type": "message",       "to": "<user_id>",  "body": "hello", "id": "<client-uuid>"}
{"type": "group_message", "to": "<group_id>", "body": "hello", "id": "<client-uuid>"}
{"type": "create_group",  "name": "<group_name>"}
{"type": "add_member",    "group_id": "<id>", "user_id": "<id>"}
{"type": "leave_group",   "group_id": "<id>"}
{"type": "typing",        "chat_id": "<id>",  "members": ["<id1>", "<id2>"]}
{"type": "ack",           "message_id": "<uuid>"}
{"type": "ping"}
{"type": "webrtc_offer",  "to": "<user_id>",  "sdp": "..."}
{"type": "webrtc_answer", "to": "<user_id>",  "sdp": "..."}
{"type": "ice_candidate", "to": "<user_id>",  "candidate": "..."}
```

**Server → Client:**
```json
{"type": "message",   "from": "<username>", "from_id": "<uuid>", "body": "...", "message_id": "<uuid>", "timestamp": "..."}
{"type": "sent",      "message_id": "<uuid>", "id": "<client-uuid>", "status": "sent"}
{"type": "typing",    "from": "<username>", "chat_id": "<uuid>"}
{"type": "presence",  "user_id": "<uuid>", "status": "online|offline"}
{"type": "pong"}
{"type": "error",     "code": "rate_limited"}
```

---

## What This Demonstrates

- **Go concurrency** — goroutine-per-connection, channel-based hub, no mutex on the hot read path
- **Distributed systems** — Redis pub/sub for cross-instance routing; adding server instances scales horizontally
- **Real-time architecture** — fan-out delivery, offline queue, delivery status tracking, duplicate prevention
- **WebRTC signaling** — SDP/ICE relay for P2P audio/video setup
- **Production readiness** — JWT auth, bcrypt, rate limiting, Docker (distroless), health checks, CORS
- **Database design** — UUID PKs, composite indexes, cascading deletes, type-safe pgx queries
- **Frontend state** — Zustand with persist middleware, anti-pattern-safe selectors, auto-reconnecting WebSocket
