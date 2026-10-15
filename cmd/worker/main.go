// File: cmd/worker/main.go
// Usage: Starts the background worker that runs queued jobs (River). Also runs `worker migrate` to create the job tables.

// Command worker runs background jobs (River). `worker migrate` applies
// River's own database migrations and exits.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"

	"oxus-df-backend/internal/config"
	"oxus-df-backend/internal/platform/database"
	"oxus-df-backend/internal/products"
)

const (
	maxWorkers      = 20
	shutdownTimeout = 30 * time.Second
)

// main starts the worker and exits with code 1 if it fails.
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "worker:", err)
		os.Exit(1)
	}
}

// run starts the job worker, or applies job-table migrations when called with the "migrate" argument.
func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	var level slog.Level
	if err := level.UnmarshalText([]byte(cfg.LogLevel)); err != nil {
		level = slog.LevelInfo
	}
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.NewPool(ctx, cfg.DatabaseURL, cfg.DatabaseMaxConns)
	if err != nil {
		return err
	}
	defer pool.Close()

	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		return migrateRiver(ctx, pool, log)
	}

	workers := river.NewWorkers()
	river.AddWorker(workers, products.NewChangedWorker(log))

	client, err := river.NewClient[pgx.Tx](riverpgxv5.New(pool), &river.Config{
		Logger:  log,
		Workers: workers,
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: maxWorkers},
		},
	})
	if err != nil {
		return fmt.Errorf("create river client: %w", err)
	}

	// Start with a background context: cancelling it would hard-stop running
	// jobs. Graceful shutdown is done with Stop below.
	if err := client.Start(context.Background()); err != nil {
		return fmt.Errorf("start river: %w", err)
	}
	log.Info("worker started")

	<-ctx.Done()
	log.Info("worker shutting down")

	stopCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	return client.Stop(stopCtx)
}

// migrateRiver creates or updates the tables River needs to store jobs.
func migrateRiver(ctx context.Context, pool *pgxpool.Pool, log *slog.Logger) error {
	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		return fmt.Errorf("create river migrator: %w", err)
	}
	res, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil)
	if err != nil {
		return fmt.Errorf("run river migrations: %w", err)
	}
	log.Info("river migrations applied", "count", len(res.Versions))
	return nil
}
