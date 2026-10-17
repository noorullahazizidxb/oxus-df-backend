// File: cmd/api/main.go
// Usage: Starts the HTTP API: loads config, connects to PostgreSQL and Valkey, wires the modules together and serves requests.

// Command api runs the HTTP API (composition root: wires modules together).
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"oxus-df-backend/internal/audit"
	"oxus-df-backend/internal/auth"
	"oxus-df-backend/internal/config"
	httpv1 "oxus-df-backend/internal/httpapi/v1"
	"oxus-df-backend/internal/middleware"
	"oxus-df-backend/internal/platform/database"
	"oxus-df-backend/internal/platform/valkey"
	"oxus-df-backend/internal/products"
)

const shutdownTimeout = 15 * time.Second

// main starts the API and exits with code 1 if it fails.
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "api:", err)
		os.Exit(1)
	}
}

// run wires every dependency, starts the HTTP server and shuts it down gracefully on Ctrl+C or SIGTERM.
func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := newLogger(cfg)
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.NewPool(ctx, cfg.DatabaseURL, cfg.DatabaseMaxConns)
	if err != nil {
		return err
	}
	defer pool.Close()
	db := database.New(pool)

	rdb, err := valkey.NewClient(ctx, cfg.ValkeyURL)
	if err != nil {
		return err
	}
	defer func() { _ = rdb.Close() }()
	limiter := valkey.NewLimiter(rdb)

	// Insert-only River client: the API enqueues jobs, cmd/worker runs them.
	riverClient, err := river.NewClient[pgx.Tx](riverpgxv5.New(pool), &river.Config{})
	if err != nil {
		return fmt.Errorf("create river client: %w", err)
	}

	// Modules.
	auditSvc := audit.NewService(audit.NewPgRepository(db))

	tokens := auth.NewTokenManager(cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTTTL)
	authSvc := auth.NewService(auth.NewPgRepository(db), limiter, tokens, auditSvc, log, auth.Policy{
		MaxAttempts: cfg.LoginMaxAttempts,
		LockWindow:  cfg.LoginLockWindow,
	})
	if err := authSvc.EnsureBootstrapAdmin(ctx, cfg.BootstrapAdminEmail, cfg.BootstrapAdminPassword); err != nil {
		return fmt.Errorf("bootstrap admin: %w", err)
	}

	productsSvc := products.NewService(
		products.NewPgRepository(db),
		db,
		auditSvc,
		products.NewRiverPublisher(riverClient),
	)

	router := httpv1.NewRouter(httpv1.Deps{
		Log:      log,
		Config:   cfg,
		Auth:     auth.NewHandler(authSvc),
		Products: products.NewHandler(productsSvc),
		Verifier: tokens,
		Limiter:  limiter,
		Metrics:  middleware.NewMetrics(),
		Ready: func(ctx context.Context) error {
			if err := db.Ping(ctx); err != nil {
				return fmt.Errorf("database: %w", err)
			}
			if err := rdb.Ping(ctx).Err(); err != nil {
				return fmt.Errorf("valkey: %w", err)
			}
			return nil
		},
	})

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// A channel is Go's typed way to pass a value between goroutines. Buffering
	// one result lets this goroutine report a startup failure without waiting
	// for the select below to begin receiving.
	serveErr := make(chan error, 1)
	go func() {
		log.Info("api listening", "addr", cfg.HTTPAddr, "env", cfg.Env)
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
	}()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
		log.Info("shutting down")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

// newLogger creates a JSON logger using the level from the config.
func newLogger(cfg *config.Config) *slog.Logger {
	var level slog.Level
	if err := level.UnmarshalText([]byte(cfg.LogLevel)); err != nil {
		level = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}
