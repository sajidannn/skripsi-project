package service

import (
	"context"
	"fmt"

	"github.com/sajidannn/pos-api/internal/model"
	"github.com/sajidannn/pos-api/internal/repository"
	"github.com/sajidannn/pos-api/internal/tenant"
)

// InventoryService handles business logic for branch/warehouse inventory queries.
type InventoryService struct {
	repo repository.InventoryRepository
}

// NewInventoryService constructs an InventoryService.
func NewInventoryService(repo repository.InventoryRepository) *InventoryService {
	return &InventoryService{repo: repo}
}

// ListByBranch returns inventory for a given branch, scoped to the tenant in ctx.
func (s *InventoryService) ListByBranch(ctx context.Context, branchID int, lowStock bool) ([]model.BranchItem, error) {
	tenantID, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("InventoryService.ListByBranch: %w", err)
	}
	return s.repo.ListByBranch(ctx, tenantID, branchID, repository.InventoryFilter{LowStock: lowStock})
}

// ListByWarehouse returns inventory for a given warehouse, scoped to the tenant in ctx.
func (s *InventoryService) ListByWarehouse(ctx context.Context, warehouseID int, lowStock bool) ([]model.WarehouseItem, error) {
	tenantID, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("InventoryService.ListByWarehouse: %w", err)
	}
	return s.repo.ListByWarehouse(ctx, tenantID, warehouseID, repository.InventoryFilter{LowStock: lowStock})
}
