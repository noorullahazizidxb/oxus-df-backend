// File: internal/audit/repository.go
// Usage: Saves audit entries into the audit_logs table.

package audit

import (
	"context"
	"fmt"

	"oxus-df-backend/internal/platform/database"
	"oxus-df-backend/internal/platform/database/sqlc"
)

// PgRepository stores audit entries in PostgreSQL.
type PgRepository struct {
	db *database.DB
}

// NewPgRepository creates the PostgreSQL audit repository.
func NewPgRepository(db *database.DB) *PgRepository { return &PgRepository{db: db} }

// Insert writes one audit row, inside the current transaction if there is one.
func (r *PgRepository) Insert(ctx context.Context, rec record) error {
	err := r.db.Queries(ctx).InsertAuditLog(ctx, sqlc.InsertAuditLogParams{
		ActorID:    rec.ActorID,
		Action:     rec.Action,
		EntityType: rec.EntityType,
		EntityID:   rec.EntityID,
		RequestID:  rec.RequestID,
		ClientIp:   rec.ClientIP,
		Metadata:   rec.Metadata,
	})
	if err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}
	return nil
}
