// Package repository defines repository interfaces for the POS domain.
// Both the single-DB and multi-DB implementations satisfy these interfaces,
// allowing the service layer to remain mode-agnostic.
package repository

import (
	"context"

	"github.com/sajidannn/pos-api/internal/dto"
	"github.com/sajidannn/pos-api/internal/model"
)

// WarehouseRepository is the data-access contract for warehouses.
type WarehouseRepository interface {
	// Create inserts a new warehouse record.
	// tenantID is always passed by the service layer; in multi-DB mode it is
	// used only to select the right connection pool — the column itself does
	// not exist in the tenant's schema.
	Create(ctx context.Context, tenantID int, req dto.CreateWarehouseRequest) (*model.Warehouse, error)

	// GetByID fetches a single warehouse.
	GetByID(ctx context.Context, tenantID, id int) (*model.Warehouse, error)

	// List returns all warehouses that belong to the tenant.
	List(ctx context.Context, tenantID int) ([]model.Warehouse, error)
}
