// File: internal/products/service_test.go
// Usage: Unit tests for the products service using in-memory fakes.

package products

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"oxus-df-backend/internal/apperr"
	"oxus-df-backend/internal/audit"
	"oxus-df-backend/internal/auth"
)

// fakeRepo is an in-memory product store for tests.
type fakeRepo struct {
	created []Product
}

// Create is the fake insert used in tests.
func (f *fakeRepo) Create(_ context.Context, p Product) (Product, error) {
	p.ID = uuid.New()
	p.CreatedAt = time.Now()
	p.UpdatedAt = p.CreatedAt
	f.created = append(f.created, p)
	return p, nil
}

// Get is the fake lookup used in tests (always not found).
func (f *fakeRepo) Get(context.Context, uuid.UUID) (Product, error) {
	return Product{}, apperr.NotFound("product not found")
}

// List is the fake list used in tests.
func (f *fakeRepo) List(_ context.Context, _ ListFilter) ([]Product, int64, error) {
	return f.created, int64(len(f.created)), nil
}

// ListAll is the fake export list used in tests.
func (f *fakeRepo) ListAll(context.Context) ([]Product, error) { return f.created, nil }

// Update is the fake update used in tests.
func (f *fakeRepo) Update(_ context.Context, p Product) (Product, error) { return p, nil }

// Delete is the fake delete used in tests.
func (f *fakeRepo) Delete(context.Context, uuid.UUID) error { return nil }

// fakeTx is a transaction helper that just runs the function.
type fakeTx struct{}

// WithinTx just runs the function; tests do not need a real transaction.
func (fakeTx) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error { return fn(ctx) }

// fakeAuditor collects audit entries in memory for tests.
type fakeAuditor struct{ entries []audit.Entry }

// Record remembers the audit entry so tests can check it.
func (f *fakeAuditor) Record(_ context.Context, e audit.Entry) error {
	f.entries = append(f.entries, e)
	return nil
}

// fakeEvents collects job actions in memory for tests.
type fakeEvents struct{ actions []string }

// ProductChanged remembers the action so tests can check a job would be queued.
func (f *fakeEvents) ProductChanged(_ context.Context, _ uuid.UUID, action string) error {
	f.actions = append(f.actions, action)
	return nil
}

// newTestService builds a products service wired to fakes and returns the fakes for assertions.
func newTestService() (*Service, *fakeRepo, *fakeAuditor, *fakeEvents) {
	repo, aud, ev := &fakeRepo{}, &fakeAuditor{}, &fakeEvents{}
	return NewService(repo, fakeTx{}, aud, ev), repo, aud, ev
}

var (
	admin  = auth.Principal{UserID: uuid.New(), Role: auth.RoleAdmin}
	viewer = auth.Principal{UserID: uuid.New(), Role: auth.RoleViewer}
)

// validCreate returns a valid create input that tests can modify.
func validCreate() CreateInput {
	return CreateInput{
		Code:       " murabaha1 ",
		Name:       "Murabaha financing",
		Currency:   "afn",
		MinAmount:  decimal.RequireFromString("1000"),
		MaxAmount:  decimal.RequireFromString("50000.50"),
		AnnualRate: decimal.RequireFromString("12.5"),
	}
}

// TestCreate_NormalizesAndRecordsAuditAndEvent checks code/currency are normalized and audit + job are recorded.
func TestCreate_NormalizesAndRecordsAuditAndEvent(t *testing.T) {
	svc, repo, aud, ev := newTestService()

	p, err := svc.Create(context.Background(), admin, validCreate())
	require.NoError(t, err)

	assert.Equal(t, "MURABAHA1", p.Code)
	assert.Equal(t, "AFN", p.Currency)
	assert.Equal(t, StatusDraft, p.Status)
	require.Len(t, repo.created, 1)

	require.Len(t, aud.entries, 1)
	assert.Equal(t, ActionCreate, aud.entries[0].Action)
	assert.Equal(t, admin.UserID, *aud.entries[0].ActorID)
	assert.Equal(t, []string{ActionCreate}, ev.actions)
}

// TestWritesRequireWriteRole checks a viewer cannot create, update, delete or export.
func TestWritesRequireWriteRole(t *testing.T) {
	svc, repo, aud, _ := newTestService()
	ctx := context.Background()

	_, err := svc.Create(ctx, viewer, validCreate())
	assert.True(t, apperr.Is(err, apperr.KindForbidden))

	_, err = svc.Update(ctx, viewer, uuid.New(), UpdateInput{})
	assert.True(t, apperr.Is(err, apperr.KindForbidden))

	assert.True(t, apperr.Is(svc.Delete(ctx, viewer, uuid.New()), apperr.KindForbidden))

	_, err = svc.Export(ctx, viewer)
	assert.True(t, apperr.Is(err, apperr.KindForbidden))

	assert.Empty(t, repo.created)
	assert.Empty(t, aud.entries)
}

// TestCreate_Validation checks each invalid input is rejected and nothing is stored or audited.
func TestCreate_Validation(t *testing.T) {
	tests := map[string]func(in *CreateInput){
		"short code":        func(in *CreateInput) { in.Code = "A" },
		"code with symbols": func(in *CreateInput) { in.Code = "BAD-CODE!" },
		"bad currency":      func(in *CreateInput) { in.Currency = "XX" },
		"empty name":        func(in *CreateInput) { in.Name = " " },
		"unknown status":    func(in *CreateInput) { in.Status = "weird" },
		"zero min":          func(in *CreateInput) { in.MinAmount = decimal.Zero },
		"max below min":     func(in *CreateInput) { in.MaxAmount = decimal.RequireFromString("10") },
		"three decimals":    func(in *CreateInput) { in.MinAmount = decimal.RequireFromString("1.234") },
		"rate above 100":    func(in *CreateInput) { in.AnnualRate = decimal.RequireFromString("100.01") },
		"negative rate":     func(in *CreateInput) { in.AnnualRate = decimal.RequireFromString("-1") },
		"exponent notation": func(in *CreateInput) { in.MaxAmount = decimal.RequireFromString("1e9") },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			svc, repo, aud, _ := newTestService()
			in := validCreate()
			mutate(&in)

			_, err := svc.Create(context.Background(), admin, in)

			require.Error(t, err)
			assert.True(t, apperr.Is(err, apperr.KindValidation), err.Error())
			assert.Empty(t, repo.created, "nothing must be stored")
			assert.Empty(t, aud.entries, "nothing must be audited")
		})
	}
}

// TestList_Pagination checks page, page size and status limits.
func TestList_Pagination(t *testing.T) {
	svc, _, _, _ := newTestService()
	ctx := context.Background()

	res, err := svc.List(ctx, ListQuery{Page: 1, PageSize: 20})
	require.NoError(t, err)
	assert.Equal(t, int32(1), res.Page)

	for _, q := range []ListQuery{
		{Page: 0, PageSize: 20},
		{Page: 1, PageSize: 0},
		{Page: 1, PageSize: 101},
	} {
		_, err := svc.List(ctx, q)
		assert.True(t, apperr.Is(err, apperr.KindValidation), "%+v", q)
	}

	bad := Status("nope")
	_, err = svc.List(ctx, ListQuery{Page: 1, PageSize: 20, Status: &bad})
	assert.True(t, apperr.Is(err, apperr.KindValidation))
}

// TestExport_BuildsWorkbookAndAudits checks the export returns an xlsx file and is audited.
func TestExport_BuildsWorkbookAndAudits(t *testing.T) {
	svc, _, aud, _ := newTestService()
	ctx := context.Background()
	_, err := svc.Create(ctx, admin, validCreate())
	require.NoError(t, err)

	data, err := svc.Export(ctx, admin)
	require.NoError(t, err)
	assert.Equal(t, []byte("PK"), data[:2], "xlsx is a zip archive")

	last := aud.entries[len(aud.entries)-1]
	assert.Equal(t, ActionExport, last.Action)
	assert.Equal(t, 1, last.Metadata["rows"])
}
