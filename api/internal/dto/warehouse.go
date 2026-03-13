package dto

import "time"

// ── Request ──────────────────────────────────────────────────────────────────

// CreateWarehouseRequest is the validated HTTP request body for POST /warehouses.
type CreateWarehouseRequest struct {
	Name string `json:"name" binding:"required,min=1,max=255"`
}

// ── Response ─────────────────────────────────────────────────────────────────

// WarehouseResponse is the outbound representation of a warehouse.
// It deliberately omits tenant_id — clients have no need for internal routing info.
type WarehouseResponse struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}
