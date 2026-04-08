package dto

import "time"

// ── Filter ────────────────────────────────────────────────────────────────────

// WarehouseFilter holds optional query-string filters for GET /warehouses.
type WarehouseFilter struct {
	Search   string `form:"search"` // case-insensitive partial match on name
	DateFrom *time.Time
	DateTo   *time.Time
}

// ── Request ──────────────────────────────────────────────────────────────────

// CreateWarehouseRequest is the validated HTTP request body for POST /warehouses.
type CreateWarehouseRequest struct {
	Name string `json:"name" binding:"required,min=1,max=255"`
}

type UpdateWarehouseRequest struct {
	Name string `json:"name" binding:"omitempty,min=1,max=255"`
}

// ── Response ─────────────────────────────────────────────────────────────────

// WarehouseResponse is the outbound representation of a warehouse.
// It deliberately omits tenant_id — clients have no need for internal routing info.
type WarehouseResponse struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}
