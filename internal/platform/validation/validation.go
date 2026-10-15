// File: internal/platform/validation/validation.go
// Usage: Input validation (validator library) that returns friendly field errors.

// Package validation wraps go-playground/validator and reports failures as
// apperr validation errors that use JSON field names.
package validation

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"

	"oxus-df-backend/internal/apperr"
)

var validate = newValidator()

// newValidator creates the validator and makes it report JSON field names.
func newValidator() *validator.Validate {
	v := validator.New()
	v.RegisterTagNameFunc(func(f reflect.StructField) string {
		name := strings.SplitN(f.Tag.Get("json"), ",", 2)[0]
		if name == "" || name == "-" {
			return f.Name
		}
		return name
	})
	return v
}

// Struct validates s using its `validate` tags.
func Struct(s any) error {
	err := validate.Struct(s)
	if err == nil {
		return nil
	}
	var verrs validator.ValidationErrors
	if !errors.As(err, &verrs) {
		return err
	}
	fields := make([]apperr.Field, 0, len(verrs))
	for _, fe := range verrs {
		fields = append(fields, apperr.Field{Name: fe.Field(), Message: message(fe)})
	}
	return apperr.Validation("validation failed", fields...)
}

// message turns a validator rule (required, min, ...) into a readable sentence.
func message(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "is required"
	case "min":
		return fmt.Sprintf("must be at least %s", fe.Param())
	case "max":
		return fmt.Sprintf("must be at most %s", fe.Param())
	case "email":
		return "must be a valid email address"
	case "oneof":
		return fmt.Sprintf("must be one of: %s", strings.ReplaceAll(fe.Param(), " ", ", "))
	case "iso4217":
		return "must be a valid ISO 4217 currency code"
	case "alphanum":
		return "must contain only letters and digits"
	default:
		return "is invalid"
	}
}
