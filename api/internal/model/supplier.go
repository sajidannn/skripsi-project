package model

import "time"

// Supplier is the domain model.
type Supplier struct {
	ID        int       `json:"id"`
	TenantID  int       `json:"tenant_id,omitempty"` // only meaningful in single-db mode
	Name      string    `json:"name"`
	Phone     *string   `json:"phone,omitempty"`
	Address   *string   `json:"address,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
