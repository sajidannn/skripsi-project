package multidb

import (
	"context"
	"fmt"

	"github.com/sajidannn/pos-api/internal/db/multidb"
	"github.com/sajidannn/pos-api/internal/model"
	"github.com/sajidannn/pos-api/internal/repository"
)

type tenantRepo struct {
	mgr *multidb.Manager
}

// NewTenantRepo returns a new multidb TenantRepository.
func NewTenantRepo(mgr *multidb.Manager) repository.TenantRepository {
	return &tenantRepo{mgr: mgr}
}

func (r *tenantRepo) List(ctx context.Context) ([]model.Tenant, error) {
	// In multi-db mode, list of tenants is stored in the master database.
	masterPool := r.mgr.Master()

	rows, err := masterPool.Query(ctx, "SELECT id, name, db_name, created_at FROM tenants ORDER BY id ASC")
	if err != nil {
		return nil, fmt.Errorf("multidb.tenantRepo.List: %w", err)
	}
	defer rows.Close()

	var tenants []model.Tenant
	for rows.Next() {
		var t model.Tenant
		if err := rows.Scan(&t.ID, &t.Name, &t.DBName, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("multidb.tenantRepo.List scan: %w", err)
		}
		tenants = append(tenants, t)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	if tenants == nil {
		tenants = []model.Tenant{}
	}

	return tenants, nil
}
