package singledb

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sajidannn/pos-api/internal/dto"
	"github.com/sajidannn/pos-api/internal/model"
)

// InventoryRepo implements repository.InventoryRepository for single-DB mode.
type InventoryRepo struct {
	db *pgxpool.Pool
}

// NewInventoryRepo creates a new InventoryRepo backed by the shared pool.
func NewInventoryRepo(db *pgxpool.Pool) *InventoryRepo {
	return &InventoryRepo{db: db}
}

// ListByBranch returns a paginated, filtered list of inventory entries for a branch.
func (r *InventoryRepo) ListByBranch(ctx context.Context, tenantID, branchID int, q dto.PageQuery, f dto.InventoryFilter) ([]model.BranchItem, int, error) {
	args := []any{tenantID, branchID}
	where := "WHERE bi.tenant_id = $1 AND bi.branch_id = $2"

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
		SELECT bi.id, bi.tenant_id, bi.branch_id, bi.item_id,
		       it.name, it.sku, bi.stock, it.cost, it.price, bi.price, COALESCE(bi.margin_threshold, it.margin_threshold), bi.updated_at,
		       COUNT(*) OVER() AS total_count
		FROM branch_items bi
		JOIN items it ON it.id = bi.item_id
		%s
		ORDER BY bi.%s %s
		LIMIT $%d OFFSET $%d`,
		where, q.Sort, q.Order, limitIdx, offsetIdx,
	)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("singledb.InventoryRepo.ListByBranch: %w", err)
	}
	defer rows.Close()

	var (
		list  []model.BranchItem
		total int
	)
	for rows.Next() {
		var bi model.BranchItem
		if err := rows.Scan(&bi.ID, &bi.TenantID, &bi.BranchID, &bi.ItemID,
			&bi.ItemName, &bi.SKU, &bi.Stock, &bi.Cost, &bi.BasePrice, &bi.BranchPrice, &bi.MarginThreshold, &bi.UpdatedAt, &total); err != nil {
			return nil, 0, fmt.Errorf("singledb.InventoryRepo.ListByBranch scan: %w", err)
		}
		list = append(list, bi)
	}
	return list, total, rows.Err()
}

// ListByWarehouse returns a paginated, filtered list of inventory entries for a warehouse.
func (r *InventoryRepo) ListByWarehouse(ctx context.Context, tenantID, warehouseID int, q dto.PageQuery, f dto.InventoryFilter) ([]model.WarehouseItem, int, error) {
	args := []any{tenantID, warehouseID}
	where := "WHERE wi.tenant_id = $1 AND wi.warehouse_id = $2"

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
		SELECT wi.id, wi.tenant_id, wi.warehouse_id, wi.item_id,
		       it.name, it.sku, it.cost, it.price, wi.stock, wi.updated_at,
		       COUNT(*) OVER() AS total_count
		FROM warehouse_items wi
		JOIN items it ON it.id = wi.item_id
		%s
		ORDER BY wi.%s %s
		LIMIT $%d OFFSET $%d`,
		where, q.Sort, q.Order, limitIdx, offsetIdx,
	)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("singledb.InventoryRepo.ListByWarehouse: %w", err)
	}
	defer rows.Close()

	var (
		list  []model.WarehouseItem
		total int
	)
	for rows.Next() {
		var wi model.WarehouseItem
		if err := rows.Scan(&wi.ID, &wi.TenantID, &wi.WarehouseID, &wi.ItemID,
			&wi.ItemName, &wi.SKU, &wi.Cost, &wi.Price, &wi.Stock, &wi.UpdatedAt, &total); err != nil {
			return nil, 0, fmt.Errorf("singledb.InventoryRepo.ListByWarehouse scan: %w", err)
		}
		list = append(list, wi)
	}
	return list, total, rows.Err()
}

// UpdateBranchItemPrice updates the override price and margin threshold for a branch item.
func (r *InventoryRepo) UpdateBranchItemPrice(ctx context.Context, tenantID, branchID, itemID int, req dto.UpdateBranchItemPriceRequest) (*model.BranchItem, error) {
	var bi model.BranchItem
	err := r.db.QueryRow(ctx,
		`UPDATE branch_items bi
		 SET price = COALESCE($1, bi.price), 
		     margin_threshold = COALESCE($2, bi.margin_threshold), 
		     updated_at = NOW()
		 FROM items it
		 WHERE bi.item_id = it.id 
		   AND bi.tenant_id = $3 
		   AND bi.branch_id = $4 
		   AND bi.item_id = $5
		 RETURNING bi.id, bi.tenant_id, bi.branch_id, bi.item_id,
		           it.name, it.sku, bi.stock, it.cost, it.price, bi.price, 
		           COALESCE(bi.margin_threshold, it.margin_threshold), bi.updated_at`,
		req.Price, req.MarginThreshold, tenantID, branchID, itemID,
	).Scan(&bi.ID, &bi.TenantID, &bi.BranchID, &bi.ItemID,
		&bi.ItemName, &bi.SKU, &bi.Stock, &bi.Cost, &bi.BasePrice, &bi.BranchPrice, &bi.MarginThreshold, &bi.UpdatedAt)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("singledb.InventoryRepo.UpdateBranchItemPrice: branch item not found")
		}
		return nil, fmt.Errorf("singledb.InventoryRepo.UpdateBranchItemPrice: %w", err)
	}
	return &bi, nil
}
