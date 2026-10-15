// File: internal/middleware/ratelimit.go
// Usage: Limits how many requests one client IP can make in a time window.

package middleware

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"oxus-df-backend/internal/apperr"
	"oxus-df-backend/internal/platform/httpx"
)

// HitCounter is satisfied by valkey.Limiter.
type HitCounter interface {
	Hit(ctx context.Context, key string, window time.Duration) (int64, error)
}

// RateLimit allows `limit` requests per client IP per window. If Valkey is
// unavailable it fails open (and logs) so an outage does not take the API down;
// login brute force is protected separately by the account lockout.
func RateLimit(counter HitCounter, limit int64, window time.Duration, log *slog.Logger, skipPaths ...string) gin.HandlerFunc {
	skip := make(map[string]struct{}, len(skipPaths))
	for _, p := range skipPaths {
		skip[p] = struct{}{}
	}

	return func(c *gin.Context) {
		if _, ok := skip[c.Request.URL.Path]; ok {
			c.Next()
			return
		}

		n, err := counter.Hit(c.Request.Context(), "ip:"+c.ClientIP(), window)
		if err != nil {
			log.WarnContext(c.Request.Context(), "rate limiter unavailable", "error", err)
			c.Next()
			return
		}
		if n > limit {
			c.Header("Retry-After", strconv.Itoa(int(window.Seconds())))
			httpx.WriteError(c, apperr.TooManyRequests("rate limit exceeded"))
			return
		}
		c.Next()
	}
}
