package dto

import "time"

// ── Request ──────────────────────────────────────────────────────────────────

// CreateBranchRequest is the validated HTTP request body for POST /branches.
type CreateBranchRequest struct {
	Name           string  `json:"name"            binding:"required,min=1,max=255"`
	Phone          string  `json:"phone"           binding:"required,min=5,max=20"`
	Address        string  `json:"address"         binding:"required,min=1,max=500"`
	OpeningBalance float64 `json:"opening_balance" binding:"min=0"`
}

type UpdateBranchRequest struct {
	Name           string  `json:"name"            binding:"omitempty,min=1,max=255"`
	Phone          string  `json:"phone"           binding:"omitempty,min=5,max=20"`
	Address        string  `json:"address"         binding:"omitempty,min=1,max=500"`
	OpeningBalance float64 `json:"opening_balance" binding:"omitempty,min=0"`
}

// ── Response ─────────────────────────────────────────────────────────────────

// BranchResponse is the outbound representation of a branch.
type BranchResponse struct {
	ID             int       `json:"id"`
	Name           string    `json:"name"`
	Phone          string    `json:"phone"`
	Address        string    `json:"address"`
	OpeningBalance float64   `json:"opening_balance"`
	CreatedAt      time.Time `json:"created_at"`
}
