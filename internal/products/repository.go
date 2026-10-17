// File: internal/products/repository.go
// Usage: Reads and writes products in PostgreSQL through the generated sqlc code.

package products

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"oxus-df-backend/internal/apperr"
	"oxus-df-backend/internal/platform/database"
	"oxus-df-backend/internal/platform/database/sqlc"
)

// PgRepository stores products in PostgreSQL.
type PgRepository struct {
	db *database.DB
}

// NewPgRepository creates the PostgreSQL products repository.
func NewPgRepository(db *database.DB) *PgRepository { return &PgRepository{db: db} }

// Create inserts a product; a duplicate code returns a conflict error.
func (r *PgRepository) Create(ctx context.Context, p Product) (Product, error) {
	row, err := r.db.Queries(ctx).CreateProduct(ctx, sqlc.CreateProductParams{
		Code:        p.Code,
		Name:        p.Name,
		Description: p.Description,
		Currency:    p.Currency,
		MinAmount:   p.MinAmount,
		MaxAmount:   p.MaxAmount,
		AnnualRate:  p.AnnualRate,
		Status:      string(p.Status),
	})
	if err != nil {
		return Product{}, mapError(err, "create product")
	}
	return toProduct(row), nil
}

// Get returns one product that is not deleted, or a not-found error.
func (r *PgRepository) Get(ctx context.Context, id uuid.UUID) (Product, error) {
	row, err := r.db.Queries(ctx).GetProduct(ctx, id)
	if err != nil {
		return Product{}, mapError(err, "get product")
	}
	return toProduct(row), nil
}

// List returns one page of products plus the total count for the same filter.
func (r *PgRepository) List(ctx context.Context, f ListFilter) ([]Product, int64, error) {
	q := r.db.Queries(ctx)

	// sqlc uses pointers for nullable SQL parameters. A nil pointer means
	// "do not filter"; a pointer to an empty string would mean a real empty
	// value. This is how Go represents optional values without JS-style
	// undefined/null unions.
	var status, search *string
	if f.Status != nil {
		s := string(*f.Status)
		status = &s
	}
	if f.Search != "" {
		search = &f.Search
	}

	rows, err := q.ListProducts(ctx, sqlc.ListProductsParams{
		Status:     status,
		Search:     search,
		PageLimit:  f.Limit,
		PageOffset: f.Offset,
	})
	if err != nil {
		return nil, 0, mapError(err, "list products")
	}
	total, err := q.CountProducts(ctx, sqlc.CountProductsParams{Status: status, Search: search})
	if err != nil {
		return nil, 0, mapError(err, "count products")
	}
	return toProducts(rows), total, nil
}

// ListAll returns every non-deleted product (used by the export).
func (r *PgRepository) ListAll(ctx context.Context) ([]Product, error) {
	rows, err := r.db.Queries(ctx).ListProductsForExport(ctx)
	if err != nil {
		return nil, mapError(err, "list products for export")
	}
	return toProducts(rows), nil
}

// Update changes the editable fields of a product, or returns not-found.
func (r *PgRepository) Update(ctx context.Context, p Product) (Product, error) {
	row, err := r.db.Queries(ctx).UpdateProduct(ctx, sqlc.UpdateProductParams{
		ID:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		MinAmount:   p.MinAmount,
		MaxAmount:   p.MaxAmount,
		AnnualRate:  p.AnnualRate,
		Status:      string(p.Status),
	})
	if err != nil {
		return Product{}, mapError(err, "update product")
	}
	return toProduct(row), nil
}

// Delete soft-deletes a product (sets deleted_at), or returns not-found.
func (r *PgRepository) Delete(ctx context.Context, id uuid.UUID) error {
	n, err := r.db.Queries(ctx).SoftDeleteProduct(ctx, id)
	if err != nil {
		return mapError(err, "delete product")
	}
	if n == 0 {
		return apperr.NotFound("product not found")
	}
	return nil
}

// mapError turns database errors into application errors.
func mapError(err error, op string) error {
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return apperr.NotFound("product not found")
	case database.IsUniqueViolation(err):
		return apperr.Conflict("a product with this code already exists")
	default:
		return fmt.Errorf("%s: %w", op, err)
	}
}

// toProduct converts a database row into the Product type.
func toProduct(row sqlc.Product) Product {
	return Product{
		ID:          row.ID,
		Code:        row.Code,
		Name:        row.Name,
		Description: row.Description,
		Currency:    row.Currency,
		MinAmount:   row.MinAmount,
		MaxAmount:   row.MaxAmount,
		AnnualRate:  row.AnnualRate,
		Status:      Status(row.Status),
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}

// toProducts converts many database rows into Products.
func toProducts(rows []sqlc.Product) []Product {
	out := make([]Product, 0, len(rows))
	for _, row := range rows {
		out = append(out, toProduct(row))
	}
	return out
}
