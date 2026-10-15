// File: internal/auth/repository.go
// Usage: Reads and writes admin users in PostgreSQL.

package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"oxus-df-backend/internal/apperr"
	"oxus-df-backend/internal/platform/database"
	"oxus-df-backend/internal/platform/database/sqlc"
)

// PgRepository stores admin users in PostgreSQL.
type PgRepository struct {
	db *database.DB
}

// NewPgRepository creates the PostgreSQL admin-user repository.
func NewPgRepository(db *database.DB) *PgRepository { return &PgRepository{db: db} }

// GetByEmail finds an admin user by email, or returns a not-found error.
func (r *PgRepository) GetByEmail(ctx context.Context, email string) (User, error) {
	row, err := r.db.Queries(ctx).GetAdminUserByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, apperr.NotFound("user not found")
	}
	if err != nil {
		return User{}, fmt.Errorf("get admin user: %w", err)
	}
	return toUser(row), nil
}

// Count returns how many admin users exist.
func (r *PgRepository) Count(ctx context.Context) (int64, error) {
	n, err := r.db.Queries(ctx).CountAdminUsers(ctx)
	if err != nil {
		return 0, fmt.Errorf("count admin users: %w", err)
	}
	return n, nil
}

// Create inserts a new admin user; a duplicate email returns a conflict error.
func (r *PgRepository) Create(ctx context.Context, u User) (User, error) {
	row, err := r.db.Queries(ctx).CreateAdminUser(ctx, sqlc.CreateAdminUserParams{
		Email:        u.Email,
		PasswordHash: u.PasswordHash,
		FullName:     u.FullName,
		Role:         string(u.Role),
	})
	if database.IsUniqueViolation(err) {
		return User{}, apperr.Conflict("user already exists")
	}
	if err != nil {
		return User{}, fmt.Errorf("create admin user: %w", err)
	}
	return toUser(row), nil
}

// toUser converts a database row into the auth User type.
func toUser(row sqlc.AdminUser) User {
	return User{
		ID:           row.ID,
		Email:        row.Email,
		PasswordHash: row.PasswordHash,
		FullName:     row.FullName,
		Role:         Role(row.Role),
		IsActive:     row.IsActive,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}
}
