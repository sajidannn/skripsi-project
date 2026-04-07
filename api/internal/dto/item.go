package dto

import "time"

// ── Request ──────────────────────────────────────────────────────────────────

// CreateItemRequest is the validated HTTP request body for POST /items.
type CreateItemRequest struct {
	Name        string  `json:"name"        binding:"required,min=1,max=255"`
	SKU         string  `json:"sku"         binding:"omitempty,max=100"`
	Price       float64 `json:"price"       binding:"required,min=0"`
	Description string  `json:"description" binding:"omitempty,max=1000"`
}

// UpdateItemRequest is the validated HTTP request body for PUT /items/:id.
type UpdateItemRequest struct {
	Name        string  `json:"name"        binding:"omitempty,min=1,max=255"`
	SKU         string  `json:"sku"         binding:"omitempty,max=100"`
	Price       float64 `json:"price"       binding:"omitempty,min=0"`
	Description string  `json:"description" binding:"omitempty,max=1000"`
}

// ── Response ─────────────────────────────────────────────────────────────────

// ItemResponse is the outbound representation of a catalogue item.
type ItemResponse struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	SKU         string    `json:"sku"`
	Price       float64   `json:"price"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
