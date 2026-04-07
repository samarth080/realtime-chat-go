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
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		rdb.Close()
		t.Skipf("Redis not available at localhost:6379: %v", err)
	}
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
