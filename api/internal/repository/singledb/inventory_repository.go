package singledb

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sajidannn/pos-api/internal/model"
	"github.com/sajidannn/pos-api/internal/repository"
)

// InventoryRepo implements repository.InventoryRepository for single-DB mode.
type InventoryRepo struct {
	db *pgxpool.Pool
}

// NewInventoryRepo creates a new InventoryRepo backed by the shared pool.
func NewInventoryRepo(db *pgxpool.Pool) *InventoryRepo {
	return &InventoryRepo{db: db}
}

// ListByBranch returns all inventory entries for a branch, scoped to the tenant.
func (r *InventoryRepo) ListByBranch(ctx context.Context, tenantID, branchID int, f repository.InventoryFilter) ([]model.BranchItem, error) {
	query := `
		SELECT bi.id, bi.tenant_id, bi.branch_id, bi.item_id,
		       it.name, it.sku, it.price, bi.stock, bi.updated_at
		FROM branch_items bi
		JOIN items it ON it.id = bi.item_id
		WHERE bi.tenant_id = $1 AND bi.branch_id = $2`
	if f.LowStock {
		query += ` AND bi.stock = 0`
	}
	query += ` ORDER BY bi.id`

	rows, err := r.db.Query(ctx, query, tenantID, branchID)
	if err != nil {
		return nil, fmt.Errorf("singledb.InventoryRepo.ListByBranch: %w", err)
	}
	defer rows.Close()

	var list []model.BranchItem
	for rows.Next() {
		var bi model.BranchItem
		if err := rows.Scan(&bi.ID, &bi.TenantID, &bi.BranchID, &bi.ItemID,
			&bi.ItemName, &bi.SKU, &bi.Price, &bi.Stock, &bi.UpdatedAt); err != nil {
			return nil, fmt.Errorf("singledb.InventoryRepo.ListByBranch scan: %w", err)
		}
		list = append(list, bi)
	}
	return list, rows.Err()
}

// ListByWarehouse returns all inventory entries for a warehouse, scoped to the tenant.
func (r *InventoryRepo) ListByWarehouse(ctx context.Context, tenantID, warehouseID int, f repository.InventoryFilter) ([]model.WarehouseItem, error) {
	query := `
		SELECT wi.id, wi.tenant_id, wi.warehouse_id, wi.item_id,
		       it.name, it.sku, it.price, wi.stock, wi.updated_at
		FROM warehouse_items wi
		JOIN items it ON it.id = wi.item_id
		WHERE wi.tenant_id = $1 AND wi.warehouse_id = $2`
	if f.LowStock {
		query += ` AND wi.stock = 0`
	}
	query += ` ORDER BY wi.id`

	rows, err := r.db.Query(ctx, query, tenantID, warehouseID)
	if err != nil {
		return nil, fmt.Errorf("singledb.InventoryRepo.ListByWarehouse: %w", err)
	}
	defer rows.Close()

	var list []model.WarehouseItem
	for rows.Next() {
		var wi model.WarehouseItem
		if err := rows.Scan(&wi.ID, &wi.TenantID, &wi.WarehouseID, &wi.ItemID,
			&wi.ItemName, &wi.SKU, &wi.Price, &wi.Stock, &wi.UpdatedAt); err != nil {
			return nil, fmt.Errorf("singledb.InventoryRepo.ListByWarehouse scan: %w", err)
		}
		list = append(list, wi)
	}
	return list, rows.Err()
}
