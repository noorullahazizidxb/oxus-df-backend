// File: internal/products/jobs.go
// Usage: Background job that runs after a product changes, and the publisher that queues it.

package products

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"oxus-df-backend/internal/platform/database"
)

// ChangedArgs is the River job enqueued whenever a product changes. It is the
// hook for follow-up work that must never be lost, for example pushing the
// product to the core banking system (wrap that call with sony/gobreaker).
type ChangedArgs struct {
	ProductID uuid.UUID `json:"product_id"`
	Action    string    `json:"action"`
}

// Kind is the unique job name River uses to find the right worker.
func (ChangedArgs) Kind() string { return "products.changed" }

// ChangedWorker runs in cmd/worker.
type ChangedWorker struct {
	river.WorkerDefaults[ChangedArgs]
	log *slog.Logger
}

// NewChangedWorker creates the worker for products.changed jobs.
func NewChangedWorker(log *slog.Logger) *ChangedWorker { return &ChangedWorker{log: log} }

// Work runs one job. Today it only logs; add follow-up work (for example syncing to the core banking system) here.
func (w *ChangedWorker) Work(ctx context.Context, job *river.Job[ChangedArgs]) error {
	w.log.InfoContext(ctx, "product changed",
		"product_id", job.Args.ProductID,
		"action", job.Args.Action,
		"attempt", job.Attempt,
	)
	return nil
}

// RiverPublisher implements EventPublisher. Jobs are inserted in the same
// database transaction as the business write, so a job exists if and only if
// the change was committed.
type RiverPublisher struct {
	client *river.Client[pgx.Tx]
}

// NewRiverPublisher creates a publisher that queues jobs through River.
func NewRiverPublisher(client *river.Client[pgx.Tx]) *RiverPublisher {
	return &RiverPublisher{client: client}
}

// ProductChanged queues a products.changed job inside the current transaction.
func (p *RiverPublisher) ProductChanged(ctx context.Context, id uuid.UUID, action string) error {
	tx, ok := database.TxFromContext(ctx)
	if !ok {
		return errors.New("products: ProductChanged must be called inside a transaction")
	}
	if _, err := p.client.InsertTx(ctx, tx, ChangedArgs{ProductID: id, Action: action}, nil); err != nil {
		return fmt.Errorf("enqueue products.changed: %w", err)
	}
	return nil
}
