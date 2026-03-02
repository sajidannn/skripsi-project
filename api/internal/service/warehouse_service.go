package service

import (
	"context"
	"fmt"

	"github.com/sajidannn/pos-api/internal/model"
	"github.com/sajidannn/pos-api/internal/repository"
	"github.com/sajidannn/pos-api/internal/tenant"
)

// WarehouseService handles business logic for warehouses.
type WarehouseService struct {
	repo repository.WarehouseRepository
}

// NewWarehouseService constructs a WarehouseService.
// The same service works regardless of whether repo is a single-DB or
// multi-DB implementation.
func NewWarehouseService(repo repository.WarehouseRepository) *WarehouseService {
	return &WarehouseService{repo: repo}
}

// Create validates the request and delegates to the repository.
func (s *WarehouseService) Create(ctx context.Context, req model.CreateWarehouseRequest) (*model.Warehouse, error) {
	tenantID, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("WarehouseService.Create: %w", err)
	}
	return s.repo.Create(ctx, tenantID, req)
}

// GetByID retrieves a single warehouse, scoped to the tenant in ctx.
func (s *WarehouseService) GetByID(ctx context.Context, id int) (*model.Warehouse, error) {
	tenantID, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("WarehouseService.GetByID: %w", err)
	}
	return s.repo.GetByID(ctx, tenantID, id)
}

// List returns all warehouses for the tenant in ctx.
func (s *WarehouseService) List(ctx context.Context) ([]model.Warehouse, error) {
	tenantID, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("WarehouseService.List: %w", err)
	}
	return s.repo.List(ctx, tenantID)
}
