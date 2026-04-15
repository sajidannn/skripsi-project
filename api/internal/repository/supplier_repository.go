package repository

import (
	"context"

	"github.com/sajidannn/pos-api/internal/dto"
	"github.com/sajidannn/pos-api/internal/model"
)

// SupplierRepository is the data-access contract for suppliers.
type SupplierRepository interface {
	Create(ctx context.Context, tenantID int, req dto.CreateSupplierRequest) (*model.Supplier, error)
	GetByID(ctx context.Context, tenantID, id int) (*model.Supplier, error)
	// List returns a paginated, filtered list of suppliers for the tenant.
	List(ctx context.Context, tenantID int, q dto.PageQuery, f dto.SupplierFilter) (suppliers []model.Supplier, total int, err error)
	Update(ctx context.Context, tenantID, id int, req dto.UpdateSupplierRequest) (*model.Supplier, error)
	Delete(ctx context.Context, tenantID, id int) error
}
