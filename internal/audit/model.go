// File: internal/audit/model.go
// Usage: Data types for the audit trail (who did what).

// Package audit records who did what. Entries are written through the same
// transaction as the change they describe (see database.WithinTx).
package audit

import "github.com/google/uuid"

// Entry is what a caller supplies. Request ID and client IP are filled from ctx.
type Entry struct {
	ActorID    *uuid.UUID
	Action     string
	EntityType string
	EntityID   *uuid.UUID
	Metadata   map[string]any
}

// record is the persisted form.
type record struct {
	ActorID    *uuid.UUID
	Action     string
	EntityType string
	EntityID   *uuid.UUID
	RequestID  string
	ClientIP   string
	Metadata   []byte
}
