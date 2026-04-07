package model

import "time"

// Tenant represents a single tenant (company/client) navigating the system.
type Tenant struct {
	ID        int        `json:"id"`
	Name      string     `json:"name"`
	// DBName is the specific database name in multi-db mode.
	// Empty/ignored for single-db mode.
	DBName    string     `json:"db_name,omitempty"`
	CreatedAt *time.Time `json:"created_at,omitempty"`
}
