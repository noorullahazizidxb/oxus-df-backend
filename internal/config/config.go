// File: internal/config/config.go
// Usage: Loads and validates settings from defaults and OXUS_* environment variables.

// Package config loads settings with koanf: built-in defaults first, then
// OXUS_* environment variables (OXUS_DATABASE_URL -> database_url).
// Vault for staging/production is planned but not added yet (see README's
// "Libraries used" section).
package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/v2"
)

const (
	envPrefix        = "OXUS_"
	minJWTSecretLen  = 32
	devSecretPrefix  = "dev-only"
	envProduction    = "production"
	minBootstrapPass = 12
)

// Config holds every setting of the application, filled from OXUS_* variables.
type Config struct {
	Env                string   `koanf:"env"`
	LogLevel           string   `koanf:"log_level"`
	HTTPAddr           string   `koanf:"http_addr"`
	HTTPTrustedProxies []string `koanf:"http_trusted_proxies"`

	DatabaseURL      string `koanf:"database_url"`
	DatabaseMaxConns int32  `koanf:"database_max_conns"`
	ValkeyURL        string `koanf:"valkey_url"`

	JWTSecret string        `koanf:"jwt_secret"`
	JWTIssuer string        `koanf:"jwt_issuer"`
	JWTTTL    time.Duration `koanf:"jwt_ttl"`

	RateLimitRequests int64         `koanf:"rate_limit_requests"`
	RateLimitWindow   time.Duration `koanf:"rate_limit_window"`
	LoginMaxAttempts  int64         `koanf:"login_max_attempts"`
	LoginLockWindow   time.Duration `koanf:"login_lock_window"`

	BootstrapAdminEmail    string `koanf:"bootstrap_admin_email"`
	BootstrapAdminPassword string `koanf:"bootstrap_admin_password"`

	AppMinVersion string `koanf:"app_min_version"`
}

var defaults = map[string]any{
	"env":                  "development",
	"log_level":            "info",
	"http_addr":            ":8080",
	"database_max_conns":   10,
	"jwt_issuer":           "oxus-df",
	"jwt_ttl":              "15m",
	"rate_limit_requests":  300,
	"rate_limit_window":    "1m",
	"login_max_attempts":   5,
	"login_lock_window":    "15m",
	"app_min_version":      "1.0.0",
	"http_trusted_proxies": "",
}

// Load reads and validates the configuration.
func Load() (*Config, error) {
	k := koanf.New(".")
	if err := k.Load(confmap.Provider(defaults, "."), nil); err != nil {
		return nil, fmt.Errorf("load defaults: %w", err)
	}
	toKey := func(s string) string {
		return strings.ToLower(strings.TrimPrefix(s, envPrefix))
	}
	if err := k.Load(env.Provider(envPrefix, ".", toKey), nil); err != nil {
		return nil, fmt.Errorf("load environment: %w", err)
	}

	var cfg Config
	if err := k.Unmarshal("", &cfg); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// IsProduction reports whether the service runs in production mode.
func (c *Config) IsProduction() bool { return c.Env == envProduction }

// validate checks required settings and returns all problems at once.
func (c *Config) validate() error {
	var errs []error
	if c.DatabaseURL == "" {
		errs = append(errs, errors.New("OXUS_DATABASE_URL is required"))
	}
	if c.ValkeyURL == "" {
		errs = append(errs, errors.New("OXUS_VALKEY_URL is required"))
	}
	if len(c.JWTSecret) < minJWTSecretLen {
		errs = append(errs, fmt.Errorf("OXUS_JWT_SECRET must be at least %d characters", minJWTSecretLen))
	}
	if c.IsProduction() && strings.HasPrefix(c.JWTSecret, devSecretPrefix) {
		errs = append(errs, errors.New("OXUS_JWT_SECRET must not be the development placeholder in production"))
	}
	if c.JWTTTL <= 0 {
		errs = append(errs, errors.New("OXUS_JWT_TTL must be positive"))
	}
	if c.RateLimitRequests <= 0 || c.RateLimitWindow <= 0 {
		errs = append(errs, errors.New("OXUS_RATE_LIMIT_REQUESTS and OXUS_RATE_LIMIT_WINDOW must be positive"))
	}
	if c.LoginMaxAttempts <= 0 || c.LoginLockWindow <= 0 {
		errs = append(errs, errors.New("OXUS_LOGIN_MAX_ATTEMPTS and OXUS_LOGIN_LOCK_WINDOW must be positive"))
	}
	if c.BootstrapAdminEmail != "" && len(c.BootstrapAdminPassword) < minBootstrapPass {
		errs = append(errs, fmt.Errorf("OXUS_BOOTSTRAP_ADMIN_PASSWORD must be at least %d characters", minBootstrapPass))
	}
	return errors.Join(errs...)
}
