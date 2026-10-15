// File: internal/platform/reqctx/reqctx.go
// Usage: Carries request ID and client IP through the context.

// Package reqctx carries per-request metadata (request ID, client IP) through
// context so services and the audit trail can use it without importing HTTP code.
package reqctx

import "context"

// Info is the per-request metadata: request ID and client IP.
type Info struct {
	RequestID string
	ClientIP  string
}

// key is the private context key under which Info is stored.
type key struct{}

// With returns a copy of ctx that carries the request info.
func With(ctx context.Context, info Info) context.Context {
	return context.WithValue(ctx, key{}, info)
}

// From returns the request info, or a zero value when none is set (jobs, tests).
func From(ctx context.Context) Info {
	info, _ := ctx.Value(key{}).(Info)
	return info
}
