package service

import (
	"context"
	"fmt"

	"github.com/sajidannn/pos-api/internal/dto"
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
func (s *InventoryService) ListByBranch(ctx context.Context, branchID int, q dto.PageQuery, f dto.InventoryFilter) ([]model.BranchItem, int, error) {
	tenantID, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("InventoryService.ListByBranch: %w", err)
	}
	return s.repo.ListByBranch(ctx, tenantID, branchID, q, f)
}

// ListByWarehouse returns inventory for a given warehouse, scoped to the tenant in ctx.
func (s *InventoryService) ListByWarehouse(ctx context.Context, warehouseID int, q dto.PageQuery, f dto.InventoryFilter) ([]model.WarehouseItem, int, error) {
	tenantID, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("InventoryService.ListByWarehouse: %w", err)
	}
	return s.repo.ListByWarehouse(ctx, tenantID, warehouseID, q, f)
}

// UpdateBranchItemPrice updates the override price and margin threshold for a branch item.
func (s *InventoryService) UpdateBranchItemPrice(ctx context.Context, branchID, itemID int, req dto.UpdateBranchItemPriceRequest) (*model.BranchItem, error) {
	tenantID, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("InventoryService.UpdateBranchItemPrice: %w", err)
	}
	return s.repo.UpdateBranchItemPrice(ctx, tenantID, branchID, itemID, req)
}
