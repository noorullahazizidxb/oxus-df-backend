// File: internal/middleware/auth.go
// Usage: Checks the Bearer token on protected routes and stores the caller in the request context.

package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	"oxus-df-backend/internal/apperr"
	"oxus-df-backend/internal/auth"
	"oxus-df-backend/internal/httpapi/v1/gen"
	"oxus-df-backend/internal/platform/httpx"
)

// TokenVerifier is satisfied by *auth.TokenManager.
type TokenVerifier interface {
	Parse(raw string) (auth.Principal, error)
}

// Authenticate is registered as a generated-server middleware. The generated
// wrapper marks operations that declare `security: bearerAuth` in the OpenAPI
// file, so public operations (login, app config) need no allow-list here.
func Authenticate(v TokenVerifier) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, secured := c.Get(gen.BearerAuthScopes); !secured {
			return
		}

		raw, ok := bearerToken(c.GetHeader("Authorization"))
		if !ok {
			httpx.WriteError(c, apperr.Unauthorized("missing bearer token"))
			return
		}
		principal, err := v.Parse(raw)
		if err != nil {
			httpx.WriteError(c, err)
			return
		}
		c.Request = c.Request.WithContext(auth.WithPrincipal(c.Request.Context(), principal))
	}
}

// bearerToken extracts the token from an "Authorization: Bearer <token>" header.
func bearerToken(header string) (string, bool) {
	scheme, token, found := strings.Cut(header, " ")
	if !found || !strings.EqualFold(scheme, "Bearer") || token == "" {
		return "", false
	}
	return token, true
}
