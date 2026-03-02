// Package tenant provides helpers for extracting and propagating tenant
// context throughout an API request lifecycle.
package tenant

import (
	"context"
	"errors"
)

type contextKey string

const tenantIDKey contextKey = "tenant_id"

// ErrNoTenant is returned when a tenant ID cannot be found in a context.
var ErrNoTenant = errors.New("tenant: no tenant_id in context")

// NewContext returns a new context carrying the given tenantID.
func NewContext(ctx context.Context, tenantID int) context.Context {
	return context.WithValue(ctx, tenantIDKey, tenantID)
}

// FromContext extracts the tenant ID from ctx.
// Returns ErrNoTenant if the value is absent or not an int.
func FromContext(ctx context.Context) (int, error) {
	v := ctx.Value(tenantIDKey)
	if v == nil {
		return 0, ErrNoTenant
	}
	id, ok := v.(int)
	if !ok {
		return 0, ErrNoTenant
	}
	return id, nil
}
