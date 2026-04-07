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
func Publish(ctx context.Context, rdb *redis.Client, userID uuid.UUID, data []byte) error {
	return rdb.Publish(ctx, channelName(userID), data).Err()
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

// Publish satisfies chat.Router interface
func (r *Router) Publish(ctx context.Context, userID uuid.UUID, data []byte) error {
	return Publish(ctx, r.rdb, userID, data)
}
