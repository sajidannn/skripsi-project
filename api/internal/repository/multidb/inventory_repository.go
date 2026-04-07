package multidb

import (
	"context"
	"fmt"

	"github.com/sajidannn/pos-api/internal/db/multidb"
	"github.com/sajidannn/pos-api/internal/model"
	"github.com/sajidannn/pos-api/internal/repository"
)

// InventoryRepo implements repository.InventoryRepository for multi-DB mode.
type InventoryRepo struct {
	mgr *multidb.Manager
}

// NewInventoryRepo creates a new InventoryRepo backed by the tenant Manager.
func NewInventoryRepo(mgr *multidb.Manager) *InventoryRepo {
	return &InventoryRepo{mgr: mgr}
}

// ListByBranch returns all inventory entries for a branch from the tenant's database.
func (r *InventoryRepo) ListByBranch(ctx context.Context, tenantID, branchID int, f repository.InventoryFilter) ([]model.BranchItem, error) {
	pool, err := r.mgr.Pool(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT bi.id, bi.branch_id, bi.item_id,
		       it.name, it.sku, it.price, bi.stock, bi.updated_at
		FROM branch_items bi
		JOIN items it ON it.id = bi.item_id
		WHERE bi.branch_id = $1`
	if f.LowStock {
		query += ` AND bi.stock = 0`
	}
	query += ` ORDER BY bi.id`

	rows, err := pool.Query(ctx, query, branchID)
	if err != nil {
		return nil, fmt.Errorf("multidb.InventoryRepo.ListByBranch: %w", err)
	}
	defer rows.Close()

	var list []model.BranchItem
	for rows.Next() {
		var bi model.BranchItem
		if err := rows.Scan(&bi.ID, &bi.BranchID, &bi.ItemID,
			&bi.ItemName, &bi.SKU, &bi.Price, &bi.Stock, &bi.UpdatedAt); err != nil {
			return nil, fmt.Errorf("multidb.InventoryRepo.ListByBranch scan: %w", err)
		}
		bi.TenantID = tenantID
		list = append(list, bi)
	}
	return list, rows.Err()
}

// ListByWarehouse returns all inventory entries for a warehouse from the tenant's database.
func (r *InventoryRepo) ListByWarehouse(ctx context.Context, tenantID, warehouseID int, f repository.InventoryFilter) ([]model.WarehouseItem, error) {
	pool, err := r.mgr.Pool(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT wi.id, wi.warehouse_id, wi.item_id,
		       it.name, it.sku, it.price, wi.stock, wi.updated_at
		FROM warehouse_items wi
		JOIN items it ON it.id = wi.item_id
		WHERE wi.warehouse_id = $1`
	if f.LowStock {
		query += ` AND wi.stock = 0`
	}
	query += ` ORDER BY wi.id`

	rows, err := pool.Query(ctx, query, warehouseID)
	if err != nil {
		return nil, fmt.Errorf("multidb.InventoryRepo.ListByWarehouse: %w", err)
	}
	defer rows.Close()

	var list []model.WarehouseItem
	for rows.Next() {
		var wi model.WarehouseItem
		if err := rows.Scan(&wi.ID, &wi.WarehouseID, &wi.ItemID,
			&wi.ItemName, &wi.SKU, &wi.Price, &wi.Stock, &wi.UpdatedAt); err != nil {
			return nil, fmt.Errorf("multidb.InventoryRepo.ListByWarehouse scan: %w", err)
		}
		wi.TenantID = tenantID
		list = append(list, wi)
	}
	return list, rows.Err()
}
