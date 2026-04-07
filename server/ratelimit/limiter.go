package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type Limiter struct {
	rdb     *redis.Client
	max     int64
	windowS int
}

func New(rdb *redis.Client, max int64, windowSeconds int) *Limiter {
	return &Limiter{rdb: rdb, max: max, windowS: windowSeconds}
}

func (l *Limiter) Allow(ctx context.Context, userID uuid.UUID) bool {
	key := fmt.Sprintf("rate:%s", userID)
	pipe := l.rdb.Pipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, time.Duration(l.windowS)*time.Second)
	pipe.Exec(ctx)

	return incr.Val() <= l.max
}
