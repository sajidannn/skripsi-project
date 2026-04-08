package dto

import "time"

// ── Filter ────────────────────────────────────────────────────────────────────

// InventoryFilter holds optional query-string filters for GET /inventory/...
type InventoryFilter struct {
	// Search does a case-insensitive partial match on item name or SKU.
	Search string `form:"search"`

	// LowStock returns only items with stock <= 0 (or some threshold) when true.
	LowStock bool `form:"low_stock"`

	// DateFrom / DateTo bound the updated_at column (inclusive).
	DateFrom *time.Time
	DateTo   *time.Time
}

// ── Response ─────────────────────────────────────────────────────────────────

// BranchItemResponse is the outbound representation of a branch inventory entry.
type BranchItemResponse struct {
	ID        int       `json:"id"`
	BranchID  int       `json:"branch_id"`
	ItemID    int       `json:"item_id"`
	ItemName  string    `json:"item_name"`
	SKU       string    `json:"sku"`
	Price     float64   `json:"price"`
	Stock     int       `json:"stock"`
	UpdatedAt time.Time `json:"updated_at"`
}

// WarehouseItemResponse is the outbound representation of a warehouse inventory entry.
type WarehouseItemResponse struct {
	ID          int       `json:"id"`
	WarehouseID int       `json:"warehouse_id"`
	ItemID      int       `json:"item_id"`
	ItemName    string    `json:"item_name"`
	SKU         string    `json:"sku"`
	Price       float64   `json:"price"`
	Stock       int       `json:"stock"`
	UpdatedAt   time.Time `json:"updated_at"`
}
