package service

import (
	"context"
	"fmt"

	"github.com/sajidannn/pos-api/internal/dto"
	"github.com/sajidannn/pos-api/internal/repository"
	"github.com/sajidannn/pos-api/internal/tenant"
)

// ReportService handles business logic for reports and balances.
type ReportService struct {
	repo repository.ReportRepository
}

// NewReportService returns a new ReportService.
func NewReportService(repo repository.ReportRepository) *ReportService {
	return &ReportService{repo: repo}
}

// GetBranchBalance retrieves and calculates the balance of a specific branch with optional filtering.
func (s *ReportService) GetBranchBalance(ctx context.Context, branchID int, filter dto.ReportFilter) (*dto.BranchBalanceResponse, error) {
	tenantID, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("ReportService.GetBranchBalance: %w", err)
	}

	bal, err := s.repo.GetBranchBalance(ctx, tenantID, branchID, filter)
	if err != nil {
		return nil, err
	}

	currentBalance := bal.OpeningBalance.Add(bal.TotalIn).Sub(bal.TotalOut)

	return &dto.BranchBalanceResponse{
		BranchID:       bal.BranchID,
		BranchName:     bal.BranchName,
		OpeningBalance: bal.OpeningBalance,
		TotalCashIn:    bal.TotalIn,
		TotalCashOut:   bal.TotalOut,
		CurrentBalance: currentBalance,
	}, nil
}

// GetTenantBalance retrieves and calculates the global balance of a tenant with optional filtering.
func (s *ReportService) GetTenantBalance(ctx context.Context, filter dto.ReportFilter) (*dto.TenantBalanceResponse, error) {
	tenantID, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("ReportService.GetTenantBalance: %w", err)
	}

	bal, err := s.repo.GetTenantBalance(ctx, tenantID, filter)
	if err != nil {
		return nil, err
	}

	netBalance := bal.TotalIn.Sub(bal.TotalOut)

	return &dto.TenantBalanceResponse{
		TenantID:     bal.TenantID,
		TotalCashIn:  bal.TotalIn,
		TotalCashOut: bal.TotalOut,
		NetBalance:   netBalance,
	}, nil
}

// InjectCapital adds a capital injection or withdrawal to the tenant's global balance.
func (s *ReportService) InjectCapital(ctx context.Context, req dto.CapitalRequest) error {
	tenantID, err := tenant.FromContext(ctx)
	if err != nil {
		return fmt.Errorf("ReportService.InjectCapital: %w", err)
	}
	return s.repo.InjectCapital(ctx, tenantID, req)
}

// GetSummary returns a dashboard summary of transactions for the current tenant.
func (s *ReportService) GetSummary(ctx context.Context, filter dto.SummaryFilter) (*dto.SummaryResponse, error) {
	tenantID, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("ReportService.GetSummary: %w", err)
	}

	summary, err := s.repo.GetSummary(ctx, tenantID, filter)
	if err != nil {
		return nil, err
	}

	grossProfit := summary.TotalSales.Sub(summary.TotalCOGS)

	return &dto.SummaryResponse{
		TotalSales:           summary.TotalSales,
		TotalPurchases:       summary.TotalPurchases,
		TotalReturns:         summary.TotalReturns,
		TotalPurchaseReturns: summary.TotalPurchaseReturns,
		TotalCOGS:            summary.TotalCOGS,
		GrossProfit:          grossProfit,
		TransactionCount:     summary.TransactionCount,
		ItemsSold:            summary.ItemsSold,
	}, nil
}

// GetTopItems returns the highest-performing items by quantity sold or revenue.
func (s *ReportService) GetTopItems(ctx context.Context, filter dto.ItemsFilter) ([]dto.ItemPerformanceResponse, error) {
	tenantID, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("ReportService.GetTopItems: %w", err)
	}
	return s.toItemPerformanceResponse(ctx, tenantID, filter, false)
}

// GetLowItems returns the lowest-performing items by quantity sold or revenue.
func (s *ReportService) GetLowItems(ctx context.Context, filter dto.ItemsFilter) ([]dto.ItemPerformanceResponse, error) {
	tenantID, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("ReportService.GetLowItems: %w", err)
	}
	return s.toItemPerformanceResponse(ctx, tenantID, filter, true)
}

func (s *ReportService) toItemPerformanceResponse(ctx context.Context, tenantID int, filter dto.ItemsFilter, ascending bool) ([]dto.ItemPerformanceResponse, error) {
	items, err := s.repo.GetItemPerformance(ctx, tenantID, filter, ascending)
	if err != nil {
		return nil, err
	}

	resp := make([]dto.ItemPerformanceResponse, len(items))
	for i, item := range items {
		resp[i] = dto.ItemPerformanceResponse{
			ItemID:       item.ItemID,
			ItemName:     item.ItemName,
			SKU:          item.SKU,
			TotalQtySold: item.TotalQtySold,
			TotalRevenue: item.TotalRevenue,
			TotalCOGS:    item.TotalCOGS,
			Profit:       item.TotalRevenue.Sub(item.TotalCOGS),
		}
	}
	return resp, nil
}

// GetSalesReport returns sales aggregated per day or month within a date range.
func (s *ReportService) GetSalesReport(ctx context.Context, filter dto.SalesReportFilter) ([]dto.SalesReportEntryResponse, error) {
	tenantID, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("ReportService.GetSalesReport: %w", err)
	}

	if filter.GroupBy == "" {
		filter.GroupBy = "day"
	}

	entries, err := s.repo.GetSalesReport(ctx, tenantID, filter)
	if err != nil {
		return nil, err
	}

	resp := make([]dto.SalesReportEntryResponse, len(entries))
	for i, e := range entries {
		resp[i] = dto.SalesReportEntryResponse{
			Date:              e.Date,
			TotalTransactions: e.TotalTransactions,
			TotalRevenue:      e.TotalRevenue,
			TotalCOGS:         e.TotalCOGS,
			GrossProfit:       e.TotalRevenue.Sub(e.TotalCOGS),
			TotalItemsSold:    e.TotalItemsSold,
		}
	}
	return resp, nil
}
