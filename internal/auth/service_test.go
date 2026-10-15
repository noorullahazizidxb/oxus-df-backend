// File: internal/auth/service_test.go
// Usage: Tests for the login service using in-memory fakes (no database needed).

package auth

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"oxus-df-backend/internal/apperr"
	"oxus-df-backend/internal/audit"
)

// fakeRepo is an in-memory user store for tests.
type fakeRepo struct {
	users map[string]User
}

// GetByEmail is the fake lookup used in tests.
func (f *fakeRepo) GetByEmail(_ context.Context, email string) (User, error) {
	u, ok := f.users[email]
	if !ok {
		return User{}, apperr.NotFound("user not found")
	}
	return u, nil
}

// Count is the fake user counter used in tests.
func (f *fakeRepo) Count(context.Context) (int64, error) { return int64(len(f.users)), nil }

// Create is the fake insert used in tests.
func (f *fakeRepo) Create(_ context.Context, u User) (User, error) {
	u.ID = uuid.New()
	f.users[u.Email] = u
	return u, nil
}

// fakeLimiter is an in-memory attempt counter for tests.
type fakeLimiter struct {
	mu     sync.Mutex
	counts map[string]int64
}

// Count is the fake attempt counter used in tests.
func (f *fakeLimiter) Count(_ context.Context, key string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.counts[key], nil
}

// Hit is the fake attempt increment used in tests.
func (f *fakeLimiter) Hit(_ context.Context, key string, _ time.Duration) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.counts[key]++
	return f.counts[key], nil
}

// Reset is the fake attempt reset used in tests.
func (f *fakeLimiter) Reset(_ context.Context, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.counts, key)
	return nil
}

// fakeAuditor collects audit entries in memory for tests.
type fakeAuditor struct{ entries []audit.Entry }

// Record remembers the audit entry so tests can check it.
func (f *fakeAuditor) Record(_ context.Context, e audit.Entry) error {
	f.entries = append(f.entries, e)
	return nil
}

// newTestService builds an auth service with two test users and fake dependencies.
func newTestService(t *testing.T) (*Service, *fakeAuditor) {
	t.Helper()
	hash, err := HashPassword("S3cret-passphrase")
	require.NoError(t, err)

	repo := &fakeRepo{users: map[string]User{
		"admin@example.com": {ID: uuid.New(), Email: "admin@example.com", PasswordHash: hash, Role: RoleAdmin, IsActive: true},
		"off@example.com":   {ID: uuid.New(), Email: "off@example.com", PasswordHash: hash, Role: RoleAdmin, IsActive: false},
	}}
	auditor := &fakeAuditor{}
	svc := NewService(
		repo,
		&fakeLimiter{counts: map[string]int64{}},
		NewTokenManager(testSecret, "test", time.Minute),
		auditor,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		Policy{MaxAttempts: 3, LockWindow: time.Minute},
	)
	return svc, auditor
}

// TestLoginSuccess checks a valid login returns a token and writes an audit entry.
func TestLoginSuccess(t *testing.T) {
	svc, auditor := newTestService(t)

	res, err := svc.Login(context.Background(), LoginInput{Email: " Admin@Example.com ", Password: "S3cret-passphrase"})
	require.NoError(t, err)
	assert.NotEmpty(t, res.AccessToken)
	assert.Equal(t, time.Minute, res.ExpiresIn)

	require.Len(t, auditor.entries, 1)
	assert.Equal(t, ActionLogin, auditor.entries[0].Action)
}

// TestLoginFailures checks wrong password, unknown user and inactive user are all rejected.
func TestLoginFailures(t *testing.T) {
	svc, _ := newTestService(t)

	tests := map[string]LoginInput{
		"wrong password": {Email: "admin@example.com", Password: "nope"},
		"unknown user":   {Email: "ghost@example.com", Password: "S3cret-passphrase"},
		"inactive user":  {Email: "off@example.com", Password: "S3cret-passphrase"},
	}
	for name, in := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := svc.Login(context.Background(), in)
			require.Error(t, err)
			assert.True(t, apperr.Is(err, apperr.KindUnauthorized))
		})
	}
}

// TestLoginValidation checks that a bad email or empty password is rejected.
func TestLoginValidation(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.Login(context.Background(), LoginInput{Email: "not-an-email", Password: ""})
	require.Error(t, err)
	assert.True(t, apperr.Is(err, apperr.KindValidation))
}

// TestLoginLocksAfterMaxAttempts checks the account locks after too many failures.
func TestLoginLocksAfterMaxAttempts(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	for range 3 {
		_, err := svc.Login(ctx, LoginInput{Email: "admin@example.com", Password: "wrong"})
		require.True(t, apperr.Is(err, apperr.KindUnauthorized))
	}

	// Even the correct password is refused while locked.
	_, err := svc.Login(ctx, LoginInput{Email: "admin@example.com", Password: "S3cret-passphrase"})
	require.Error(t, err)
	assert.True(t, apperr.Is(err, apperr.KindTooManyRequests))
}

// TestBootstrapAdmin checks the first admin is created once and only once.
func TestBootstrapAdmin(t *testing.T) {
	svc := NewService(
		&fakeRepo{users: map[string]User{}},
		&fakeLimiter{counts: map[string]int64{}},
		NewTokenManager(testSecret, "test", time.Minute),
		&fakeAuditor{},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		Policy{MaxAttempts: 3, LockWindow: time.Minute},
	)
	ctx := context.Background()

	require.NoError(t, svc.EnsureBootstrapAdmin(ctx, "Root@Example.com", "a-long-enough-password"))
	_, err := svc.Login(ctx, LoginInput{Email: "root@example.com", Password: "a-long-enough-password"})
	require.NoError(t, err)

	// Second call is a no-op because an admin now exists.
	require.NoError(t, svc.EnsureBootstrapAdmin(ctx, "other@example.com", "another-long-password"))
	repo, ok := svc.repo.(*fakeRepo)
	require.True(t, ok)
	assert.Len(t, repo.users, 1)
}
