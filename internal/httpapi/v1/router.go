// File: internal/httpapi/v1/router.go
// Usage: Builds the Gin router: global middleware, health/metrics routes and the generated /api/v1 routes.

package v1

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"

	"oxus-df-backend/internal/auth"
	"oxus-df-backend/internal/config"
	"oxus-df-backend/internal/httpapi/v1/gen"
	"oxus-df-backend/internal/middleware"
	"oxus-df-backend/internal/platform/httpx"
	"oxus-df-backend/internal/products"
)

const (
	basePath       = "/api/v1"
	maxBodyBytes   = 1 << 20 // 1 MiB
	readyzDeadline = 2 * time.Second
)

// Deps are everything the router needs; built in cmd/api.
type Deps struct {
	Log      *slog.Logger
	Config   *config.Config
	Auth     *auth.Handler
	Products *products.Handler
	Verifier middleware.TokenVerifier
	Limiter  middleware.HitCounter
	Metrics  *middleware.Metrics
	Ready    func(ctx context.Context) error
}

// NewRouter builds the Gin engine: global middleware, health and metrics
// endpoints, and the generated /api/v1 routes.
func NewRouter(d Deps) *gin.Engine {
	if d.Config.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}
	binding.EnableDecoderDisallowUnknownFields = true

	r := gin.New()
	// An empty list means "trust no proxy"; set OXUS_HTTP_TRUSTED_PROXIES behind Nginx.
	if err := r.SetTrustedProxies(d.Config.HTTPTrustedProxies); err != nil {
		d.Log.Error("invalid trusted proxies, trusting none", "error", err)
		_ = r.SetTrustedProxies(nil)
	}

	r.Use(
		middleware.RequestID(),
		middleware.Recovery(d.Log),
		middleware.Logging(d.Log),
		d.Metrics.Middleware(),
		middleware.BodyLimit(maxBodyBytes),
		middleware.RateLimit(d.Limiter, d.Config.RateLimitRequests, d.Config.RateLimitWindow, d.Log,
			"/healthz", "/readyz", "/metrics"),
	)

	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	r.GET("/readyz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), readyzDeadline)
		defer cancel()
		if err := d.Ready(ctx); err != nil {
			d.Log.WarnContext(ctx, "readiness check failed", "error", err)
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})
	r.GET("/metrics", gin.WrapH(d.Metrics.Handler()))

	server := &Server{auth: d.Auth, products: d.Products, minAppVersion: d.Config.AppMinVersion}
	gen.RegisterHandlersWithOptions(r, server, gen.GinServerOptions{
		BaseURL:      basePath,
		Middlewares:  []gen.MiddlewareFunc{gen.MiddlewareFunc(middleware.Authenticate(d.Verifier))},
		ErrorHandler: httpx.ParamErrorHandler,
	})

	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "not_found", "message": "route not found"}})
	})
	return r
}
