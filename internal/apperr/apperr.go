// File: internal/apperr/apperr.go
// Usage: One shared error type. Services return these errors; the HTTP layer turns them into status codes (400, 404, ...).

// Package apperr defines the error type shared by every layer. Services return
// *Error values; the HTTP layer maps them to status codes and response bodies.
// Any other error is treated as an internal error and never shown to the client.
package apperr

import "errors"

// Kind classifies an error independently of any transport.
type Kind uint8

const (
	KindValidation Kind = iota + 1
	KindUnauthorized
	KindForbidden
	KindNotFound
	KindConflict
	KindTooManyRequests
)

// Field describes one invalid input field.
type Field struct {
	Name    string
	Message string
}

// Error is a safe-to-expose application error.
type Error struct {
	Kind    Kind
	Code    string
	Message string
	Fields  []Field
}

// Error makes *Error satisfy the standard error interface.
func (e *Error) Error() string { return e.Code + ": " + e.Message }

// Validation builds a 400 error with optional per-field messages.
func Validation(message string, fields ...Field) *Error {
	return &Error{Kind: KindValidation, Code: "validation_failed", Message: message, Fields: fields}
}

// Unauthorized builds a 401 error (not logged in or bad credentials).
func Unauthorized(message string) *Error {
	return &Error{Kind: KindUnauthorized, Code: "unauthorized", Message: message}
}

// Forbidden builds a 403 error (logged in but not allowed).
func Forbidden(message string) *Error {
	return &Error{Kind: KindForbidden, Code: "forbidden", Message: message}
}

// NotFound builds a 404 error.
func NotFound(message string) *Error {
	return &Error{Kind: KindNotFound, Code: "not_found", Message: message}
}

// Conflict builds a 409 error (for example a duplicate code).
func Conflict(message string) *Error {
	return &Error{Kind: KindConflict, Code: "conflict", Message: message}
}

// TooManyRequests builds a 429 error (rate limit or locked account).
func TooManyRequests(message string) *Error {
	return &Error{Kind: KindTooManyRequests, Code: "too_many_requests", Message: message}
}

// As returns the *Error inside err, if any.
func As(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

// Is reports whether err is an *Error of the given kind.
func Is(err error, kind Kind) bool {
	e, ok := As(err)
	return ok && e.Kind == kind
}
