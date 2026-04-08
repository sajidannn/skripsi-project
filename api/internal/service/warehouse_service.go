package service

import (
	"context"
	"fmt"

	"github.com/sajidannn/pos-api/internal/dto"
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
func (s *WarehouseService) Create(ctx context.Context, req dto.CreateWarehouseRequest) (*model.Warehouse, error) {
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

// List returns a paginated, filtered list of warehouses for the tenant in ctx.
func (s *WarehouseService) List(ctx context.Context, q dto.PageQuery, f dto.WarehouseFilter) ([]model.Warehouse, int, error) {
	tenantID, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("WarehouseService.List: %w", err)
	}
	return s.repo.List(ctx, tenantID, q, f)
}

func (s *WarehouseService) Update(ctx context.Context, id int, req dto.UpdateWarehouseRequest) (*model.Warehouse, error) {
	tenantID, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("WarehouseService.Update: %w", err)
	}
	return s.repo.Update(ctx, tenantID, id, req)
}
