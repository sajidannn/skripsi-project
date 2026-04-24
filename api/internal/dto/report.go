package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

// ── Request ──────────────────────────────────────────────────────────────────

// ReportFilter contains optional date ranges for filtering reports.
type ReportFilter struct {
	DateFrom *time.Time `form:"date_from" time_format:"2006-01-02"`
	DateTo   *time.Time `form:"date_to"   time_format:"2006-01-02"`
}

// SummaryFilter extends ReportFilter with an optional branch scope.
type SummaryFilter struct {
	BranchID *int       `form:"branch_id"`
	DateFrom *time.Time `form:"date_from" time_format:"2006-01-02"`
	DateTo   *time.Time `form:"date_to"   time_format:"2006-01-02"`
}

// ItemsFilter extends SummaryFilter with sort and limit options for item rankings.
type ItemsFilter struct {
	BranchID *int       `form:"branch_id"`
	DateFrom *time.Time `form:"date_from" time_format:"2006-01-02"`
	DateTo   *time.Time `form:"date_to"   time_format:"2006-01-02"`
	SortBy   string     `form:"sort_by"   binding:"omitempty,oneof=qty revenue"`
	Limit    int        `form:"limit"`
}

// SalesReportFilter holds parameters for the periodic sales report.
// GroupBy accepts "day" or "month"; defaults to "day" if not set.
type SalesReportFilter struct {
	BranchID *int       `form:"branch_id"`
	DateFrom *time.Time `form:"date_from" time_format:"2006-01-02" binding:"required"`
	DateTo   *time.Time `form:"date_to"   time_format:"2006-01-02" binding:"required"`
	GroupBy  string     `form:"group_by"  binding:"omitempty,oneof=day month"`
}

// RemitRequest is the request body for POST /transactions/remit/branch/:id.
// It represents the manager setting aside branch cash up to the tenant.
type RemitRequest struct {
	Amount  decimal.Decimal `json:"amount"  binding:"required"`
	Note    string          `json:"note"    binding:"omitempty,max=500"`
}

// CapitalRequest is the validated HTTP request body for POST /reports/balance/tenant/capital.
type CapitalRequest struct {
	Direction string          `json:"direction" binding:"required,oneof=IN OUT"`
	Amount    decimal.Decimal `json:"amount"    binding:"required,min=1"`
	Note      string          `json:"note"      binding:"omitempty,max=1000"`
}

// ── Response ─────────────────────────────────────────────────────────────────

// BranchBalanceResponse represents the calculated balance of a specific branch.
type BranchBalanceResponse struct {
	BranchID       int             `json:"branch_id"`
	BranchName     string          `json:"branch_name"`
	OpeningBalance decimal.Decimal `json:"opening_balance"`
	TotalCashIn    decimal.Decimal `json:"total_cash_in"`
	TotalCashOut   decimal.Decimal `json:"total_cash_out"`
	CurrentBalance decimal.Decimal `json:"current_balance"`
}

// TenantBalanceResponse represents the calculated total balance of a tenant.
type TenantBalanceResponse struct {
	TenantID     int             `json:"tenant_id,omitempty"` // populated in global db mode
	TotalCashIn  decimal.Decimal `json:"total_cash_in"`
	TotalCashOut decimal.Decimal `json:"total_cash_out"`
	NetBalance   decimal.Decimal `json:"net_balance"`
}

// SummaryResponse represents the dashboard summary of transactions.
type SummaryResponse struct {
	TotalSales           decimal.Decimal `json:"total_sales"`
	TotalPurchases       decimal.Decimal `json:"total_purchases"`
	TotalReturns         decimal.Decimal `json:"total_returns"`
	TotalPurchaseReturns decimal.Decimal `json:"total_purchase_returns"`
	TotalCOGS            decimal.Decimal `json:"total_cogs"`
	GrossProfit          decimal.Decimal `json:"gross_profit"`
	TransactionCount     int             `json:"transaction_count"`
	ItemsSold            int             `json:"items_sold"`
}

// ItemPerformanceResponse represents one item in the top/low items report.
type ItemPerformanceResponse struct {
	ItemID       int             `json:"item_id"`
	ItemName     string          `json:"item_name"`
	SKU          string          `json:"sku"`
	TotalQtySold int             `json:"total_qty_sold"`
	TotalRevenue decimal.Decimal `json:"total_revenue"`
	TotalCOGS    decimal.Decimal `json:"total_cogs"`
	Profit       decimal.Decimal `json:"profit"`
}

// SalesReportEntryResponse represents one time-bucket row in the sales report.
type SalesReportEntryResponse struct {
	Date              string          `json:"date"`
	TotalTransactions int             `json:"total_transactions"`
	TotalRevenue      decimal.Decimal `json:"total_revenue"`
	TotalCOGS         decimal.Decimal `json:"total_cogs"`
	GrossProfit       decimal.Decimal `json:"gross_profit"`
	TotalItemsSold    int             `json:"total_items_sold"`
}
