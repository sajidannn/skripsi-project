package model

import "github.com/shopspring/decimal"

// BranchBalance represents the aggregated balance of a branch.
type BranchBalance struct {
	BranchID       int
	BranchName     string
	OpeningBalance decimal.Decimal
	TotalIn        decimal.Decimal
	TotalOut       decimal.Decimal
}

// TenantBalance represents the aggregated balance of a tenant.
type TenantBalance struct {
	TenantID int
	TotalIn  decimal.Decimal
	TotalOut decimal.Decimal
}

// ReportSummary holds aggregated transaction metrics for the summary report.
type ReportSummary struct {
	TotalSales           decimal.Decimal
	TotalPurchases       decimal.Decimal
	TotalReturns         decimal.Decimal
	TotalPurchaseReturns decimal.Decimal
	TotalCOGS            decimal.Decimal
	TransactionCount     int
	ItemsSold            int
}

// ItemPerformance represents a single item's aggregated sales data.
type ItemPerformance struct {
	ItemID       int
	ItemName     string
	SKU          string
	TotalQtySold int
	TotalRevenue decimal.Decimal
	TotalCOGS    decimal.Decimal
}

// SalesReportEntry represents one time-bucket (day/month) in the sales report.
type SalesReportEntry struct {
	Date              string
	TotalTransactions int
	TotalRevenue      decimal.Decimal
	TotalCOGS         decimal.Decimal
	TotalItemsSold    int
}
