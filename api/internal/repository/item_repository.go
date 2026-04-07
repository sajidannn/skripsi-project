package repository

import (
	"context"

	"github.com/sajidannn/pos-api/internal/dto"
	"github.com/sajidannn/pos-api/internal/model"
)

// ItemRepository is the data-access contract for catalogue items.
type ItemRepository interface {
	// Create inserts a new item record for the given tenant.
	Create(ctx context.Context, tenantID int, req dto.CreateItemRequest) (*model.Item, error)

	// GetByID fetches a single item scoped to the tenant.
	GetByID(ctx context.Context, tenantID, id int) (*model.Item, error)

	// List returns all items belonging to the tenant.
	List(ctx context.Context, tenantID int) ([]model.Item, error)

	// Update modifies an existing item record.
	Update(ctx context.Context, tenantID, id int, req dto.UpdateItemRequest) (*model.Item, error)

	// Delete removes an item record.
	Delete(ctx context.Context, tenantID, id int) error
}
