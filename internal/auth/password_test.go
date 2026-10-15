// File: internal/auth/password_test.go
// Usage: Tests for password hashing and verification.

package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPasswordHashAndVerify checks that the right password passes and a wrong one fails.
func TestPasswordHashAndVerify(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	require.NoError(t, err)
	assert.Contains(t, hash, "$argon2id$")

	ok, err := VerifyPassword("correct horse battery staple", hash)
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = VerifyPassword("wrong password", hash)
	require.NoError(t, err)
	assert.False(t, ok)
}

// TestHashesAreSalted checks that hashing the same password twice gives different hashes.
func TestHashesAreSalted(t *testing.T) {
	a, err := HashPassword("same")
	require.NoError(t, err)
	b, err := HashPassword("same")
	require.NoError(t, err)
	assert.NotEqual(t, a, b)
}

// TestVerifyPasswordRejectsMalformedHash checks that broken hash strings return an error.
func TestVerifyPasswordRejectsMalformedHash(t *testing.T) {
	for _, bad := range []string{"", "plaintext", "$bcrypt$v=1$x$y$z", "$argon2id$v=19$m=1,t=1,p=1$!!!$!!!"} {
		ok, err := VerifyPassword("x", bad)
		assert.False(t, ok, bad)
		assert.Error(t, err, bad)
	}
}
