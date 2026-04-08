package repository

import (
	"context"

	"github.com/sajidannn/pos-api/internal/dto"
	"github.com/sajidannn/pos-api/internal/model"
)

// BranchRepository is the data-access contract for branches.
type BranchRepository interface {
	// Create inserts a new branch record.
	Create(ctx context.Context, tenantID int, req dto.CreateBranchRequest) (*model.Branch, error)

	// Update a branch.
	Update(ctx context.Context, tenantID, id int, req dto.UpdateBranchRequest) (*model.Branch, error)

	// GetByID fetches a single branch.
	GetByID(ctx context.Context, tenantID, id int) (*model.Branch, error)

	// List returns a paginated, filtered list of branches that belong to the tenant.
	List(ctx context.Context, tenantID int, q dto.PageQuery, f dto.BranchFilter) ([]model.Branch, int, error)
}
