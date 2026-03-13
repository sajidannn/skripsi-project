// Package validator provides helpers for parsing go-playground/validator
// errors (which Gin uses internally for ShouldBindJSON) into human-readable
// field → message maps suitable for API error responses.
package validator

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

// ParseBindingError converts a binding/validation error returned by
// c.ShouldBindJSON into a map of { "field_name": "human message" }.
//
// If the error is not a validator.ValidationErrors (e.g. malformed JSON),
// the map will contain a single "body" key with the raw error message.
func ParseBindingError(err error) map[string]string {
	var ve validator.ValidationErrors
	if !errors.As(err, &ve) {
		// Not a validation error — likely malformed JSON / wrong content-type.
		return map[string]string{"body": err.Error()}
	}

	out := make(map[string]string, len(ve))
	for _, fe := range ve {
		field := toSnakeCase(fe.Field())
		out[field] = fieldMessage(fe)
	}
	return out
}

// fieldMessage returns a human-readable message for a single field error.
func fieldMessage(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", toSnakeCase(fe.Field()))
	case "min":
		if fe.Type().Kind().String() == "string" {
			return fmt.Sprintf("%s must be at least %s characters", toSnakeCase(fe.Field()), fe.Param())
		}
		return fmt.Sprintf("%s must be at least %s", toSnakeCase(fe.Field()), fe.Param())
	case "max":
		if fe.Type().Kind().String() == "string" {
			return fmt.Sprintf("%s must be at most %s characters", toSnakeCase(fe.Field()), fe.Param())
		}
		return fmt.Sprintf("%s must be at most %s", toSnakeCase(fe.Field()), fe.Param())
	case "email":
		return fmt.Sprintf("%s must be a valid email address", toSnakeCase(fe.Field()))
	case "url":
		return fmt.Sprintf("%s must be a valid URL", toSnakeCase(fe.Field()))
	case "numeric":
		return fmt.Sprintf("%s must be a number", toSnakeCase(fe.Field()))
	case "alpha":
		return fmt.Sprintf("%s must contain only letters", toSnakeCase(fe.Field()))
	case "alphanum":
		return fmt.Sprintf("%s must contain only letters and numbers", toSnakeCase(fe.Field()))
	case "oneof":
		return fmt.Sprintf("%s must be one of: %s", toSnakeCase(fe.Field()), strings.ReplaceAll(fe.Param(), " ", ", "))
	case "gt":
		return fmt.Sprintf("%s must be greater than %s", toSnakeCase(fe.Field()), fe.Param())
	case "gte":
		return fmt.Sprintf("%s must be greater than or equal to %s", toSnakeCase(fe.Field()), fe.Param())
	case "lt":
		return fmt.Sprintf("%s must be less than %s", toSnakeCase(fe.Field()), fe.Param())
	case "lte":
		return fmt.Sprintf("%s must be less than or equal to %s", toSnakeCase(fe.Field()), fe.Param())
	default:
		return fmt.Sprintf("%s is invalid (%s)", toSnakeCase(fe.Field()), fe.Tag())
	}
}

// toSnakeCase converts a Go struct field name (PascalCase) to snake_case
// so that it matches the JSON key the client sent.
//
//	"OpeningBalance" → "opening_balance"
//	"Name"           → "name"
func toSnakeCase(s string) string {
	var b strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(r + 32)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
