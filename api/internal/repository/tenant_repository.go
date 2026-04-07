package repository

import (
	"context"

	"github.com/sajidannn/pos-api/internal/model"
)

// TenantRepository is the data-access contract for tenants.
type TenantRepository interface {
	// List returns all registered tenants.
	List(ctx context.Context) ([]model.Tenant, error)
}
