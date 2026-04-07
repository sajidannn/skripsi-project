package model

import "time"

// Customer is the domain model.
type Customer struct {
	ID        int       `json:"id"`
	TenantID  int       `json:"tenant_id,omitempty"` // only meaningful in single-db mode
	BranchID  int       `json:"branch_id"`
	Name      string    `json:"name"`
	Phone     *string   `json:"phone,omitempty"`
	Email     *string   `json:"email,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
