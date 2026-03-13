package service

import (
	"context"
	"fmt"

	"github.com/sajidannn/pos-api/internal/dto"
	"github.com/sajidannn/pos-api/internal/model"
	"github.com/sajidannn/pos-api/internal/repository"
	"github.com/sajidannn/pos-api/internal/tenant"
)

// BranchService handles business logic for branches.
type BranchService struct {
	repo repository.BranchRepository
}

// NewBranchService constructs a BranchService.
func NewBranchService(repo repository.BranchRepository) *BranchService {
	return &BranchService{repo: repo}
}

// Create validates the request and delegates to the repository.
func (s *BranchService) Create(ctx context.Context, req dto.CreateBranchRequest) (*model.Branch, error) {
	tenantID, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("BranchService.Create: %w", err)
	}
	return s.repo.Create(ctx, tenantID, req)
}

// GetByID retrieves a single branch, scoped to the tenant in ctx.
func (s *BranchService) GetByID(ctx context.Context, id int) (*model.Branch, error) {
	tenantID, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("BranchService.GetByID: %w", err)
	}
	return s.repo.GetByID(ctx, tenantID, id)
}

// List returns all branches for the tenant in ctx.
func (s *BranchService) List(ctx context.Context) ([]model.Branch, error) {
	tenantID, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("BranchService.List: %w", err)
	}
	return s.repo.List(ctx, tenantID)
}
