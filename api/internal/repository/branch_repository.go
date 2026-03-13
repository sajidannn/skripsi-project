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

	// GetByID fetches a single branch.
	GetByID(ctx context.Context, tenantID, id int) (*model.Branch, error)

	// List returns all branches that belong to the tenant.
	List(ctx context.Context, tenantID int) ([]model.Branch, error)
}
