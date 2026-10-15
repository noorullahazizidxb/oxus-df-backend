// File: internal/products/model.go
// Usage: Product data types and input shapes with validation tags.

// Package products is the CRUD feature: financing products managed from the
// admin panel. Money values are shopspring/decimal, never floats.
package products

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Status is the life-cycle state of a product: draft, active or archived.
type Status string

const (
	StatusDraft    Status = "draft"
	StatusActive   Status = "active"
	StatusArchived Status = "archived"
)

// Valid reports whether the status is draft, active or archived.
func (s Status) Valid() bool {
	switch s {
	case StatusDraft, StatusActive, StatusArchived:
		return true
	default:
		return false
	}
}

// Product is a financing product as the business sees it.
type Product struct {
	ID          uuid.UUID
	Code        string
	Name        string
	Description string
	Currency    string
	MinAmount   decimal.Decimal
	MaxAmount   decimal.Decimal
	AnnualRate  decimal.Decimal
	Status      Status
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// CreateInput is validated by the service. Decimal rules (range, scale) are
// checked in validateMoney because validator tags cannot express them.
type CreateInput struct {
	Code        string          `json:"code" validate:"required,alphanum,min=2,max=32"`
	Name        string          `json:"name" validate:"required,min=2,max=120"`
	Description string          `json:"description" validate:"max=2000"`
	Currency    string          `json:"currency" validate:"required,iso4217"`
	MinAmount   decimal.Decimal `json:"min_amount"`
	MaxAmount   decimal.Decimal `json:"max_amount"`
	AnnualRate  decimal.Decimal `json:"annual_rate"`
	Status      Status          `json:"status" validate:"omitempty,oneof=draft active archived"`
}

// UpdateInput replaces the editable fields. Code and currency never change
// because other records will reference them.
type UpdateInput struct {
	Name        string          `json:"name" validate:"required,min=2,max=120"`
	Description string          `json:"description" validate:"max=2000"`
	MinAmount   decimal.Decimal `json:"min_amount"`
	MaxAmount   decimal.Decimal `json:"max_amount"`
	AnnualRate  decimal.Decimal `json:"annual_rate"`
	Status      Status          `json:"status" validate:"required,oneof=draft active archived"`
}

// ListQuery is what the handler passes in; the service validates it.
type ListQuery struct {
	Status   *Status
	Search   string
	Page     int32
	PageSize int32
}

// ListFilter is what the repository receives (already validated, offset-based).
type ListFilter struct {
	Status *Status
	Search string
	Limit  int32
	Offset int32
}

// ListResult is one page of products plus the total count for paging.
type ListResult struct {
	Items    []Product
	Total    int64
	Page     int32
	PageSize int32
}
