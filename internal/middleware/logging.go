// File: internal/middleware/logging.go
// Usage: Request logging (one JSON line per request) and panic recovery.

package middleware

import (
	"errors"
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"

	"oxus-df-backend/internal/platform/httpx"
	"oxus-df-backend/internal/platform/reqctx"
)

var errPanic = errors.New("panic recovered")

// Logging writes one structured log line per request. Query strings are never
// logged because they can carry sensitive values.
func Logging(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		status := c.Writer.Status()
		level := slog.LevelInfo
		switch {
		case status >= 500:
			level = slog.LevelError
		case status >= 400:
			level = slog.LevelWarn
		}

		attrs := []slog.Attr{
			slog.String("request_id", reqctx.From(c.Request.Context()).RequestID),
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.String("route", c.FullPath()),
			slog.Int("status", status),
			slog.Duration("latency", time.Since(start)),
			slog.String("client_ip", c.ClientIP()),
		}
		if len(c.Errors) > 0 {
			attrs = append(attrs, slog.String("errors", c.Errors.String()))
		}
		log.LogAttrs(c.Request.Context(), level, "http request", attrs...)
	}
}

// Recovery turns panics into a 500 response and logs the stack trace.
func Recovery(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				log.ErrorContext(c.Request.Context(), "panic recovered",
					"panic", r,
					"stack", string(debug.Stack()),
					"request_id", reqctx.From(c.Request.Context()).RequestID,
				)
				httpx.WriteError(c, errPanic)
			}
		}()
		c.Next()
	}
}
