package model

import (
	"time"

	"github.com/shopspring/decimal"
)

// BranchItem represents a product's stock level at a specific branch.
type BranchItem struct {
	ID        int             `json:"id"`
	TenantID  int             `json:"tenant_id,omitempty"` // only meaningful in single-db mode
	BranchID  int             `json:"branch_id"`
	ItemID    int             `json:"item_id"`
	ItemName  string          `json:"item_name"`
	SKU       string          `json:"sku"`
	Price     decimal.Decimal `json:"price"`
	Stock     int             `json:"stock"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// WarehouseItem represents a product's stock level at a specific warehouse.
type WarehouseItem struct {
	ID          int             `json:"id"`
	TenantID    int             `json:"tenant_id,omitempty"` // only meaningful in single-db mode
	WarehouseID int             `json:"warehouse_id"`
	ItemID      int             `json:"item_id"`
	ItemName    string          `json:"item_name"`
	SKU         string          `json:"sku"`
	Price       decimal.Decimal `json:"price"`
	Stock       int             `json:"stock"`
	UpdatedAt   time.Time       `json:"updated_at"`
}
