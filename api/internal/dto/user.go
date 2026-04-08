package dto

import "time"

// ── Filter ────────────────────────────────────────────────────────────────────

// UserFilter holds optional query-string filters for GET /users.
type UserFilter struct {
	Search   string `form:"search"` // case-insensitive partial match on name or email
	Role     string `form:"role"`
	DateFrom *time.Time
	DateTo   *time.Time
}

// ── Auth ──────────────────────────────────────────────────────────────────────

// LoginRequest is the body for POST /auth/login.
type LoginRequest struct {
	TenantID int    `json:"tenant_id" binding:"required"`
	Email    string `json:"email"     binding:"required,email"`
	Password string `json:"password"  binding:"required,min=6"`
}

// LoginResponse is returned on a successful login.
type LoginResponse struct {
	Token string `json:"token"`
}

// ── Request ───────────────────────────────────────────────────────────────────

// CreateUserRequest is the validated body for POST /users.
// Role must be one of: owner, manager, cashier.
type CreateUserRequest struct {
	Name     string `json:"name"     binding:"required,min=1,max=255"`
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required,min=6,max=72"`
	Role     string `json:"role"     binding:"required,oneof=owner manager cashier"`
}

// UpdateUserRequest is the validated body for PUT /users/:id.
// All fields are optional; only non-empty values are applied.
type UpdateUserRequest struct {
	Name string `json:"name" binding:"omitempty,min=1,max=255"`
	Role string `json:"role" binding:"omitempty,oneof=owner manager cashier"`
}

// ── Response ──────────────────────────────────────────────────────────────────

// UserResponse is the outbound representation of a user. Password is never included.
type UserResponse struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}
