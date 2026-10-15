//go:build integration

// File: internal/products/repository_integration_test.go
// Usage: Integration tests against a real PostgreSQL container (run with -tags=integration).

package products

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"oxus-df-backend/internal/apperr"
	"oxus-df-backend/internal/platform/database"
)

// newTestDB starts a real PostgreSQL container and applies the SQL migrations.
func newTestDB(t *testing.T) *database.DB {
	t.Helper()
	ctx := context.Background()

	ctr, err := tcpostgres.Run(ctx, "postgres:17-alpine",
		tcpostgres.WithDatabase("oxus"),
		tcpostgres.WithUsername("oxus"),
		tcpostgres.WithPassword("oxus"),
		tcpostgres.BasicWaitStrategies(),
	)
	testcontainers.CleanupContainer(t, ctr)
	require.NoError(t, err)

	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	pool, err := database.NewPool(ctx, dsn, 5)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	files, err := filepath.Glob("../../db/migrations/*.up.sql")
	require.NoError(t, err)
	require.NotEmpty(t, files)
	sort.Strings(files)
	for _, f := range files {
		sqlText, err := os.ReadFile(f)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(sqlText))
		require.NoError(t, err, f)
	}
	return database.New(pool)
}

// sampleProduct builds a valid product with the given code for tests.
func sampleProduct(code string) Product {
	return Product{
		Code:        code,
		Name:        "Sample " + code,
		Description: "integration test",
		Currency:    "AFN",
		MinAmount:   decimal.RequireFromString("1000.00"),
		MaxAmount:   decimal.RequireFromString("5000.50"),
		AnnualRate:  decimal.RequireFromString("12.50"),
		Status:      StatusActive,
	}
}

// TestPgRepository_CRUD checks create, get, update, list, search, soft delete and duplicate codes on real PostgreSQL.
func TestPgRepository_CRUD(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	repo := NewPgRepository(db)

	// Create + Get keep exact decimal values.
	created, err := repo.Create(ctx, sampleProduct("P1"))
	require.NoError(t, err)
	got, err := repo.Get(ctx, created.ID)
	require.NoError(t, err)
	assert.True(t, got.MaxAmount.Equal(decimal.RequireFromString("5000.50")))

	// Duplicate code is a conflict.
	_, err = repo.Create(ctx, sampleProduct("P1"))
	assert.True(t, apperr.Is(err, apperr.KindConflict))

	// Update.
	changed := created
	changed.Name = "Renamed"
	changed.Status = StatusArchived
	updated, err := repo.Update(ctx, changed)
	require.NoError(t, err)
	assert.Equal(t, "Renamed", updated.Name)
	assert.Equal(t, StatusArchived, updated.Status)

	// List with filter and search.
	_, err = repo.Create(ctx, sampleProduct("P2"))
	require.NoError(t, err)
	active := StatusActive
	items, total, err := repo.List(ctx, ListFilter{Status: &active, Search: "p2", Limit: 10})
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	assert.Equal(t, "P2", items[0].Code)

	// Soft delete hides the row and frees the code.
	require.NoError(t, repo.Delete(ctx, created.ID))
	_, err = repo.Get(ctx, created.ID)
	assert.True(t, apperr.Is(err, apperr.KindNotFound))
	assert.True(t, apperr.Is(repo.Delete(ctx, created.ID), apperr.KindNotFound))
	_, err = repo.Create(ctx, sampleProduct("P1"))
	assert.NoError(t, err)

	// Unknown id.
	_, err = repo.Get(ctx, uuid.New())
	assert.True(t, apperr.Is(err, apperr.KindNotFound))
}

// TestWithinTx_RollsBackOnError checks that nothing is saved when the transaction function fails.
func TestWithinTx_RollsBackOnError(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	repo := NewPgRepository(db)

	err := db.WithinTx(ctx, func(ctx context.Context) error {
		if _, err := repo.Create(ctx, sampleProduct("TX1")); err != nil {
			return err
		}
		return assert.AnError
	})
	require.ErrorIs(t, err, assert.AnError)

	items, total, err := repo.List(ctx, ListFilter{Limit: 10})
	require.NoError(t, err)
	assert.Zero(t, total)
	assert.Empty(t, items)
}
