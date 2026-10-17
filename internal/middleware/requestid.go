// File: internal/middleware/requestid.go
// Usage: Gives every request an ID and limits the request body size.

// Package middleware holds the cross-cutting HTTP concerns: request IDs,
// logging, panic recovery, rate limiting, metrics, authentication.
package middleware

import (
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"oxus-df-backend/internal/platform/reqctx"
)

const RequestIDHeader = "X-Request-ID"

var validRequestID = regexp.MustCompile(`^[A-Za-z0-9._-]{8,64}$`)

// RequestID accepts a well-formed incoming ID or generates one, echoes it in
// the response and stores it (with the client IP) in the request context.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(RequestIDHeader)
		if !validRequestID.MatchString(id) {
			id = uuid.NewString()
		}
		c.Header(RequestIDHeader, id)
		ctx := reqctx.With(c.Request.Context(), reqctx.Info{RequestID: id, ClientIP: c.ClientIP()})
		// Contexts and requests are immutable-style values: WithContext returns
		// a derived request, so middleware must assign it for later handlers.
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// BodyLimit rejects request bodies larger than max bytes.
func BodyLimit(limit int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		c.Next()
	}
}
