// File: internal/platform/httpx/errors.go
// Usage: Small Gin helpers: write error responses, read JSON bodies, default values for optional params.

// Package httpx holds small Gin helpers shared by every module's handler.
package httpx

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"oxus-df-backend/internal/apperr"
)

// fieldPayload is one invalid field in an error response.
type fieldPayload struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// errorPayload is the error code, message and field details sent to the client.
type errorPayload struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details []fieldPayload `json:"details,omitempty"`
}

// errorBody is the JSON envelope: {"error": {...}}.
type errorBody struct {
	Error errorPayload `json:"error"`
}

var statusByKind = map[apperr.Kind]int{
	apperr.KindValidation:      http.StatusBadRequest,
	apperr.KindUnauthorized:    http.StatusUnauthorized,
	apperr.KindForbidden:       http.StatusForbidden,
	apperr.KindNotFound:        http.StatusNotFound,
	apperr.KindConflict:        http.StatusConflict,
	apperr.KindTooManyRequests: http.StatusTooManyRequests,
}

// WriteError renders err as the standard error body and aborts the request.
// Unknown errors become a generic 500 and are attached to the Gin context so
// the logging middleware records the real cause.
func WriteError(c *gin.Context, err error) {
	e, ok := apperr.As(err)
	status, known := http.StatusInternalServerError, false
	if ok {
		status, known = statusByKind[e.Kind]
	}
	if !known {
		_ = c.Error(err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, errorBody{Error: errorPayload{
			Code:    "internal_error",
			Message: "internal server error",
		}})
		return
	}

	payload := errorPayload{Code: e.Code, Message: e.Message}
	for _, f := range e.Fields {
		payload.Details = append(payload.Details, fieldPayload{Field: f.Name, Message: f.Message})
	}
	c.AbortWithStatusJSON(status, errorBody{Error: payload})
}

// BindJSON decodes the request body into dst. On failure it writes a 400 and
// returns false, so handlers can simply `if !httpx.BindJSON(c, &req) { return }`.
func BindJSON(c *gin.Context, dst any) bool {
	if err := c.ShouldBindJSON(dst); err != nil {
		WriteError(c, apperr.Validation("invalid request body"))
		return false
	}
	return true
}

// ParamErrorHandler is used by the generated server when path or query
// parameters cannot be parsed.
func ParamErrorHandler(c *gin.Context, err error, _ int) {
	WriteError(c, apperr.Validation(err.Error()))
}

// ValueOr returns *p, or def when p is nil. Handy for optional query params.
func ValueOr[T any](p *T, def T) T {
	if p == nil {
		return def
	}
	return *p
}
