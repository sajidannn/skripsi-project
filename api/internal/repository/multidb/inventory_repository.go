package multidb

import (
	"context"
	"fmt"

	"github.com/sajidannn/pos-api/internal/db/multidb"
	"github.com/sajidannn/pos-api/internal/dto"
	"github.com/sajidannn/pos-api/internal/model"
)

// InventoryRepo implements repository.InventoryRepository for multi-DB mode.
type InventoryRepo struct {
	mgr *multidb.Manager
}

// NewInventoryRepo creates a new InventoryRepo backed by the tenant Manager.
func NewInventoryRepo(mgr *multidb.Manager) *InventoryRepo {
	return &InventoryRepo{mgr: mgr}
}

// ListByBranch returns a paginated, filtered list of inventory entries for a branch.
func (r *InventoryRepo) ListByBranch(ctx context.Context, tenantID, branchID int, q dto.PageQuery, f dto.InventoryFilter) ([]model.BranchItem, int, error) {
	pool, err := r.mgr.Pool(ctx, tenantID)
	if err != nil {
		return nil, 0, err
	}

	args := []any{branchID}
	where := "WHERE bi.branch_id = $1"

	if f.LowStock {
		where += ` AND bi.stock <= 0`
	}
	if f.Search != "" {
		args = append(args, "%"+f.Search+"%")
		n := len(args)
		where += fmt.Sprintf(" AND (it.name ILIKE $%d OR it.sku ILIKE $%d)", n, n)
	}
	if f.DateFrom != nil {
		args = append(args, *f.DateFrom)
		where += fmt.Sprintf(" AND bi.updated_at >= $%d", len(args))
	}
	if f.DateTo != nil {
		args = append(args, *f.DateTo)
		where += fmt.Sprintf(" AND bi.updated_at <= $%d", len(args))
	}

	args = append(args, q.Limit, q.Offset())
	limitIdx := len(args) - 1
	offsetIdx := len(args)

	query := fmt.Sprintf(`
		SELECT bi.id, bi.branch_id, bi.item_id,
		       it.name, it.sku, it.price, bi.stock, bi.updated_at,
		       COUNT(*) OVER() AS total_count
		FROM branch_items bi
		JOIN items it ON it.id = bi.item_id
		%s
		ORDER BY bi.%s %s
		LIMIT $%d OFFSET $%d`,
		where, q.Sort, q.Order, limitIdx, offsetIdx,
	)

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("multidb.InventoryRepo.ListByBranch: %w", err)
	}
	defer rows.Close()

	var (
		list  []model.BranchItem
		total int
	)
	for rows.Next() {
		var bi model.BranchItem
		if err := rows.Scan(&bi.ID, &bi.BranchID, &bi.ItemID,
			&bi.ItemName, &bi.SKU, &bi.Price, &bi.Stock, &bi.UpdatedAt, &total); err != nil {
			return nil, 0, fmt.Errorf("multidb.InventoryRepo.ListByBranch scan: %w", err)
		}
		bi.TenantID = tenantID
		list = append(list, bi)
	}
	return list, total, rows.Err()
}

// ListByWarehouse returns a paginated, filtered list of inventory entries for a warehouse.
func (r *InventoryRepo) ListByWarehouse(ctx context.Context, tenantID, warehouseID int, q dto.PageQuery, f dto.InventoryFilter) ([]model.WarehouseItem, int, error) {
	pool, err := r.mgr.Pool(ctx, tenantID)
	if err != nil {
		return nil, 0, err
	}

	args := []any{warehouseID}
	where := "WHERE wi.warehouse_id = $1"

	if f.LowStock {
		where += ` AND wi.stock <= 0`
	}
	if f.Search != "" {
		args = append(args, "%"+f.Search+"%")
		n := len(args)
		where += fmt.Sprintf(" AND (it.name ILIKE $%d OR it.sku ILIKE $%d)", n, n)
	}
	if f.DateFrom != nil {
		args = append(args, *f.DateFrom)
		where += fmt.Sprintf(" AND wi.updated_at >= $%d", len(args))
	}
	if f.DateTo != nil {
		args = append(args, *f.DateTo)
		where += fmt.Sprintf(" AND wi.updated_at <= $%d", len(args))
	}

	args = append(args, q.Limit, q.Offset())
	limitIdx := len(args) - 1
	offsetIdx := len(args)

	query := fmt.Sprintf(`
		SELECT wi.id, wi.warehouse_id, wi.item_id,
		       it.name, it.sku, it.price, wi.stock, wi.updated_at,
		       COUNT(*) OVER() AS total_count
		FROM warehouse_items wi
		JOIN items it ON it.id = wi.item_id
		%s
		ORDER BY wi.%s %s
		LIMIT $%d OFFSET $%d`,
		where, q.Sort, q.Order, limitIdx, offsetIdx,
	)

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("multidb.InventoryRepo.ListByWarehouse: %w", err)
	}
	defer rows.Close()

	var (
		list  []model.WarehouseItem
		total int
	)
	for rows.Next() {
		var wi model.WarehouseItem
		if err := rows.Scan(&wi.ID, &wi.WarehouseID, &wi.ItemID,
			&wi.ItemName, &wi.SKU, &wi.Price, &wi.Stock, &wi.UpdatedAt, &total); err != nil {
			return nil, 0, fmt.Errorf("multidb.InventoryRepo.ListByWarehouse scan: %w", err)
		}
		wi.TenantID = tenantID
		list = append(list, wi)
	}
	return list, total, rows.Err()
}
