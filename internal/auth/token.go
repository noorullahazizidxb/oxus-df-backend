// File: internal/auth/token.go
// Usage: Creates and verifies short-lived JWT access tokens.

package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"oxus-df-backend/internal/apperr"
)

// claims are the fields stored inside the JWT: our role plus the standard claims.
type claims struct {
	Role Role `json:"role"`
	jwt.RegisteredClaims
}

// TokenManager issues and verifies short-lived HS256 access tokens.
type TokenManager struct {
	secret []byte
	issuer string
	ttl    time.Duration
}

// NewTokenManager creates a token manager with the signing secret, issuer name and token lifetime.
func NewTokenManager(secret, issuer string, ttl time.Duration) *TokenManager {
	return &TokenManager{secret: []byte(secret), issuer: issuer, ttl: ttl}
}

// Issue creates a signed access token for the user.
func (m *TokenManager) Issue(u User) (string, time.Duration, error) {
	now := time.Now()
	c := claims{
		Role: u.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   u.ID.String(),
			ID:        uuid.NewString(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(m.secret)
	if err != nil {
		return "", 0, err
	}
	return signed, m.ttl, nil
}

// Parse verifies signature, issuer and expiry and returns the caller.
func (m *TokenManager) Parse(raw string) (Principal, error) {
	var c claims
	_, err := jwt.ParseWithClaims(raw, &c,
		func(*jwt.Token) (any, error) { return m.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(m.issuer),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return Principal{}, apperr.Unauthorized("invalid or expired token")
	}

	id, err := uuid.Parse(c.Subject)
	if err != nil || !c.Role.Valid() {
		return Principal{}, apperr.Unauthorized("invalid or expired token")
	}
	return Principal{UserID: id, Role: c.Role}, nil
}
