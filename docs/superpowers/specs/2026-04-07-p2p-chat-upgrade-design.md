# P2P Chat System — Full Upgrade Design Spec
**Date:** 2026-04-07  
**Status:** Implemented  
**Scope:** Full rewrite from Python/MySQL proof-of-concept to production-grade Go backend + React frontend + Go CLI

---

## 1. Goals

Transform the existing Python WebSocket CRUD app into a credible L4-level portfolio project demonstrating:
- Go concurrency (goroutines, channels)
- Redis pub/sub for cross-instance message routing (horizontal scaling)
- PostgreSQL with async queries and proper indexing
- Real-time WebSocket architecture with JWT auth
- WebRTC P2P signaling (server as relay only)
- Presence, typing indicators, read receipts
- React frontend + Go CLI client
- Cloud deployment on Fly.io

---

## 2. Repository Structure

```
peer-to-peer-chat/
├── server/                    # Go backend
│   ├── main.go                # Entry point, router setup
│   ├── auth/                  # JWT + bcrypt, register/login handlers
│   ├── ws/                    # WebSocket hub, connection manager
│   ├── chat/                  # Message handlers (DM + group)
│   ├── presence/              # Online/offline/typing via Redis
│   ├── signaling/             # WebRTC SDP/ICE relay
│   ├── db/                    # PostgreSQL queries (sqlc generated)
│   │   ├── schema.sql
│   │   ├── queries.sql
│   │   └── sqlc.yaml
│   └── middleware/            # JWT validation, rate limiting
├── web/                       # React + Vite frontend
│   ├── src/
│   │   ├── components/        # ChatWindow, MessageBubble, GroupPanel, ContactList
│   │   ├── hooks/             # useWebSocket, useWebRTC, usePresence, useTyping
│   │   └── pages/             # Login, Chat, Groups, Call
│   └── vite.config.ts
├── cli/                       # Go terminal client
│   └── main.go
├── migrations/                # PostgreSQL schema (golang-migrate)
│   ├── 000001_init.up.sql
│   └── 000001_init.down.sql
├── docker-compose.yml         # Local dev: server + postgres + redis
├── Dockerfile                 # Multi-stage Go build
├── fly.toml                   # Fly.io deploy config
└── README.md
```

---

## 3. Tech Stack

| Layer | Technology | Reason |
|-------|-----------|--------|
| Server language | Go 1.23 | Goroutines, strong concurrency, compiled binary |
| HTTP/WebSocket | `gin` + `gorilla/websocket` | Battle-tested, performant |
| Auth | JWT (`golang-jwt/jwt`), bcrypt | Industry standard |
| Database | PostgreSQL 16 | ACID, JSON support, full-text search |
| DB queries | `pgx/v5` | Type-safe hand-written queries, no ORM overhead |
| DB migrations | `golang-migrate` | Version-controlled schema |
| Cache/PubSub | Redis 7, `go-redis/v9` | Pub/sub, presence, rate limiting, typing |
| Frontend | React 18, Vite, Tailwind CSS | Fast dev, modern stack |
| CLI client | Go, `gorilla/websocket` | Same language as server |
| Deploy | Render (server), Supabase (Postgres), Upstash (Redis), Netlify (frontend) | Free tier, no credit card |

---

## 4. Authentication

### Registration & Login
- `POST /auth/register` — accepts `{username, password}`, stores `bcrypt(password, cost=12)`, returns signed JWT
- `POST /auth/login` — verifies bcrypt hash, returns JWT
- JWT payload: `{user_id: uuid, username: string, exp: unix}`
- JWT secret loaded from environment variable `JWT_SECRET`
- Tokens expire after 7 days

### CORS
All routes include CORS middleware allowing `*` origin — required for Netlify frontend to call the Render backend:
```go
c.Header("Access-Control-Allow-Origin", "*")
c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
// OPTIONS preflight → 204 No Content
```

### WebSocket Auth
- Client connects to `GET /ws?token=<jwt>`
- Middleware validates JWT before upgrading connection
- `user_id` and `username` bound to connection context — never trusted from client message payload

### No username-only trust
The current system trusts `data.get("sender")` in every message — a complete auth bypass. In the new system, sender identity is always derived from the validated JWT on the connection.

---

## 5. WebSocket Hub Architecture

```
Hub (single goroutine — owns clients map)
├── register   chan *Client   — new authenticated connection
├── unregister chan *Client   — connection closed
└── route      chan Message   — inbound message to be routed

Client (one per connection)
├── user_id, username         — from JWT
├── send      chan []byte     — outbound message queue
├── readPump  goroutine       — reads WebSocket → sends to Hub.route
└── writePump goroutine       — reads Client.send → writes to WebSocket
```

- Hub owns the `clients map[uuid]*Client` — no mutex needed (single goroutine)
- `readPump` and `writePump` are the only goroutines touching the WebSocket connection
- Each client gets a buffered `send` channel (size 256); if full, connection is dropped

### Cross-Instance Routing (Redis Pub/Sub)

When the target user is connected to a different server instance:

```
Instance 1 (sender connected)
  → INSERT message to PostgreSQL
  → PUBLISH to Redis channel "chat:user:{receiver_id}"

Instance 2 (receiver connected)
  → subscribed to "chat:user:{receiver_id}"
  → receives published message
  → pushes to receiver's WebSocket via Client.send channel
```

Each instance subscribes to a channel per connected user on connect and unsubscribes on disconnect.

---

## 6. Message Protocol (JSON over WebSocket)

All messages are JSON. `type` field determines routing.

**Client → Server:**
```json
{"type": "message",       "to": "<user_id>",   "body": "hello",    "id": "<client-uuid>"}
{"type": "group_message", "to": "<group_id>",  "body": "hello",    "id": "<client-uuid>"}
{"type": "typing",        "to": "<chat_id>"}
{"type": "ack",           "message_id": "<uuid>"}
{"type": "webrtc_offer",  "to": "<user_id>",   "sdp": "..."}
{"type": "webrtc_answer", "to": "<user_id>",   "sdp": "..."}
{"type": "ice_candidate", "to": "<user_id>",   "candidate": "..."}
{"type": "ping"}
```

**Server → Client (push):**
```json
{"type": "message",     "from": "<user_id>", "body": "...", "id": "<uuid>", "timestamp": "..."}
{"type": "delivered",   "message_id": "<uuid>"}
{"type": "read",        "message_id": "<uuid>", "by": "<user_id>"}
{"type": "typing",      "from": "<user_id>", "chat_id": "<uuid>"}
{"type": "presence",    "user_id": "<uuid>", "status": "online|offline", "last_seen": "..."}
{"type": "webrtc_offer","from": "<user_id>", "sdp": "..."}
{"type": "pong"}
```

---

## 7. Database Schema

```sql
-- migrations/000001_init.up.sql

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    username      TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE chats (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE chat_members (
    chat_id UUID REFERENCES chats(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (chat_id, user_id)
);

CREATE TABLE messages (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    chat_id    UUID REFERENCES chats(id) ON DELETE CASCADE,
    sender_id  UUID REFERENCES users(id),
    body       TEXT NOT NULL,
    status     TEXT NOT NULL DEFAULT 'sent', -- sent | delivered | read
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX idx_messages_chat_time ON messages(chat_id, created_at DESC);
CREATE INDEX idx_messages_sender    ON messages(sender_id);

CREATE TABLE groups (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name       TEXT UNIQUE NOT NULL,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE group_members (
    group_id UUID REFERENCES groups(id) ON DELETE CASCADE,
    user_id  UUID REFERENCES users(id)  ON DELETE CASCADE,
    role     TEXT NOT NULL DEFAULT 'member', -- member | admin
    joined_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (group_id, user_id)
);

CREATE TABLE group_messages (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    group_id   UUID REFERENCES groups(id) ON DELETE CASCADE,
    sender_id  UUID REFERENCES users(id),
    body       TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX idx_group_messages_group_time ON group_messages(group_id, created_at DESC);

CREATE TABLE read_receipts (
    message_id UUID REFERENCES messages(id) ON DELETE CASCADE,
    reader_id  UUID REFERENCES users(id),
    read_at    TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (message_id, reader_id)
);
```

Key improvements over current schema:
- UUIDs instead of auto-increment ints (no enumeration attacks)
- `status` column on messages for delivery tracking
- Composite indexes on `(chat_id, created_at)` — critical for pagination queries
- `ON DELETE CASCADE` throughout — no orphan rows
- `role` column on group_members replacing the implicit creator check

---

## 8. Message Delivery Pipeline

```
1. Client A sends {type:"message", to:B_id, body:"...", id:"<client-uuid>"}

2. Server middleware: validate JWT → extract sender_id

3. Rate limit check:
   INCR rate:{sender_id}:msgs (EXPIRE 60s if new key)
   If count > 30: return {type:"error", code:"rate_limited"}

4. Find or create chat between A and B (upsert chat_members)

5. INSERT into messages table (async via pgxpool)
   → message_id = server-generated UUID

6. Local delivery attempted via hub.Send(receiverID):
   → If receiver is on THIS instance (hub.Send returns true):
       Message delivered directly to B's WebSocket
       UPDATE messages SET status='delivered' immediately (prevents reconnect re-delivery)
   → If receiver is on ANOTHER instance (hub.Send returns false):
       PUBLISH chat:user:{B_id} <serialized message> via Redis pub/sub
       Message stays 'sent' until B's instance marks it delivered

7. Server returns {type:"sent", message_id:"<uuid>", id:"<client-uuid>"} to A
   (client-uuid echoed back for client-side dedup/correlation)

**Reconnect flow:**
On WebSocket connect, server queries:
```sql
SELECT m.*, u.username FROM messages m
JOIN chat_members cm ON cm.chat_id = m.chat_id
JOIN users u ON u.id = m.sender_id
WHERE cm.user_id = $1 AND m.sender_id != $1 AND m.status = 'sent'
ORDER BY m.created_at ASC LIMIT 100
```
Pushes all pending messages, then marks each as `delivered`.
Only messages with `status='sent'` are re-delivered — messages already delivered locally are skipped (duplicate prevention).
```

**Reconnect flow:**
```sql
SELECT * FROM messages m
JOIN chat_members cm ON cm.chat_id = m.chat_id
WHERE cm.user_id = $1 AND m.sender_id != $1 AND m.status = 'sent'
ORDER BY m.created_at ASC
LIMIT 100
```
Push all, then batch-update to `delivered`.

---

## 9. Redis Key Space

```
presence:{user_id}             STRING  "online"    TTL 30s (heartbeat refreshes)
typing:{chat_id}:{user_id}     STRING  "1"         TTL 5s  (ephemeral, no stop event needed)
rate:{user_id}:msgs            STRING  count        TTL 60s (sliding window)
chat:user:{user_id}            CHANNEL (pub/sub, no stored data)
```

---

## 10. Presence & Typing

**Presence:**
- On WebSocket connect (post-auth): `SET presence:{user_id} online EX 30`
- Client sends `{type:"ping"}` every 20s → server responds `{type:"pong"}` + refreshes TTL
- On WebSocket disconnect: `DEL presence:{user_id}`, UPDATE `users.last_seen = NOW()`
- Fan out to all of user's contacts: `{type:"presence", user_id:"...", status:"offline", last_seen:"..."}`
- Contact list queries: `MGET presence:{id1} presence:{id2} ...` in one Redis call

**Typing:**
- Client sends `{type:"typing", to:chat_id}` (debounced: no more than once per 3s)
- Server: `SET typing:{chat_id}:{sender_id} 1 EX 5`
- Server fans out to other chat members: `{type:"typing", from:username, chat_id:...}`
- Auto-expires after 5s — no "stopped typing" message needed
- No DB writes for typing — purely ephemeral Redis

---

## 11. WebRTC Signaling

The Go server acts as a signaling relay only. After handshake, data flows P2P.

```
A connects, wants DataChannel to B:

1. A → Server: {type:"webrtc_offer",  to:B_id, sdp:"<offer SDP>"}
2. Server → B:  {type:"webrtc_offer",  from:A_id, sdp:"<offer SDP>"}
3. B → Server:  {type:"webrtc_answer", to:A_id, sdp:"<answer SDP>"}
4. Server → A:  {type:"webrtc_answer", from:B_id, sdp:"<answer SDP>"}
5. A ↔ Server ↔ B: ICE candidate exchange (multiple messages)
6. WebRTC DataChannel established A ↔ B directly
7. File transfer / voice / custom data flows P2P, never touches server
```

React client STUN config:
```js
const pc = new RTCPeerConnection({
  iceServers: [{ urls: "stun:stun.l.google.com:19302" }]
})
```

No TURN server in initial deployment (95% of connections succeed with STUN alone). TURN can be added later via Coturn on a VPS if needed.

---

## 12. React Frontend

### Pages
- `/login` — username + password form, JWT stored in `localStorage`
- `/chat` — split panel: contact list (left, with presence dots + unread counts) + chat window (right, with message bubbles, typing indicator, read receipts)
- `/groups` — group list + group chat window with member management
- `/call/:user_id` — WebRTC DataChannel file transfer UI

### Key Hooks
```typescript
useWebSocket(token: string)
  // Manages WS connection lifecycle, reconnect with backoff, message dispatch

usePresence(contactIds: string[])
  // Subscribes to presence updates, returns {[userId]: "online"|"offline"|Date}

useTyping(chatId: string)
  // Sends debounced typing events, tracks incoming typing indicators

useWebRTC(targetUserId: string)
  // RTCPeerConnection lifecycle: offer/answer/ICE, DataChannel for file transfer
```

### State Persistence
Zustand store uses `persist` middleware — auth token, DM messages, and contacts are saved to `localStorage` under key `p2p-chat-store`. State survives page refresh without re-fetching from server.

**Critical selector pattern:** Zustand selectors must not return new object/array references on every render. Use:
```ts
// Correct — selector returns stable reference
const messages = useStore((s) => s.dmMessages[partnerId]) ?? []
// Wrong — creates new [] on every render → infinite re-render loop
const messages = useStore((s) => s.dmMessages[partnerId] ?? [])
```

### Tailwind UI components
- `MessageBubble` — sent/received styling, timestamp, read receipt tick (✓/✓✓)
- `TypingIndicator` — animated dots, auto-hide after typing TTL
- `PresenceDot` — green (online) / grey (offline) with last seen tooltip
- `ContactList` — sorted by last message time, unread badge counts

---

## 13. Go CLI Client

Connects to `wss://<app>.fly.dev/ws?token=<jwt>`.

Commands:
```
register <username> <password>   — calls POST /auth/register, saves JWT
login <username> <password>      — calls POST /auth/login, saves JWT
send <username> <message>        — sends DM
group <groupname> <message>      — sends group message
history <username> [limit]       — fetches chat history (REST endpoint)
status                           — shows online contacts
create-group <name>              — creates group
add <groupname> <username>       — adds member to group
```

Useful for:
- Demonstrating the protocol without a browser
- Load testing with multiple CLI instances
- Showing server logs during demos

---

## 14. Deployment

### Local Dev
```bash
docker-compose up   # starts server + postgres + redis
```

`docker-compose.yml` services: `server` (Go binary), `postgres:16`, `redis:7-alpine`

### Production (Free Tier — No Credit Card Required)

| Service | Role | Notes |
|---------|------|-------|
| Render | Go server | Docker runtime, PORT=10000, free web service |
| Supabase | PostgreSQL | Use Session pooler URL (IPv4) not Direct URL (IPv6) |
| Upstash | Redis | Use `rediss://` TLS URL |
| Netlify | React frontend | Build: `npm run build`, publish: `dist/` |

**Server env vars (Render):**
```
DATABASE_URL=<supabase session pooler url>
REDIS_URL=<upstash rediss:// url>
JWT_SECRET=<random 32-byte hex>
ENV=production
PORT=10000
```

**Frontend env vars (Netlify):**
```
VITE_WS_URL=wss://<app>.onrender.com
VITE_API_URL=https://<app>.onrender.com
```

Note: Render free tier spins down after 15 min inactivity. First request after cold start takes ~30–50s. `render.yaml` in repo root configures the service.

### Dockerfile (multi-stage)
```dockerfile
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY server/ .
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o chat-server .

FROM gcr.io/distroless/static-debian12
COPY --from=builder /app/chat-server /chat-server
EXPOSE 8080
CMD ["/chat-server"]
```

Note: `go mod tidy` instead of `go mod download` — regenerates `go.sum` during build to avoid missing dependency entries. Uses `distroless/static-debian12` (not `base`) for a fully static binary.

---

## 15. Environment Variables

```
PORT            8080
DATABASE_URL    postgres://user:pass@host/dbname
REDIS_URL       redis://host:6379
JWT_SECRET      <random 32-byte hex>
ENV             development|production
```

---

## 16. What This Demonstrates (Resume Narrative)

- **Go concurrency:** goroutine-per-connection model, channel-based hub with no mutex on hot path
- **Distributed systems:** Redis pub/sub for cross-instance message routing — system scales horizontally by adding WS server instances
- **Real-time architecture:** fan-out delivery, offline queue, delivery status tracking
- **WebRTC:** signaling server implementation, ICE/STUN NAT traversal, P2P DataChannel
- **Production readiness:** JWT auth, bcrypt, rate limiting, TLS, Docker, cloud deployment, health checks
- **Database design:** UUID PKs, composite indexes, cascading deletes, `sqlc` type-safe queries
- **Full stack:** Go backend + React frontend + Go CLI, all speaking the same WebSocket protocol
