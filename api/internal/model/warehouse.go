package model

import "time"

// Warehouse is the domain model — tenant_id is present to keep a single unified
// struct for both DB modes. In multi-DB mode tenant_id will always be 0 (unused).
type Warehouse struct {
	ID        int       `json:"id"`
	TenantID  int       `json:"tenant_id,omitempty"` // only meaningful in single-db mode
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateWarehouseRequest is the validated request payload.
type CreateWarehouseRequest struct {
	Name string `json:"name" binding:"required,min=1,max=255"`
}
