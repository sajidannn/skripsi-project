package singledb

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sajidannn/pos-api/internal/model"
	"github.com/sajidannn/pos-api/internal/repository"
)

type tenantRepo struct {
	db *pgxpool.Pool
}

// NewTenantRepo returns a new singledb TenantRepository.
func NewTenantRepo(db *pgxpool.Pool) repository.TenantRepository {
	return &tenantRepo{db: db}
}

func (r *tenantRepo) List(ctx context.Context) ([]model.Tenant, error) {
	rows, err := r.db.Query(ctx, "SELECT id, name, created_at FROM tenants ORDER BY id ASC")
	if err != nil {
		return nil, fmt.Errorf("singledb.tenantRepo.List: %w", err)
	}
	defer rows.Close()

	var tenants []model.Tenant
	for rows.Next() {
		var t model.Tenant
		if err := rows.Scan(&t.ID, &t.Name, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("singledb.tenantRepo.List scan: %w", err)
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
