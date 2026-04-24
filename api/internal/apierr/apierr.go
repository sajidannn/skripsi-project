// Package apierr defines application-level errors for the POS API.
//
// Error flow:
//  1. Repository / service returns a sentinel or wrapped error
//  2. Handler calls `_ = c.Error(apierr.Wrap(err, "..."))` then returns
//  3. The ErrorHandler middleware (registered last in the chain) renders it
//
// In production (DEBUG=false) 5xx responses never leak internal details.
// In debug mode (DEBUG=true) the `detail` field is populated for 5xx errors.
package apierr

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5"
)

// AppError is the single error type used throughout the API.
type AppError struct {
	HTTPStatus int               // HTTP status code to respond with
	Code       string            // machine-readable code (e.g. "NOT_FOUND")
	Message    string            // safe, user-facing message — always sent to client
	Detail     string            // internal detail — only sent when debug mode is on
	Fields     map[string]string // per-field validation messages (422 only)
	Err        error             // root cause — never sent to client
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error { return e.Err }

// ── Constructors ─────────────────────────────────────────────────────────────

// NotFound returns a 404 AppError.
func NotFound(message string) *AppError {
	return &AppError{
		HTTPStatus: http.StatusNotFound,
		Code:       "NOT_FOUND",
		Message:    message,
	}
}

// BadRequest returns a 400 AppError.
func BadRequest(message string) *AppError {
	return &AppError{
		HTTPStatus: http.StatusBadRequest,
		Code:       "BAD_REQUEST",
		Message:    message,
	}
}

// Unauthorized returns a 401 AppError.
func Unauthorized(message string) *AppError {
	return &AppError{
		HTTPStatus: http.StatusUnauthorized,
		Code:       "UNAUTHORIZED",
		Message:    message,
	}
}

// Forbidden returns a 403 AppError.
func Forbidden(message string) *AppError {
	return &AppError{
		HTTPStatus: http.StatusForbidden,
		Code:       "FORBIDDEN",
		Message:    message,
	}
}

// Conflict returns a 409 AppError.
func Conflict(message string) *AppError {
	return &AppError{
		HTTPStatus: http.StatusConflict,
		Code:       "CONFLICT",
		Message:    message,
	}
}

// InsufficientBalance returns a 402 AppError for failed balance guard checks.
func InsufficientBalance(message string) *AppError {
	return &AppError{
		HTTPStatus: http.StatusPaymentRequired,
		Code:       "INSUFFICIENT_BALANCE",
		Message:    message,
	}
}

// ValidationFailed returns a 422 AppError carrying per-field error messages.
// fields is a map of { "field_name": "human-readable message" }.
func ValidationFailed(fields map[string]string) *AppError {
	return &AppError{
		HTTPStatus: http.StatusUnprocessableEntity,
		Code:       "VALIDATION_ERROR",
		Message:    "validation failed",
		Fields:     fields,
	}
}

// Internal returns a 500 AppError.
// `clientMessage` is what the client sees; `detail` is the internal note
// shown only in debug mode.
func Internal(err error, detail string) *AppError {
	return &AppError{
		HTTPStatus: http.StatusInternalServerError,
		Code:       "INTERNAL_ERROR",
		Message:    "internal server error",
		Detail:     detail,
		Err:        err,
	}
}

// Wrap converts any error into an *AppError intelligently:
//   - pgx.ErrNoRows → 404 NotFound with `notFoundMessage`
//   - *AppError passthrough — returned as-is
//   - anything else → 500 Internal with err.Error() as detail
//
// This is the primary helper to use inside handlers.
func Wrap(err error, notFoundMessage string) *AppError {
	if err == nil {
		return nil
	}

	// Already an AppError — passthrough.
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr
	}

	// DB row not found.
	if errors.Is(err, pgx.ErrNoRows) {
		return NotFound(notFoundMessage)
	}

	// Everything else is treated as an internal server error.
	return Internal(err, err.Error())
}

// Is enables errors.Is() matching against another *AppError by Code.
func (e *AppError) Is(target error) bool {
	var t *AppError
	if errors.As(target, &t) {
		return e.Code == t.Code
	}
	return false
}
