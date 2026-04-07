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
