package repository

import (
	"context"

	"github.com/sajidannn/pos-api/internal/dto"
	"github.com/sajidannn/pos-api/internal/model"
)

// ReportRepository defines database operations for reporting and analytics.
type ReportRepository interface {
	// GetBranchBalance calculates the balance of a specific branch with optional filtering.
	GetBranchBalance(ctx context.Context, tenantID, branchID int, filter dto.ReportFilter) (*model.BranchBalance, error)

	// GetTenantBalance calculates the global balance of a tenant with optional filtering.
	GetTenantBalance(ctx context.Context, tenantID int, filter dto.ReportFilter) (*model.TenantBalance, error)

	// InjectCapital inserts a capital ADJUSTMENT record into tenant_cashflow.
	InjectCapital(ctx context.Context, tenantID int, req dto.CapitalRequest) error

	// GetSummary returns aggregated transaction metrics for a given period.
	GetSummary(ctx context.Context, tenantID int, filter dto.SummaryFilter) (*model.ReportSummary, error)

	// GetItemPerformance returns items ranked by sales quantity or revenue.
	// ascending=true returns the lowest-performing items (slow-movers).
	GetItemPerformance(ctx context.Context, tenantID int, filter dto.ItemsFilter, ascending bool) ([]model.ItemPerformance, error)

	// GetSalesReport returns sales grouped by day or month within the given period.
	GetSalesReport(ctx context.Context, tenantID int, filter dto.SalesReportFilter) ([]model.SalesReportEntry, error)
}
