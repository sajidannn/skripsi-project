package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

// ── Filter ────────────────────────────────────────────────────────────────────

// ItemFilter holds the optional query-string filters for GET /items.
// All fields are optional; zero values mean "no filter applied".
//
// Supported query params:
//
//	?search=boots&sku=ITEM-XYZ&min_price=10000&max_price=500000
//	&date_from=2024-01-01&date_to=2024-12-31
type ItemFilter struct {
	// Search does a case-insensitive partial match across name, sku, and description.
	// Implemented with ILIKE which is safer than full-text search when no FTS index exists.
	Search string `form:"search"`

	// SKU filters for an exact match on the sku column.
	SKU string `form:"sku"`

	// MinCost / MaxCost bound the cost column (inclusive).
	MinCost decimal.Decimal `form:"min_cost"`
	MaxCost decimal.Decimal `form:"max_cost"`

	// MinPrice / MaxPrice bound the price column (inclusive).
	MinPrice decimal.Decimal `form:"min_price"`
	MaxPrice decimal.Decimal `form:"max_price"`

	// DateFrom / DateTo bound the created_at column (inclusive).
	// Expected format: YYYY-MM-DD (parsed by the handler).
	DateFrom *time.Time
	DateTo   *time.Time
}

// ── Request ──────────────────────────────────────────────────────────────────

// CreateItemRequest is the validated HTTP request body for POST /items.
type CreateItemRequest struct {
	Name        string          `json:"name"        binding:"required,min=1,max=255"`
	SKU         string          `json:"sku"         binding:"omitempty,max=100"`
	Cost            decimal.Decimal `json:"cost"             binding:"required"`
	Price           decimal.Decimal `json:"price"            binding:"required"`
	MarginThreshold decimal.Decimal `json:"margin_threshold" binding:"omitempty"`
	Description     string          `json:"description"      binding:"omitempty,max=1000"`
}

// UpdateItemRequest is the validated HTTP request body for PUT /items/:id.
type UpdateItemRequest struct {
	Name        string           `json:"name"        binding:"omitempty,min=1,max=255"`
	SKU         string           `json:"sku"         binding:"omitempty,max=100"`
	Cost            *decimal.Decimal `json:"cost"             binding:"omitempty"`
	Price           *decimal.Decimal `json:"price"            binding:"omitempty"`
	MarginThreshold *decimal.Decimal `json:"margin_threshold" binding:"omitempty"`
	Description     string           `json:"description"      binding:"omitempty,max=1000"`
}

// ── Response ─────────────────────────────────────────────────────────────────

// ItemResponse is the outbound representation of a catalogue item.
type ItemResponse struct {
	ID          int             `json:"id"`
	Name        string          `json:"name"`
	SKU         string          `json:"sku"`
	Cost            decimal.Decimal `json:"cost"`
	Price           decimal.Decimal `json:"price"`
	MarginThreshold decimal.Decimal `json:"margin_threshold"`
	Description     string          `json:"description"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}
