package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

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
	ID        int             `json:"id"`
	BranchID  int             `json:"branch_id"`
	ItemID    int             `json:"item_id"`
	ItemName  string          `json:"item_name"`
	SKU       string          `json:"sku"`
	Stock     int             `json:"stock"`
	Cost            decimal.Decimal  `json:"cost"`
	BasePrice       decimal.Decimal  `json:"base_price"`
	BranchPrice     *decimal.Decimal `json:"branch_price,omitempty"`
	FinalPrice      decimal.Decimal  `json:"final_price"`
	MarginPercent   float64          `json:"margin_percent"`
	MarginWarning   bool             `json:"margin_warning"`
	MarginThreshold decimal.Decimal  `json:"margin_threshold"`
	UpdatedAt       time.Time        `json:"updated_at"`
}

// WarehouseItemResponse is the outbound representation of a warehouse inventory entry.
type WarehouseItemResponse struct {
	ID          int             `json:"id"`
	WarehouseID int             `json:"warehouse_id"`
	ItemID      int             `json:"item_id"`
	ItemName    string          `json:"item_name"`
	SKU         string          `json:"sku"`
	Stock       int             `json:"stock"`
	Cost        decimal.Decimal `json:"cost"`
	Price       decimal.Decimal `json:"price"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// ── Request ──────────────────────────────────────────────────────────────────

// UpdateBranchItemPriceRequest is the validated body for PUT /inventory/branch/{branch_id}/item/{item_id}
type UpdateBranchItemPriceRequest struct {
	Price           *decimal.Decimal `json:"price"            binding:"omitempty"`
	MarginThreshold *decimal.Decimal `json:"margin_threshold" binding:"omitempty"`
}
