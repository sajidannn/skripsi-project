package dto

import "time"

// ── Filter ────────────────────────────────────────────────────────────────────

// SupplierFilter holds optional query-string filters for GET /suppliers.
type SupplierFilter struct {
	// Search does a case-insensitive partial match on name, phone, and address.
	Search string `form:"search"`

	// DateFrom / DateTo bound the created_at column (inclusive).
	DateFrom *time.Time
	DateTo   *time.Time
}

// CreateSupplierRequest is the validated body for POST /suppliers.
type CreateSupplierRequest struct {
	Name    string `json:"name"    binding:"required,min=1,max=255"`
	Phone   string `json:"phone"   binding:"omitempty,max=50"`
	Address string `json:"address" binding:"omitempty"`
}

// UpdateSupplierRequest is the validated body for PUT /suppliers/:id.
// All fields are optional.
type UpdateSupplierRequest struct {
	Name    string `json:"name"    binding:"omitempty,min=1,max=255"`
	Phone   string `json:"phone"   binding:"omitempty,max=50"`
	Address string `json:"address" binding:"omitempty"`
}

// SupplierResponse is the outbound representation of a supplier.
type SupplierResponse struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Phone     *string   `json:"phone,omitempty"`
	Address   *string   `json:"address,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
