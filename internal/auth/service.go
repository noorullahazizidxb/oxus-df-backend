// File: internal/auth/service.go
// Usage: Login rules: check password, lock account after too many failures, issue token, write audit entries.

package auth

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"oxus-df-backend/internal/apperr"
	"oxus-df-backend/internal/audit"
	"oxus-df-backend/internal/platform/validation"
)

const (
	ActionLogin       = "auth.login"
	ActionLoginFailed = "auth.login_failed"
	entityType        = "admin_user"
)

// Repository is the admin-user storage the login service needs (faked in tests).
type Repository interface {
	GetByEmail(ctx context.Context, email string) (User, error)
	Count(ctx context.Context) (int64, error)
	Create(ctx context.Context, u User) (User, error)
}

// AttemptLimiter counts failed logins per account (backed by Valkey).
type AttemptLimiter interface {
	Count(ctx context.Context, key string) (int64, error)
	Hit(ctx context.Context, key string, window time.Duration) (int64, error)
	Reset(ctx context.Context, key string) error
}

// Auditor writes audit entries (faked in tests).
type Auditor interface {
	Record(ctx context.Context, e audit.Entry) error
}

// Policy configures the login lockout.
type Policy struct {
	MaxAttempts int64
	LockWindow  time.Duration
}

// Service holds the login rules: password check, lockout, token and audit.
type Service struct {
	repo    Repository
	limiter AttemptLimiter
	tokens  *TokenManager
	audit   Auditor
	log     *slog.Logger
	policy  Policy
}

// NewService creates the auth service with all its dependencies.
func NewService(repo Repository, limiter AttemptLimiter, tokens *TokenManager, auditor Auditor, log *slog.Logger, policy Policy) *Service {
	return &Service{repo: repo, limiter: limiter, tokens: tokens, audit: auditor, log: log, policy: policy}
}

// Login verifies credentials and returns an access token. After
// Policy.MaxAttempts failures the account is locked for Policy.LockWindow.
func (s *Service) Login(ctx context.Context, in LoginInput) (LoginResult, error) {
	in.Email = normalizeEmail(in.Email)
	if err := validation.Struct(in); err != nil {
		return LoginResult{}, err
	}
	email := in.Email
	key := "login:" + email

	attempts, err := s.limiter.Count(ctx, key)
	if err != nil {
		return LoginResult{}, fmt.Errorf("read login attempts: %w", err) // fail closed
	}
	if attempts >= s.policy.MaxAttempts {
		return LoginResult{}, apperr.TooManyRequests("too many failed login attempts, try again later")
	}

	user, err := s.authenticate(ctx, email, in.Password)
	if err != nil {
		if apperr.Is(err, apperr.KindUnauthorized) {
			s.registerFailure(ctx, key, email)
		}
		return LoginResult{}, err
	}

	if err := s.limiter.Reset(ctx, key); err != nil {
		s.log.ErrorContext(ctx, "reset login attempts", "error", err)
	}
	token, ttl, err := s.tokens.Issue(user)
	if err != nil {
		return LoginResult{}, fmt.Errorf("issue token: %w", err)
	}
	s.recordAudit(ctx, &user.ID, ActionLogin, nil)
	return LoginResult{AccessToken: token, ExpiresIn: ttl}, nil
}

// authenticate checks email and password. It always runs a hash check so timing does not reveal whether the email exists.
func (s *Service) authenticate(ctx context.Context, email, password string) (User, error) {
	user, err := s.repo.GetByEmail(ctx, email)
	found := err == nil
	if err != nil && !apperr.Is(err, apperr.KindNotFound) {
		return User{}, err
	}

	hash := dummyHash() // keep timing equal for unknown accounts
	if found {
		hash = user.PasswordHash
	}
	ok, err := VerifyPassword(password, hash)
	if err != nil {
		return User{}, fmt.Errorf("verify password: %w", err)
	}
	if !found || !ok || !user.IsActive {
		return User{}, apperr.Unauthorized("invalid email or password")
	}
	return user, nil
}

// registerFailure counts a failed login for the lockout and writes an audit entry.
func (s *Service) registerFailure(ctx context.Context, key, email string) {
	if _, err := s.limiter.Hit(ctx, key, s.policy.LockWindow); err != nil {
		s.log.ErrorContext(ctx, "record failed login", "error", err)
	}
	s.recordAudit(ctx, nil, ActionLoginFailed, map[string]any{"email": email})
}

// recordAudit writes an audit entry; a failure is only logged so login is not blocked.
func (s *Service) recordAudit(ctx context.Context, actor *uuid.UUID, action string, meta map[string]any) {
	err := s.audit.Record(ctx, audit.Entry{
		ActorID:    actor,
		Action:     action,
		EntityType: entityType,
		EntityID:   actor,
		Metadata:   meta,
	})
	if err != nil {
		s.log.ErrorContext(ctx, "write audit entry", "action", action, "error", err)
	}
}

// EnsureBootstrapAdmin creates the first super admin when none exists, so a
// fresh environment can log in. It does nothing once any admin exists.
func (s *Service) EnsureBootstrapAdmin(ctx context.Context, email, password string) error {
	if email == "" || password == "" {
		return nil
	}
	n, err := s.repo.Count(ctx)
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}

	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	_, err = s.repo.Create(ctx, User{
		Email:        normalizeEmail(email),
		PasswordHash: hash,
		FullName:     "Bootstrap Admin",
		Role:         RoleSuperAdmin,
		IsActive:     true,
	})
	switch {
	case err == nil:
		s.log.InfoContext(ctx, "bootstrap admin created; remove OXUS_BOOTSTRAP_ADMIN_* from the environment")
		return nil
	case apperr.Is(err, apperr.KindConflict):
		return nil // another instance won the race
	default:
		return err
	}
}

// normalizeEmail trims spaces and lowercases the email.
func normalizeEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }
