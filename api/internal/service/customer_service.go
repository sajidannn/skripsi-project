package service

import (
	"context"
	"fmt"

	"github.com/sajidannn/pos-api/internal/dto"
	"github.com/sajidannn/pos-api/internal/model"
	"github.com/sajidannn/pos-api/internal/repository"
	"github.com/sajidannn/pos-api/internal/tenant"
)

type CustomerService struct {
	repo repository.CustomerRepository
}

func NewCustomerService(repo repository.CustomerRepository) *CustomerService {
	return &CustomerService{repo: repo}
}

func (s *CustomerService) Create(ctx context.Context, req dto.CreateCustomerRequest) (*model.Customer, error) {
	tenantID, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("CustomerService.Create: %w", err)
	}
	return s.repo.Create(ctx, tenantID, req)
}

func (s *CustomerService) GetByID(ctx context.Context, id int) (*model.Customer, error) {
	tenantID, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("CustomerService.GetByID: %w", err)
	}
	return s.repo.GetByID(ctx, tenantID, id)
}

func (s *CustomerService) List(ctx context.Context, branchID int) ([]model.Customer, error) {
	tenantID, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("CustomerService.List: %w", err)
	}
	return s.repo.List(ctx, tenantID, branchID)
}

func (s *CustomerService) Update(ctx context.Context, id int, req dto.UpdateCustomerRequest) (*model.Customer, error) {
	tenantID, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("CustomerService.Update: %w", err)
	}
	return s.repo.Update(ctx, tenantID, id, req)
}

func (s *CustomerService) Delete(ctx context.Context, id int) error {
	tenantID, err := tenant.FromContext(ctx)
	if err != nil {
		return fmt.Errorf("CustomerService.Delete: %w", err)
	}
	return s.repo.Delete(ctx, tenantID, id)
}
