// Package multidb provides repository implementations for multi-DB mode.
// Tenant isolation is achieved at the database level — each tenant has its own
// PostgreSQL database.  There is no tenant_id column in these schemas; the
// Manager selects the correct connection pool per request.
package multidb

import (
	"context"
	"fmt"

	"github.com/sajidannn/pos-api/internal/db/multidb"
	"github.com/sajidannn/pos-api/internal/dto"
	"github.com/sajidannn/pos-api/internal/model"
)

// WarehouseRepo implements repository.WarehouseRepository for multi-DB mode.
type WarehouseRepo struct {
	mgr *multidb.Manager
}

// NewWarehouseRepo creates a new WarehouseRepo backed by the tenant Manager.
func NewWarehouseRepo(mgr *multidb.Manager) *WarehouseRepo {
	return &WarehouseRepo{mgr: mgr}
}

// Create inserts a warehouse row into the tenant's own database.
func (r *WarehouseRepo) Create(ctx context.Context, tenantID int, req dto.CreateWarehouseRequest) (*model.Warehouse, error) {
	pool, err := r.mgr.Pool(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	var w model.Warehouse
	err = pool.QueryRow(ctx,
		`INSERT INTO warehouses (name)
		 VALUES ($1)
		 RETURNING id, name, created_at`,
		req.Name,
	).Scan(&w.ID, &w.Name, &w.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("multidb.WarehouseRepo.Create: %w", err)
	}
	// TenantID is not stored in multi-DB schema; set it for the response struct
	w.TenantID = tenantID
	return &w, nil
}

// GetByID fetches a warehouse by ID from the tenant's database.
func (r *WarehouseRepo) GetByID(ctx context.Context, tenantID, id int) (*model.Warehouse, error) {
	pool, err := r.mgr.Pool(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	var w model.Warehouse
	err = pool.QueryRow(ctx,
		`SELECT id, name, created_at FROM warehouses WHERE id = $1`,
		id,
	).Scan(&w.ID, &w.Name, &w.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("multidb.WarehouseRepo.GetByID: %w", err)
	}
	w.TenantID = tenantID
	return &w, nil
}

// List returns a paginated, filtered list of warehouses from the tenant's database.
func (r *WarehouseRepo) List(ctx context.Context, tenantID int, q dto.PageQuery, f dto.WarehouseFilter) ([]model.Warehouse, int, error) {
	pool, err := r.mgr.Pool(ctx, tenantID)
	if err != nil {
		return nil, 0, err
	}

	var args []any
	where := "WHERE TRUE"

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

	// Query 1: Count total
	countQuery := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM warehouses
		%s`, where)

	var total int
	err = pool.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("multidb.WarehouseRepo.List count: %w", err)
	}

	if total == 0 {
		return []model.Warehouse{}, 0, nil
	}

	// Query 2: Get data
	dataArgs := append(args, q.Limit, q.Offset())
	limitIdx := len(dataArgs) - 1
	offsetIdx := len(dataArgs)

	query := fmt.Sprintf(`
		SELECT id, name, created_at
		FROM warehouses
		%s
		ORDER BY %s %s
		LIMIT $%d OFFSET $%d`,
		where, q.Sort, q.Order, limitIdx, offsetIdx,
	)

	rows, err := pool.Query(ctx, query, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("multidb.WarehouseRepo.List data: %w", err)
	}
	defer rows.Close()

	var list []model.Warehouse
	for rows.Next() {
		var w model.Warehouse
		if err := rows.Scan(&w.ID, &w.Name, &w.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("multidb.WarehouseRepo.List scan: %w", err)
		}
		w.TenantID = tenantID
		list = append(list, w)
	}
	return list, total, rows.Err()
}

func (r *WarehouseRepo) Update(ctx context.Context, tenantID, id int, req dto.UpdateWarehouseRequest) (*model.Warehouse, error) {
	pool, err := r.mgr.Pool(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	var w model.Warehouse
	err = pool.QueryRow(ctx,
		`UPDATE warehouses
		 SET name = COALESCE($1, name)
		 WHERE id = $2
		 RETURNING id, name, created_at`,
		req.Name, id,
	).Scan(&w.ID, &w.Name, &w.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("multidb.WarehouseRepo.Update: %w", err)
	}
	w.TenantID = tenantID
	return &w, nil
}
