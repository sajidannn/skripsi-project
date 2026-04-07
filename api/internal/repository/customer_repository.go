package repository

import (
	"context"

	"github.com/sajidannn/pos-api/internal/dto"
	"github.com/sajidannn/pos-api/internal/model"
)

// CustomerRepository is the data-access contract for customers.
type CustomerRepository interface {
	Create(ctx context.Context, tenantID int, req dto.CreateCustomerRequest) (*model.Customer, error)
	GetByID(ctx context.Context, tenantID, id int) (*model.Customer, error)
	// List returns customers. If branchID is > 0, it filters by branch.
	List(ctx context.Context, tenantID, branchID int) ([]model.Customer, error)
	Update(ctx context.Context, tenantID, id int, req dto.UpdateCustomerRequest) (*model.Customer, error)
	Delete(ctx context.Context, tenantID, id int) error
}
