package dto

import "time"

// ── Request ──────────────────────────────────────────────────────────────────

// SaleItemRequest defines a single item inside a sale payload.
// Since price is protected and fetched from backend, we only need the ID and qty.
type SaleItemRequest struct {
	BranchItemID int `json:"branch_item_id" binding:"required"`
	Qty          int `json:"qty"            binding:"required,min=1"`
}

// CreateSaleRequest is the validated HTTP request body for POST /transactions/sale.
type CreateSaleRequest struct {
	BranchID   int               `json:"branch_id"   binding:"required"`
	CustomerID *int              `json:"customer_id" binding:"omitempty"` // Walk-in customers might not have an ID
	Tax        float64           `json:"tax"         binding:"min=0"`
	Discount   float64           `json:"discount"    binding:"min=0"`
	Note       string            `json:"note"        binding:"omitempty,max=1000"`
	Items      []SaleItemRequest `json:"items"       binding:"required,min=1,dive"`
}

// ── Response ─────────────────────────────────────────────────────────────────

// TransactionItemResponse structures details inside the transacion response.
type TransactionItemResponse struct {
	BranchItemID    *int    `json:"branch_item_id,omitempty"`
	WarehouseItemID *int    `json:"warehouse_item_id,omitempty"`
	Quantity        int     `json:"quantity"`
	Price           float64 `json:"price"`     // The price they actually bought it for
	Subtotal        float64 `json:"subtotal"`  // qty * price
}

// TransactionResponse is the generic outbound representation of a Transaction.
// Used for Sale, Purchase, Transfer, and Return responses uniformly.
type TransactionResponse struct {
	ID          int                       `json:"id"`
	TrxNo       string                    `json:"trxno"`
	TransType   string                    `json:"trans_type"`
	BranchID    *int                      `json:"branch_id,omitempty"`
	WarehouseID *int                      `json:"warehouse_id,omitempty"`
	CustomerID  *int                      `json:"customer_id,omitempty"`
	SupplierID  *int                      `json:"supplier_id,omitempty"`
	UserID      *int                      `json:"user_id,omitempty"`
	Tax         float64                   `json:"tax"`
	Discount    float64                   `json:"discount"`
	TotalAmount float64                   `json:"total_amount"` // Include tax and discount
	Note        string                    `json:"note,omitempty"`
	CreatedAt   time.Time                 `json:"created_at"`
	Details     []TransactionItemResponse `json:"details"`
}
