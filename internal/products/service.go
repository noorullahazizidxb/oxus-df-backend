// File: internal/products/service.go
// Usage: Products business rules: permissions, validation, money rules, audit and job in one transaction.

package products

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"oxus-df-backend/internal/apperr"
	"oxus-df-backend/internal/audit"
	"oxus-df-backend/internal/auth"
	"oxus-df-backend/internal/platform/validation"
)

const (
	ActionCreate = "products.create"
	ActionUpdate = "products.update"
	ActionDelete = "products.delete"
	ActionExport = "products.export"

	entityType     = "product"
	maxPage        = 100_000
	maxPageSize    = 100
	maxSearchChars = 100
)

var (
	errForbidden = apperr.Forbidden("your role is not allowed to perform this action")
	maxMoney     = decimal.RequireFromString("9999999999999999.99") // NUMERIC(18,2)
	hundred      = decimal.NewFromInt(100)
)

// Ports. They are defined here, next to the consumer, so tests can fake them.

// Repository is the product storage the service needs.
type Repository interface {
	Create(ctx context.Context, p Product) (Product, error)
	Get(ctx context.Context, id uuid.UUID) (Product, error)
	List(ctx context.Context, f ListFilter) ([]Product, int64, error)
	ListAll(ctx context.Context) ([]Product, error)
	Update(ctx context.Context, p Product) (Product, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// Transactor runs a function inside one database transaction.
type Transactor interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// Auditor writes audit entries.
type Auditor interface {
	Record(ctx context.Context, e audit.Entry) error
}

// EventPublisher enqueues a background job in the caller's transaction.
type EventPublisher interface {
	ProductChanged(ctx context.Context, id uuid.UUID, action string) error
}

// Service holds the products business rules.
type Service struct {
	repo   Repository
	tx     Transactor
	audit  Auditor
	events EventPublisher
}

// NewService creates the products service with its repository, transaction helper, auditor and event publisher.
func NewService(repo Repository, tx Transactor, auditor Auditor, events EventPublisher) *Service {
	return &Service{repo: repo, tx: tx, audit: auditor, events: events}
}

// Create adds a product. The insert, audit entry and job commit together.
func (s *Service) Create(ctx context.Context, actor auth.Principal, in CreateInput) (Product, error) {
	if !actor.Role.CanWrite() {
		return Product{}, errForbidden
	}

	in.Code = strings.ToUpper(strings.TrimSpace(in.Code))
	in.Name = strings.TrimSpace(in.Name)
	in.Currency = strings.ToUpper(strings.TrimSpace(in.Currency))
	if in.Status == "" {
		in.Status = StatusDraft
	}
	if err := validation.Struct(in); err != nil {
		return Product{}, err
	}
	if err := validateMoney(in.MinAmount, in.MaxAmount, in.AnnualRate); err != nil {
		return Product{}, err
	}

	var created Product
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		created, err = s.repo.Create(ctx, Product{
			Code:        in.Code,
			Name:        in.Name,
			Description: in.Description,
			Currency:    in.Currency,
			MinAmount:   in.MinAmount,
			MaxAmount:   in.MaxAmount,
			AnnualRate:  in.AnnualRate,
			Status:      in.Status,
		})
		if err != nil {
			return err
		}
		return s.record(ctx, actor, ActionCreate, created.ID, map[string]any{"code": created.Code})
	})
	if err != nil {
		return Product{}, err
	}
	return created, nil
}

// Get returns one product.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (Product, error) {
	return s.repo.Get(ctx, id)
}

// List returns one page of products.
func (s *Service) List(ctx context.Context, q ListQuery) (ListResult, error) {
	var fields []apperr.Field
	if q.Page < 1 || q.Page > maxPage {
		fields = append(fields, apperr.Field{Name: "page", Message: "must be between 1 and 100000"})
	}
	if q.PageSize < 1 || q.PageSize > maxPageSize {
		fields = append(fields, apperr.Field{Name: "page_size", Message: "must be between 1 and 100"})
	}
	if q.Status != nil && !q.Status.Valid() {
		fields = append(fields, apperr.Field{Name: "status", Message: "must be one of: draft, active, archived"})
	}
	q.Search = strings.TrimSpace(q.Search)
	if len(q.Search) > maxSearchChars {
		fields = append(fields, apperr.Field{Name: "q", Message: "must be at most 100 characters"})
	}
	if len(fields) > 0 {
		return ListResult{}, apperr.Validation("validation failed", fields...)
	}

	items, total, err := s.repo.List(ctx, ListFilter{
		Status: q.Status,
		Search: q.Search,
		Limit:  q.PageSize,
		Offset: (q.Page - 1) * q.PageSize,
	})
	if err != nil {
		return ListResult{}, err
	}
	return ListResult{Items: items, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}

// Update replaces the editable fields of a product.
func (s *Service) Update(ctx context.Context, actor auth.Principal, id uuid.UUID, in UpdateInput) (Product, error) {
	if !actor.Role.CanWrite() {
		return Product{}, errForbidden
	}

	in.Name = strings.TrimSpace(in.Name)
	if err := validation.Struct(in); err != nil {
		return Product{}, err
	}
	if err := validateMoney(in.MinAmount, in.MaxAmount, in.AnnualRate); err != nil {
		return Product{}, err
	}

	var updated Product
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		updated, err = s.repo.Update(ctx, Product{
			ID:          id,
			Name:        in.Name,
			Description: in.Description,
			MinAmount:   in.MinAmount,
			MaxAmount:   in.MaxAmount,
			AnnualRate:  in.AnnualRate,
			Status:      in.Status,
		})
		if err != nil {
			return err
		}
		return s.record(ctx, actor, ActionUpdate, id, map[string]any{"status": string(updated.Status)})
	})
	if err != nil {
		return Product{}, err
	}
	return updated, nil
}

// Delete soft-deletes a product.
func (s *Service) Delete(ctx context.Context, actor auth.Principal, id uuid.UUID) error {
	if !actor.Role.CanWrite() {
		return errForbidden
	}
	return s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.repo.Delete(ctx, id); err != nil {
			return err
		}
		return s.record(ctx, actor, ActionDelete, id, nil)
	})
}

// Export builds an Excel file of all products. The audit entry is mandatory:
// if it cannot be written the export is refused.
func (s *Service) Export(ctx context.Context, actor auth.Principal) ([]byte, error) {
	if !actor.Role.CanExport() {
		return nil, errForbidden
	}
	items, err := s.repo.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	data, err := buildWorkbook(items)
	if err != nil {
		return nil, err
	}
	if err := s.audit.Record(ctx, newEntry(actor, ActionExport, nil, map[string]any{"rows": len(items)})); err != nil {
		return nil, err
	}
	return data, nil
}

// record writes the audit entry and enqueues the background job. It must be
// called inside WithinTx so both commit with the data change.
func (s *Service) record(ctx context.Context, actor auth.Principal, action string, id uuid.UUID, meta map[string]any) error {
	if err := s.audit.Record(ctx, newEntry(actor, action, &id, meta)); err != nil {
		return err
	}
	return s.events.ProductChanged(ctx, id, action)
}

// newEntry builds an audit entry for the given actor and action.
func newEntry(actor auth.Principal, action string, id *uuid.UUID, meta map[string]any) audit.Entry {
	actorID := actor.UserID
	return audit.Entry{
		ActorID:    &actorID,
		Action:     action,
		EntityType: entityType,
		EntityID:   id,
		Metadata:   meta,
	}
}

// validateMoney enforces the financial rules of a product. Values that are not
// plain numbers (exponent notation, more than 2 decimals) are rejected before
// any comparison, so malformed decimals never reach the arithmetic.
func validateMoney(minAmount, maxAmount, rate decimal.Decimal) error {
	var fields []apperr.Field
	add := func(name, msg string) { fields = append(fields, apperr.Field{Name: name, Message: msg}) }

	plain := func(name string, v decimal.Decimal) bool {
		if v.Exponent() < -2 || v.Exponent() > 0 {
			add(name, "must be a plain number with at most 2 decimal places")
			return false
		}
		return true
	}

	minOK := plain("min_amount", minAmount)
	maxOK := plain("max_amount", maxAmount)
	rateOK := plain("annual_rate", rate)

	if minOK {
		switch {
		case !minAmount.IsPositive():
			add("min_amount", "must be greater than 0")
		case minAmount.GreaterThan(maxMoney):
			add("min_amount", "is too large")
		}
	}
	if maxOK && maxAmount.GreaterThan(maxMoney) {
		add("max_amount", "is too large")
	}
	if minOK && maxOK && maxAmount.LessThan(minAmount) {
		add("max_amount", "must be greater than or equal to min_amount")
	}
	if rateOK && (rate.IsNegative() || rate.GreaterThan(hundred)) {
		add("annual_rate", "must be between 0 and 100")
	}

	if len(fields) == 0 {
		return nil
	}
	return apperr.Validation("validation failed", fields...)
}
