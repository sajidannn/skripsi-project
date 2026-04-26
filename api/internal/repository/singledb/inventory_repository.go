package singledb

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sajidannn/pos-api/internal/apierr"
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
		where += ` AND bi.stock <= 20`
	}
	if f.MarginWarning {
		where += ` AND COALESCE(bi.margin_threshold, it.margin_threshold) > 0 AND (((COALESCE(bi.price, it.price) - it.cost) / NULLIF(it.cost, 0)) * 100.0) <= COALESCE(bi.margin_threshold, it.margin_threshold)`
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

	// Query 1: Count total
	countQuery := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM branch_items bi
		JOIN items it ON it.id = bi.item_id
		%s`, where)

	var total int
	err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("singledb.InventoryRepo.ListByBranch count: %w", err)
	}

	if total == 0 {
		return []model.BranchItem{}, 0, nil
	}

	// Query 2: Get data
	dataArgs := append(args, q.Limit, q.Offset())
	limitIdx := len(dataArgs) - 1
	offsetIdx := len(dataArgs)

	query := fmt.Sprintf(`
		SELECT bi.id, bi.tenant_id, bi.branch_id, bi.item_id,
		       it.name, it.sku, bi.stock, it.cost, it.price, bi.price, COALESCE(bi.margin_threshold, it.margin_threshold), bi.updated_at
		FROM branch_items bi
		JOIN items it ON it.id = bi.item_id
		%s
		ORDER BY bi.%s %s
		LIMIT $%d OFFSET $%d`,
		where, q.Sort, q.Order, limitIdx, offsetIdx,
	)

	rows, err := r.db.Query(ctx, query, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("singledb.InventoryRepo.ListByBranch data: %w", err)
	}
	defer rows.Close()

	var list []model.BranchItem
	for rows.Next() {
		var bi model.BranchItem
		if err := rows.Scan(&bi.ID, &bi.TenantID, &bi.BranchID, &bi.ItemID,
			&bi.ItemName, &bi.SKU, &bi.Stock, &bi.Cost, &bi.BasePrice, &bi.BranchPrice, &bi.MarginThreshold, &bi.UpdatedAt); err != nil {
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

	// Query 1: Count total
	countQuery := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM warehouse_items wi
		JOIN items it ON it.id = wi.item_id
		%s`, where)

	var total int
	err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("singledb.InventoryRepo.ListByWarehouse count: %w", err)
	}

	if total == 0 {
		return []model.WarehouseItem{}, 0, nil
	}

	// Query 2: Get data
	dataArgs := append(args, q.Limit, q.Offset())
	limitIdx := len(dataArgs) - 1
	offsetIdx := len(dataArgs)

	query := fmt.Sprintf(`
		SELECT wi.id, wi.tenant_id, wi.warehouse_id, wi.item_id,
		       it.name, it.sku, it.cost, it.price, wi.stock, wi.updated_at
		FROM warehouse_items wi
		JOIN items it ON it.id = wi.item_id
		%s
		ORDER BY wi.%s %s
		LIMIT $%d OFFSET $%d`,
		where, q.Sort, q.Order, limitIdx, offsetIdx,
	)

	rows, err := r.db.Query(ctx, query, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("singledb.InventoryRepo.ListByWarehouse data: %w", err)
	}
	defer rows.Close()

	var list []model.WarehouseItem
	for rows.Next() {
		var wi model.WarehouseItem
		if err := rows.Scan(&wi.ID, &wi.TenantID, &wi.WarehouseID, &wi.ItemID,
			&wi.ItemName, &wi.SKU, &wi.Cost, &wi.Price, &wi.Stock, &wi.UpdatedAt); err != nil {
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
			return nil, apierr.NotFound("branch item not found")
		}
		return nil, fmt.Errorf("singledb.InventoryRepo.UpdateBranchItemPrice: %w", err)
	}
	return &bi, nil
}
