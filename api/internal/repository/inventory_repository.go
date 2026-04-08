package repository

import (
	"context"

	"github.com/sajidannn/pos-api/internal/dto"
	"github.com/sajidannn/pos-api/internal/model"
)

// InventoryRepository is the data-access contract for branch/warehouse inventory.
type InventoryRepository interface {
	// ListByBranch returns a paginated, filtered list of inventory entries for a given branch.
	ListByBranch(ctx context.Context, tenantID, branchID int, q dto.PageQuery, f dto.InventoryFilter) (entries []model.BranchItem, total int, err error)

	// ListByWarehouse returns a paginated, filtered list of inventory entries for a given warehouse.
	ListByWarehouse(ctx context.Context, tenantID, warehouseID int, q dto.PageQuery, f dto.InventoryFilter) (entries []model.WarehouseItem, total int, err error)
}
