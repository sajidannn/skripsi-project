// Package singledb provides repository implementations for single-DB mode.
// All queries filter by tenant_id, isolating each tenant's data within the
// shared database.
package singledb

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sajidannn/pos-api/internal/dto"
	"github.com/sajidannn/pos-api/internal/model"
)

// WarehouseRepo implements repository.WarehouseRepository for single-DB mode.
type WarehouseRepo struct {
	db *pgxpool.Pool
}

// NewWarehouseRepo creates a new WarehouseRepo backed by the shared pool.
func NewWarehouseRepo(db *pgxpool.Pool) *WarehouseRepo {
	return &WarehouseRepo{db: db}
}

// Create inserts a warehouse row for the given tenant.
func (r *WarehouseRepo) Create(ctx context.Context, tenantID int, req dto.CreateWarehouseRequest) (*model.Warehouse, error) {
	var w model.Warehouse
	err := r.db.QueryRow(ctx,
		`INSERT INTO warehouses (tenant_id, name)
		 VALUES ($1, $2)
		 RETURNING id, tenant_id, name, created_at`,
		tenantID, req.Name,
	).Scan(&w.ID, &w.TenantID, &w.Name, &w.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("singledb.WarehouseRepo.Create: %w", err)
	}
	return &w, nil
}

// GetByID fetches a warehouse by its ID, scoped to tenantID.
func (r *WarehouseRepo) GetByID(ctx context.Context, tenantID, id int) (*model.Warehouse, error) {
	var w model.Warehouse
	err := r.db.QueryRow(ctx,
		`SELECT id, tenant_id, name, created_at
		 FROM warehouses
		 WHERE id = $1 AND tenant_id = $2`,
		id, tenantID,
	).Scan(&w.ID, &w.TenantID, &w.Name, &w.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("singledb.WarehouseRepo.GetByID: %w", err)
	}
	return &w, nil
}

// Update a warehouse.
func (r *WarehouseRepo) Update(ctx context.Context, tenantID, id int, req dto.UpdateWarehouseRequest) (*model.Warehouse, error) {
	var w model.Warehouse
	err := r.db.QueryRow(ctx,
		`UPDATE warehouses
		 SET name = COALESCE($1, name)
		 WHERE id = $2 AND tenant_id = $3
		 RETURNING id, tenant_id, name, created_at`,
		req.Name, id, tenantID,
	).Scan(&w.ID, &w.TenantID, &w.Name, &w.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("singledb.WarehouseRepo.Update: %w", err)
	}
	return &w, nil
}

// List returns a paginated, filtered list of warehouses for a tenant.
func (r *WarehouseRepo) List(ctx context.Context, tenantID int, q dto.PageQuery, f dto.WarehouseFilter) ([]model.Warehouse, int, error) {
	args := []any{tenantID}
	where := "WHERE tenant_id = $1"

	if f.Search != "" {
		args = append(args, "%"+f.Search+"%")
		where += fmt.Sprintf(" AND name ILIKE $%d", len(args))
	}
	if f.DateFrom != nil {
		args = append(args, *f.DateFrom)
		where += fmt.Sprintf(" AND created_at >= $%d", len(args))
	}
	if f.DateTo != nil {
		args = append(args, *f.DateTo)
		where += fmt.Sprintf(" AND created_at <= $%d", len(args))
	}

	args = append(args, q.Limit, q.Offset())
	limitIdx := len(args) - 1
	offsetIdx := len(args)

	query := fmt.Sprintf(`
		SELECT id, tenant_id, name, created_at,
		       COUNT(*) OVER() AS total_count
		FROM warehouses
		%s
		ORDER BY %s %s
		LIMIT $%d OFFSET $%d`,
		where, q.Sort, q.Order, limitIdx, offsetIdx,
	)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("singledb.WarehouseRepo.List: %w", err)
	}
	defer rows.Close()

	var (
		list  []model.Warehouse
		total int
	)
	for rows.Next() {
		var w model.Warehouse
		if err := rows.Scan(&w.ID, &w.TenantID, &w.Name, &w.CreatedAt, &total); err != nil {
			return nil, 0, fmt.Errorf("singledb.WarehouseRepo.List scan: %w", err)
		}
		list = append(list, w)
	}
	return list, total, rows.Err()
}
