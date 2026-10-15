// File: internal/auth/principal.go
// Usage: Stores and reads the logged-in caller (principal) from the request context.

package auth

import (
	"context"

	"github.com/gin-gonic/gin"

	"oxus-df-backend/internal/apperr"
)

// principalKey is the private context key under which the caller is stored.
type principalKey struct{}

// WithPrincipal stores the authenticated caller in ctx.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// PrincipalFrom returns the authenticated caller, if any.
func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}

// PrincipalFromGin is the handler-side shortcut: it fails with 401 when the
// auth middleware did not run for the route.
func PrincipalFromGin(c *gin.Context) (Principal, error) {
	p, ok := PrincipalFrom(c.Request.Context())
	if !ok {
		return Principal{}, apperr.Unauthorized("authentication required")
	}
	return p, nil
}
