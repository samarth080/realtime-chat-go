package presence

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const ttl = 30 * time.Second

func key(userID uuid.UUID) string {
	return fmt.Sprintf("presence:%s", userID)
}

// SetOnline marks a user as online with a 30s TTL (refreshed by heartbeat)
func SetOnline(ctx context.Context, rdb *redis.Client, userID uuid.UUID) error {
	return rdb.Set(ctx, key(userID), "online", ttl).Err()
}

// SetOffline removes a user's presence key immediately
func SetOffline(ctx context.Context, rdb *redis.Client, userID uuid.UUID) error {
	return rdb.Del(ctx, key(userID)).Err()
}

// IsOnline returns true if the user has an active presence key
func IsOnline(ctx context.Context, rdb *redis.Client, userID uuid.UUID) bool {
	val, err := rdb.Exists(ctx, key(userID)).Result()
	return err == nil && val > 0
}

// GetPresenceBatch returns a map of userID -> isOnline for a slice of user IDs
func GetPresenceBatch(ctx context.Context, rdb *redis.Client, userIDs []uuid.UUID) (map[uuid.UUID]bool, error) {
	if len(userIDs) == 0 {
		return map[uuid.UUID]bool{}, nil
	}

	keys := make([]string, len(userIDs))
	for i, id := range userIDs {
		keys[i] = key(id)
	}

	vals, err := rdb.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}

	result := make(map[uuid.UUID]bool, len(userIDs))
	for i, id := range userIDs {
		result[id] = vals[i] != nil
	}
	return result, nil
}
