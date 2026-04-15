package service

import (
	"context"
	"fmt"

	"github.com/sajidannn/pos-api/internal/dto"
	"github.com/sajidannn/pos-api/internal/model"
	"github.com/sajidannn/pos-api/internal/repository"
	"github.com/sajidannn/pos-api/internal/tenant"
)

type SupplierService struct {
	repo repository.SupplierRepository
}

func NewSupplierService(repo repository.SupplierRepository) *SupplierService {
	return &SupplierService{repo: repo}
}

func (s *SupplierService) Create(ctx context.Context, req dto.CreateSupplierRequest) (*model.Supplier, error) {
	tenantID, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("SupplierService.Create: %w", err)
	}
	return s.repo.Create(ctx, tenantID, req)
}

func (s *SupplierService) GetByID(ctx context.Context, id int) (*model.Supplier, error) {
	tenantID, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("SupplierService.GetByID: %w", err)
	}
	return s.repo.GetByID(ctx, tenantID, id)
}

func (s *SupplierService) List(ctx context.Context, q dto.PageQuery, f dto.SupplierFilter) ([]model.Supplier, int, error) {
	tenantID, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("SupplierService.List: %w", err)
	}
	return s.repo.List(ctx, tenantID, q, f)
}

func (s *SupplierService) Update(ctx context.Context, id int, req dto.UpdateSupplierRequest) (*model.Supplier, error) {
	tenantID, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("SupplierService.Update: %w", err)
	}
	return s.repo.Update(ctx, tenantID, id, req)
}

func (s *SupplierService) Delete(ctx context.Context, id int) error {
	tenantID, err := tenant.FromContext(ctx)
	if err != nil {
		return fmt.Errorf("SupplierService.Delete: %w", err)
	}
	return s.repo.Delete(ctx, tenantID, id)
}
