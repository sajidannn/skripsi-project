package dto

import "time"

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
