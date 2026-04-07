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
