// File: internal/platform/valkey/valkey.go
// Usage: Valkey connection and a counter used for rate limiting and login lockout.

// Package valkey is the adapter to Valkey. It uses the go-redis client, which
// works unchanged with Valkey.
package valkey

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// NewClient connects to Valkey using a redis:// URL and verifies connectivity.
func NewClient(ctx context.Context, url string) (*redis.Client, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("parse valkey url: %w", err)
	}
	client := redis.NewClient(opts)
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping valkey: %w", err)
	}
	return client, nil
}

// Limiter is a fixed-window counter. It backs both the per-IP API rate limit
// and the admin login lockout, so the logic lives in one place.
type Limiter struct {
	rdb *redis.Client
}

// NewLimiter creates a counter on top of the given Valkey client.
func NewLimiter(rdb *redis.Client) *Limiter { return &Limiter{rdb: rdb} }

// redisKey adds a prefix so our keys never clash with other data.
func (l *Limiter) redisKey(key string) string { return "oxus:rl:" + key }

// Hit increments the counter and starts the window on the first hit.
func (l *Limiter) Hit(ctx context.Context, key string, window time.Duration) (int64, error) {
	k := l.redisKey(key)
	// Commands are queued first and executed together by Exec. The returned
	// command object is a Go library pattern: read its value only after Exec.
	pipe := l.rdb.TxPipeline()
	incr := pipe.Incr(ctx, k)
	pipe.ExpireNX(ctx, k, window)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, fmt.Errorf("valkey hit: %w", err)
	}
	return incr.Val(), nil
}

// Count returns the current counter value without changing it.
func (l *Limiter) Count(ctx context.Context, key string) (int64, error) {
	n, err := l.rdb.Get(ctx, l.redisKey(key)).Int64()
	if errors.Is(err, redis.Nil) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("valkey count: %w", err)
	}
	return n, nil
}

// Reset clears the counter.
func (l *Limiter) Reset(ctx context.Context, key string) error {
	if err := l.rdb.Del(ctx, l.redisKey(key)).Err(); err != nil {
		return fmt.Errorf("valkey reset: %w", err)
	}
	return nil
}
