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
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		rdb.Close()
		t.Skipf("Redis not available at localhost:6379: %v", err)
	}
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
