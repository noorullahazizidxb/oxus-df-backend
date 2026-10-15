// File: internal/audit/service.go
// Usage: Audit use-case: adds request ID and client IP to an entry and stores it.

package audit

import (
	"context"
	"encoding/json"
	"fmt"

	"oxus-df-backend/internal/platform/reqctx"
)

// Repository is the storage the audit service needs (faked in tests).
type Repository interface {
	Insert(ctx context.Context, r record) error
}

// Service is the audit use-case.
type Service struct {
	repo Repository
}

// NewService creates the audit service.
func NewService(repo Repository) *Service { return &Service{repo: repo} }

// Record stores an entry. When ctx carries a transaction the entry commits or
// rolls back together with the business change.
func (s *Service) Record(ctx context.Context, e Entry) error {
	meta := e.Metadata
	if meta == nil {
		meta = map[string]any{}
	}
	raw, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshal audit metadata: %w", err)
	}

	info := reqctx.From(ctx)
	return s.repo.Insert(ctx, record{
		ActorID:    e.ActorID,
		Action:     e.Action,
		EntityType: e.EntityType,
		EntityID:   e.EntityID,
		RequestID:  info.RequestID,
		ClientIP:   info.ClientIP,
		Metadata:   raw,
	})
}
