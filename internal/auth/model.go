// File: internal/auth/model.go
// Usage: Auth data types: roles, user, principal (the logged-in caller) and login input/result.

// Package auth handles admin login (Argon2id + short-lived JWT), login
// lockout, and the Principal that other modules use for permission checks.
package auth

import (
	"time"

	"github.com/google/uuid"
)

// Role is what an admin user is allowed to do: super_admin, admin or viewer.
type Role string

const (
	RoleSuperAdmin Role = "super_admin"
	RoleAdmin      Role = "admin"
	RoleViewer     Role = "viewer"
)

// Valid reports whether the role is one of the known roles.
func (r Role) Valid() bool {
	switch r {
	case RoleSuperAdmin, RoleAdmin, RoleViewer:
		return true
	default:
		return false
	}
}

// CanWrite reports whether the role may create, change or delete data.
func (r Role) CanWrite() bool { return r == RoleSuperAdmin || r == RoleAdmin }

// CanExport reports whether the role may download data exports.
func (r Role) CanExport() bool { return r == RoleSuperAdmin || r == RoleAdmin }

// User is an admin account as stored in the database.
type User struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	FullName     string
	Role         Role
	IsActive     bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Principal is the authenticated caller, as seen by services.
type Principal struct {
	UserID uuid.UUID
	Role   Role
}

// LoginInput is the email and password sent to the login endpoint.
type LoginInput struct {
	Email    string `json:"email" validate:"required,email,max=254"`
	Password string `json:"password" validate:"required,max=128"`
}

// LoginResult is what a successful login returns: the token and how long it lasts.
type LoginResult struct {
	AccessToken string
	ExpiresIn   time.Duration
}
