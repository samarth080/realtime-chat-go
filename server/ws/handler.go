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

// ConnectHook is called after a client connects (use for presence, pubsub subscribe)
type ConnectHook func(userID uuid.UUID)

// DisconnectHook is called after a client disconnects (use for presence, pubsub unsubscribe)
type DisconnectHook func(userID uuid.UUID)

// ServeWS handles GET /ws — requires JWTAuth middleware to have set user_id and username
func ServeWS(hub *Hub, dispatcher Dispatcher, onConnect ConnectHook, onDisconnect DisconnectHook) gin.HandlerFunc {
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

		ip := c.GetHeader("X-Forwarded-For")
		if ip == "" {
			ip = c.ClientIP()
		}
		log.Printf("connect: user=%s ip=%s", username, ip)

		client := newRealClient(userID, username)
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
