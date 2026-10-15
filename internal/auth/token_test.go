// File: internal/auth/token_test.go
// Usage: Tests for JWT creation, expiry and tampering.

package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"oxus-df-backend/internal/apperr"
)

var testSecret = strings.Repeat("s", 32)

// TestTokenRoundTrip checks a token we issue can be read back correctly.
func TestTokenRoundTrip(t *testing.T) {
	m := NewTokenManager(testSecret, "test", time.Minute)
	user := User{ID: uuid.New(), Role: RoleAdmin}

	raw, ttl, err := m.Issue(user)
	require.NoError(t, err)
	assert.Equal(t, time.Minute, ttl)

	p, err := m.Parse(raw)
	require.NoError(t, err)
	assert.Equal(t, user.ID, p.UserID)
	assert.Equal(t, RoleAdmin, p.Role)
}

// TestTokenRejected checks expired, tampered, wrong-secret and garbage tokens are rejected.
func TestTokenRejected(t *testing.T) {
	user := User{ID: uuid.New(), Role: RoleViewer}
	valid, _, err := NewTokenManager(testSecret, "test", time.Minute).Issue(user)
	require.NoError(t, err)
	expired, _, err := NewTokenManager(testSecret, "test", -time.Minute).Issue(user)
	require.NoError(t, err)

	tests := map[string]struct {
		manager *TokenManager
		token   string
	}{
		"expired":      {NewTokenManager(testSecret, "test", time.Minute), expired},
		"wrong secret": {NewTokenManager(strings.Repeat("x", 32), "test", time.Minute), valid},
		"wrong issuer": {NewTokenManager(testSecret, "other", time.Minute), valid},
		"tampered":     {NewTokenManager(testSecret, "test", time.Minute), valid[:len(valid)-3] + "abc"},
		"garbage":      {NewTokenManager(testSecret, "test", time.Minute), "not-a-jwt"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := tc.manager.Parse(tc.token)
			require.Error(t, err)
			assert.True(t, apperr.Is(err, apperr.KindUnauthorized))
		})
	}
}
