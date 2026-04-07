package presence_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/samarth080/peer-chat/presence"
	"github.com/stretchr/testify/require"
)

func testRedis(t *testing.T) *redis.Client {
	t.Helper()
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	require.NoError(t, rdb.Ping(context.Background()).Err())
	t.Cleanup(func() { rdb.Close() })
	return rdb
}

func TestSetAndIsOnline(t *testing.T) {
	rdb := testRedis(t)
	ctx := context.Background()
	id := uuid.New()

	require.NoError(t, presence.SetOnline(ctx, rdb, id))
	t.Cleanup(func() { presence.SetOffline(ctx, rdb, id) })
	require.True(t, presence.IsOnline(ctx, rdb, id))

	require.NoError(t, presence.SetOffline(ctx, rdb, id))
	require.False(t, presence.IsOnline(ctx, rdb, id))
}

func TestSetOnline_TTLRefresh(t *testing.T) {
	rdb := testRedis(t)
	ctx := context.Background()
	id := uuid.New()
	t.Cleanup(func() { presence.SetOffline(ctx, rdb, id) })

	require.NoError(t, presence.SetOnline(ctx, rdb, id))
	ttl := rdb.TTL(ctx, "presence:"+id.String()).Val()
	require.Greater(t, ttl, 25*time.Second)
	require.LessOrEqual(t, ttl, 31*time.Second)
}

func TestGetPresenceBatch(t *testing.T) {
	rdb := testRedis(t)
	ctx := context.Background()

	onlineID := uuid.New()
	offlineID := uuid.New()
	require.NoError(t, presence.SetOnline(ctx, rdb, onlineID))
	t.Cleanup(func() { presence.SetOffline(ctx, rdb, onlineID) })

	results, err := presence.GetPresenceBatch(ctx, rdb, []uuid.UUID{onlineID, offlineID})
	require.NoError(t, err)
	require.True(t, results[onlineID])
	require.False(t, results[offlineID])
}
