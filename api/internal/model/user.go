package model

import "time"

// User is the domain model for a POS user (employee).
// The Password field is intentionally omitted — it is never returned to the client.
type User struct {
	ID        int       `json:"id"`
	TenantID  int       `json:"tenant_id,omitempty"` // only meaningful in single-db mode
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}
