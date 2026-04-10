package model

import "time"

// TransactionType enum mimics the DB constraint
type TransactionType string

const (
	TxSale     TransactionType = "SALE"
	TxPurchase TransactionType = "PURC"
	TxTransfer TransactionType = "TRANSFER"
	TxReturn   TransactionType = "RETURN"
)

// CashflowType enum mimics the DB cashflow_type constraint
type CashflowType string

const (
	CflowSale       CashflowType = "SALE"
	CflowTransfer   CashflowType = "TRANSFER"
	CflowAdjustment CashflowType = "ADJUSTMENT"
	CflowPurch      CashflowType = "PURC"
	CflowWithdraw   CashflowType = "WITHDRAW"
)

// Transaction represents the main transaction header.
type Transaction struct {
	ID          int             `json:"id"`
	TenantID    int             `json:"tenant_id,omitempty"` // populated in global db mode
	TrxNo       string          `json:"trxno"`
	BranchID    *int            `json:"branch_id,omitempty"`
	WarehouseID *int            `json:"warehouse_id,omitempty"`
	CustomerID  *int            `json:"customer_id,omitempty"`
	SupplierID  *int            `json:"supplier_id,omitempty"`
	UserID      *int            `json:"user_id,omitempty"`
	TransType   TransactionType `json:"trans_type"`
	TotalAmount float64         `json:"total_amount"`
	Tax         float64         `json:"tax"`
	Discount    float64         `json:"discount"`
	Note        string          `json:"note,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`

	// Joined attributes for nested struct responses
	Details []TransactionDetail `json:"details,omitempty"`
}

// TransactionDetail represents the items within a transaction.
type TransactionDetail struct {
	ID              int     `json:"id"`
	TransactionID   int     `json:"transaction_id"`
	BranchItemID    *int    `json:"branch_item_id,omitempty"`
	WarehouseItemID *int    `json:"warehouse_item_id,omitempty"`
	Quantity        int     `json:"quantity"`
	COGS            float64 `json:"cogs"`
	Price           float64 `json:"price"`
	Subtotal        float64 `json:"subtotal"`
}

// BranchCashflow represents cashflow movements inside a branch.
type BranchCashflow struct {
	ID            int          `json:"id"`
	TenantID      int          `json:"tenant_id,omitempty"`
	BranchID      int          `json:"branch_id"`
	TransactionID *int         `json:"transaction_id,omitempty"` // nullable for manual adjustments
	FlowType      CashflowType `json:"flow_type"`
	Direction     string       `json:"direction"` // IN, OUT
	Amount        float64      `json:"amount"`
	CreatedAt     time.Time    `json:"created_at"`
}

// TenantCashflow represents global tenant cash movements (e.g. paying supplier).
type TenantCashflow struct {
	ID            int          `json:"id"`
	TenantID      int          `json:"tenant_id,omitempty"`
	TransactionID *int         `json:"transaction_id,omitempty"`
	FlowType      CashflowType `json:"flow_type"`
	Direction     string       `json:"direction"` // IN, OUT
	Amount        float64      `json:"amount"`
	CreatedAt     time.Time    `json:"created_at"`
}

// ── Domain Types for Closure Pattern (Two-Phase Execution) ───────────────────

// ProcessSaleItem represents the read-only DB data fetched by the Repository
// and provided to the Service for calculation.
type ProcessSaleItem struct {
	BranchItemID int
	AvailableQty int
	COGS         float64
	Price        float64
}

// FinalSaleAggregate represents the pure business calculation result
// returned by the Service closure back to the Repository for execution.
type FinalSaleAggregate struct {
	TotalAmount float64
	Details     []TransactionDetail
}

// ProcessPurchaseItem represents the read-only DB data fetched by the Repository
// and provided to the Service for WAC calculation.
type ProcessPurchaseItem struct {
	ItemID       int
	GlobalStock  int     // Sum of stock from all branches and warehouses
	ExistingCost float64 // Current master cost in items table
}

// FinalPurchaseAggregate represents the business calculation result for a purchase
// returned by the Service closure back to the Repository for execution.
type FinalPurchaseAggregate struct {
	TotalAmount float64
	Details     []TransactionDetail
	NewCosts    map[int]float64 // Map of item_id -> new average cost
}

// ProcessTransferItem represents the read-only DB data fetched by the Repository
// for source and destination locations to validate stock and get COGS.
type ProcessTransferItem struct {
	ItemID        int
	SourceStock   int
	DestStock     int
	SourceItemLocID int // branch_item_id or warehouse_item_id of source
	DestItemLocID   int // branch_item_id or warehouse_item_id of dest
	ExistingCost  float64
}

// FinalTransferAggregate represents the business calculation result for a transfer.
type FinalTransferAggregate struct {
	SourceDetails []TransactionDetail
	DestDetails   []TransactionDetail
}
