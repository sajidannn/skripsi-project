package model

import "time"

// Branch is the domain model.
type Branch struct {
	ID             int       `json:"id"`
	TenantID       int       `json:"tenant_id,omitempty"` // only meaningful in single-db mode
	Name           string    `json:"name"`
	Phone          string    `json:"phone"`
	Address        string    `json:"address"`
	OpeningBalance float64   `json:"opening_balance"`
	CreatedAt      time.Time `json:"created_at"`
}
