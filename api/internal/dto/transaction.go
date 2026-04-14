package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

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
	Tax        decimal.Decimal   `json:"tax"         binding:"min=0"`
	Discount   decimal.Decimal   `json:"discount"    binding:"min=0"`
	Note       string            `json:"note"        binding:"omitempty,max=1000"`
	Items      []SaleItemRequest `json:"items"       binding:"required,min=1,dive"`
}

// PurchaseItemRequest defines a single item inside a purchase payload.
type PurchaseItemRequest struct {
	ItemID int             `json:"item_id" binding:"required"`
	Cost   decimal.Decimal `json:"cost"    binding:"required,min=0"`
	Qty    int             `json:"qty"     binding:"required,min=1"`
}

// CreatePurchaseRequest is the validated HTTP request body for POST /transactions/purchase.
type CreatePurchaseRequest struct {
	WarehouseID *int                  `json:"warehouse_id" binding:"required_without=BranchID"`
	BranchID    *int                  `json:"branch_id"    binding:"required_without=WarehouseID"`
	SupplierID  int                   `json:"supplier_id"  binding:"required"`
	Tax         decimal.Decimal       `json:"tax"          binding:"min=0"`
	Discount    decimal.Decimal       `json:"discount"     binding:"min=0"`
	Note        string                `json:"note"         binding:"omitempty,max=1000"`
	Items       []PurchaseItemRequest `json:"items"        binding:"required,min=1,dive"`
}

// TransferItemRequest defines a single item inside a transfer payload.
type TransferItemRequest struct {
	ItemID int `json:"item_id" binding:"required"`
	Qty    int `json:"qty"     binding:"required,min=1"`
}

// CreateTransferRequest is the validated HTTP request body for POST /transactions/transfer.
type CreateTransferRequest struct {
	SourceType string                `json:"source_type" binding:"required,oneof=branch warehouse"`
	SourceID   int                   `json:"source_id"   binding:"required"`
	DestType   string                `json:"dest_type"   binding:"required,oneof=branch warehouse"`
	DestID     int                   `json:"dest_id"     binding:"required"`
	Note      string                `json:"note"        binding:"omitempty,max=1000"`
	Items     []TransferItemRequest `json:"items"       binding:"required,min=1,dive"`
}

// ReturnItemRequest defines a single item in a return payload.
type ReturnItemRequest struct {
	BranchItemID int             `json:"branch_item_id" binding:"required"`
	Qty          int             `json:"qty"            binding:"required,min=1"`
	Price        decimal.Decimal `json:"price"          binding:"required,min=0"`
}

// CreateReturnRequest is for POST /transactions/return.
type CreateReturnRequest struct {
	OriginalTrxNo string              `json:"original_trx_no" binding:"required"`
	BranchID      int                 `json:"branch_id"       binding:"required"`
	CustomerID    *int                `json:"customer_id,omitempty"`
	Note          string              `json:"note"            binding:"omitempty,max=1000"`
	Items         []ReturnItemRequest `json:"items"           binding:"required,min=1,dive"`
}

// AdjustStockItemRequest defines one item change in a bulk adjustment.
type AdjustStockItemRequest struct {
	ItemID      int `json:"item_id"       binding:"required"`
	ActualStock int `json:"actual_stock"  binding:"required,min=0"`
}

// AdjustStockRequest handles bulk stock reconciliation.
type AdjustStockRequest struct {
	LocationType string                   `json:"location_type" binding:"required,oneof=branch warehouse"`
	LocationID   int                      `json:"location_id"   binding:"required"`
	Reason       string                   `json:"reason"        binding:"required,max=500"`
	Items        []AdjustStockItemRequest `json:"items"         binding:"required,min=1,dive"`
}

// VoidRequest is the validated HTTP request body for POST /transactions/:id/void.
type VoidRequest struct {
	Reason string `json:"reason" binding:"required,max=500"`
}

// ── Filter ────────────────────────────────────────────────────────────────────

// TransactionFilter holds query-string filters for GET /transactions.
type TransactionFilter struct {
	TransType   string `form:"trans_type"` // SALE, PURC, TRANSFER, RETURN
	BranchID    *int   `form:"branch_id"`
	WarehouseID *int   `form:"warehouse_id"`
	CustomerID  *int   `form:"customer_id"`
	SupplierID  *int   `form:"supplier_id"`
	Search      string `form:"search"` // partial match on trxno, note
	DateFrom    *time.Time
	DateTo      *time.Time
}

// ── Response ─────────────────────────────────────────────────────────────────

// TransactionItemResponse structures details inside the transacion response.
type TransactionItemResponse struct {
	BranchItemID    *int            `json:"branch_item_id,omitempty"`
	WarehouseItemID *int            `json:"warehouse_item_id,omitempty"`
	Quantity        int             `json:"quantity"`
	COGS            decimal.Decimal `json:"cogs"` // HPP captured at time of transaction
	Price           decimal.Decimal `json:"price"`
	Subtotal        decimal.Decimal `json:"subtotal"`
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
	Tax         decimal.Decimal           `json:"tax"`
	Discount    decimal.Decimal           `json:"discount"`
	TotalAmount decimal.Decimal           `json:"total_amount"` // Include tax and discount
	Note         string                    `json:"note,omitempty"`
	ReferenceTransactionID *int            `json:"reference_transaction_id,omitempty"`
	CreatedAt   time.Time                 `json:"created_at"`
	Details     []TransactionItemResponse `json:"details"`
}

// TransactionListResponse is a compact representation for list views (no details).
type TransactionListResponse struct {
	ID          int             `json:"id"`
	TrxNo       string          `json:"trxno"`
	TransType   string          `json:"trans_type"`
	BranchID    *int            `json:"branch_id,omitempty"`
	WarehouseID *int            `json:"warehouse_id,omitempty"`
	CustomerID  *int            `json:"customer_id,omitempty"`
	SupplierID  *int            `json:"supplier_id,omitempty"`
	TotalAmount decimal.Decimal `json:"total_amount"`
	CreatedAt   time.Time       `json:"created_at"`
}
