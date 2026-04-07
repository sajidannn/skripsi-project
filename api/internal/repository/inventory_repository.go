package repository

import (
	"context"

	"github.com/sajidannn/pos-api/internal/model"
)

// InventoryFilter carries optional query parameters for inventory listings.
type InventoryFilter struct {
	LowStock bool // when true, only return items whose stock == 0
}

// InventoryRepository is the data-access contract for branch/warehouse inventory.
type InventoryRepository interface {
	// ListByBranch returns all inventory entries for a given branch.
	ListByBranch(ctx context.Context, tenantID, branchID int, f InventoryFilter) ([]model.BranchItem, error)

	// ListByWarehouse returns all inventory entries for a given warehouse.
	ListByWarehouse(ctx context.Context, tenantID, warehouseID int, f InventoryFilter) ([]model.WarehouseItem, error)
}
