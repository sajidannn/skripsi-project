package dto

import "time"

// ── Filter ────────────────────────────────────────────────────────────────────

// CustomerFilter holds optional query-string filters for GET /customers.
type CustomerFilter struct {
	// Search does a case-insensitive partial match on name, phone, and email.
	Search string `form:"search"`

	// BranchID filters customers by the branch they belong to.
	BranchID int `form:"branch_id"`

	// DateFrom / DateTo bound the created_at column (inclusive).
	DateFrom *time.Time
	DateTo   *time.Time
}

// CreateCustomerRequest is the validated body for POST /customers.
type CreateCustomerRequest struct {
	BranchID int    `json:"branch_id" binding:"required,gt=0"`
	Name     string `json:"name"      binding:"required,min=1,max=255"`
	Phone    string `json:"phone"     binding:"omitempty,max=50"`
	Email    string `json:"email"     binding:"omitempty,email"`
}

// UpdateCustomerRequest is the validated body for PUT /customers/:id.
// All fields are optional.
type UpdateCustomerRequest struct {
	Name  string `json:"name"  binding:"omitempty,min=1,max=255"`
	Phone string `json:"phone" binding:"omitempty,max=50"`
	Email string `json:"email" binding:"omitempty,email"`
}

// CustomerResponse is the outbound representation of a customer.
type CustomerResponse struct {
	ID        int       `json:"id"`
	BranchID  int       `json:"branch_id"`
	Name      string    `json:"name"`
	Phone     *string   `json:"phone,omitempty"`
	Email     *string   `json:"email,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
