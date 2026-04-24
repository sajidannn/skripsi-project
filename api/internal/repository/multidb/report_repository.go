package multidb

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/sajidannn/pos-api/internal/apierr"
	"github.com/sajidannn/pos-api/internal/db/multidb"
	"github.com/sajidannn/pos-api/internal/dto"
	"github.com/sajidannn/pos-api/internal/model"
)

// ReportRepo implements repository.ReportRepository for multi-DB mode.
type ReportRepo struct {
	mgr *multidb.Manager
}

// NewReportRepo creates a new ReportRepo backed by the tenant Manager.
func NewReportRepo(mgr *multidb.Manager) *ReportRepo {
	return &ReportRepo{mgr: mgr}
}

// GetBranchBalance calculates the balance of a specific branch with optional filtering.
func (r *ReportRepo) GetBranchBalance(ctx context.Context, tenantID, branchID int, filter dto.ReportFilter) (*model.BranchBalance, error) {
	pool, err := r.mgr.Pool(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	args := []any{branchID}
	where := ""
	if filter.DateFrom != nil {
		args = append(args, *filter.DateFrom)
		where += fmt.Sprintf(" AND bc.created_at >= $%d", len(args))
	}
	if filter.DateTo != nil {
		args = append(args, *filter.DateTo)
		where += fmt.Sprintf(" AND bc.created_at <= $%d", len(args))
	}

	query := fmt.Sprintf(`
		SELECT b.id, b.name, b.opening_balance,
		       COALESCE(SUM(CASE WHEN bc.direction = 'IN' THEN bc.amount ELSE 0 END), 0) AS total_in,
		       COALESCE(SUM(CASE WHEN bc.direction = 'OUT' THEN bc.amount ELSE 0 END), 0) AS total_out
		FROM branches b
		LEFT JOIN branch_cashflow bc ON bc.branch_id = b.id %s
		WHERE b.id = $1
		GROUP BY b.id
	`, where)

	var bal model.BranchBalance
	err = pool.QueryRow(ctx, query, args...).Scan(
		&bal.BranchID, &bal.BranchName, &bal.OpeningBalance, &bal.TotalIn, &bal.TotalOut,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apierr.NotFound(fmt.Sprintf("Branch ID %d not found", branchID))
		}
		return nil, fmt.Errorf("multidb.ReportRepo.GetBranchBalance: %w", err)
	}

	return &bal, nil
}

// GetTenantBalance calculates the global balance of a tenant with optional filtering.
func (r *ReportRepo) GetTenantBalance(ctx context.Context, tenantID int, filter dto.ReportFilter) (*model.TenantBalance, error) {
	pool, err := r.mgr.Pool(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	args := []any{}
	where := ""
	if filter.DateFrom != nil {
		args = append(args, *filter.DateFrom)
		where += fmt.Sprintf(" WHERE created_at >= $%d", len(args))
	}
	if filter.DateTo != nil {
		args = append(args, *filter.DateTo)
		if where == "" {
			where += fmt.Sprintf(" WHERE created_at <= $%d", len(args))
		} else {
			where += fmt.Sprintf(" AND created_at <= $%d", len(args))
		}
	}

	query := fmt.Sprintf(`
		SELECT
		    COALESCE(SUM(CASE WHEN direction = 'IN'  THEN amount ELSE 0 END), 0) AS total_in,
		    COALESCE(SUM(CASE WHEN direction = 'OUT' THEN amount ELSE 0 END), 0) AS total_out
		FROM tenant_cashflow
		%s
	`, where)

	var bal model.TenantBalance
	err = pool.QueryRow(ctx, query, args...).Scan(
		&bal.TotalIn, &bal.TotalOut,
	)

	if err != nil {
		return nil, fmt.Errorf("multidb.ReportRepo.GetTenantBalance: %w", err)
	}

	bal.TenantID = tenantID
	return &bal, nil
}

// InjectCapital inserts a capital ADJUSTMENT record into tenant_cashflow.
func (r *ReportRepo) InjectCapital(ctx context.Context, tenantID int, req dto.CapitalRequest) error {
	pool, err := r.mgr.Pool(ctx, tenantID)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO tenant_cashflow (flow_type, direction, amount)
		VALUES ('ADJUSTMENT', $1, $2)
	`
	_, err = pool.Exec(ctx, query, req.Direction, req.Amount)
	if err != nil {
		return fmt.Errorf("multidb.ReportRepo.InjectCapital: %w", err)
	}
	return nil
}

// GetSummary returns aggregated transaction metrics for a given period.
func (r *ReportRepo) GetSummary(ctx context.Context, tenantID int, filter dto.SummaryFilter) (*model.ReportSummary, error) {
	pool, err := r.mgr.Pool(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	args := []any{}
	where := ""

	if filter.BranchID != nil {
		args = append(args, *filter.BranchID)
		where += fmt.Sprintf(" AND t.branch_id = $%d", len(args))
	}
	if filter.DateFrom != nil {
		args = append(args, *filter.DateFrom)
		where += fmt.Sprintf(" AND t.created_at >= $%d", len(args))
	}
	if filter.DateTo != nil {
		args = append(args, *filter.DateTo)
		where += fmt.Sprintf(" AND t.created_at <= $%d", len(args))
	}

	// Construct WHERE clause — add leading WHERE if any filter is present, else omit
	whereClause := ""
	if where != "" {
		whereClause = "WHERE 1=1" + where
	}

	query := fmt.Sprintf(`
		SELECT
		    COALESCE(SUM(CASE WHEN t.trans_type = 'SALE'        THEN t.total_amount ELSE 0 END), 0) AS total_sales,
		    COALESCE(SUM(CASE WHEN t.trans_type = 'PURC'        THEN t.total_amount ELSE 0 END), 0) AS total_purchases,
		    COALESCE(SUM(CASE WHEN t.trans_type = 'RETURN'      THEN t.total_amount ELSE 0 END), 0) AS total_returns,
		    COALESCE(SUM(CASE WHEN t.trans_type = 'RETURN_PURC' THEN t.total_amount ELSE 0 END), 0) AS total_purchase_returns,
		    COALESCE(SUM(CASE WHEN t.trans_type = 'SALE'        THEN td.cogs * td.quantity ELSE 0 END), 0) AS total_cogs,
		    COUNT(DISTINCT t.id) AS transaction_count,
		    COALESCE(SUM(CASE WHEN t.trans_type = 'SALE'        THEN td.quantity ELSE 0 END), 0) AS items_sold
		FROM transactions t
		LEFT JOIN transaction_detail td ON td.transaction_id = t.id
		%s
	`, whereClause)

	var s model.ReportSummary
	err = pool.QueryRow(ctx, query, args...).Scan(
		&s.TotalSales, &s.TotalPurchases, &s.TotalReturns, &s.TotalPurchaseReturns,
		&s.TotalCOGS, &s.TransactionCount, &s.ItemsSold,
	)
	if err != nil {
		return nil, fmt.Errorf("multidb.ReportRepo.GetSummary: %w", err)
	}
	return &s, nil
}

// GetItemPerformance returns items ranked by sales quantity or revenue.
// ascending=true returns the lowest-performing items (slow-movers).
func (r *ReportRepo) GetItemPerformance(ctx context.Context, tenantID int, filter dto.ItemsFilter, ascending bool) ([]model.ItemPerformance, error) {
	pool, err := r.mgr.Pool(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	args := []any{}
	where := ""

	if filter.BranchID != nil {
		args = append(args, *filter.BranchID)
		where += fmt.Sprintf(" AND t.branch_id = $%d", len(args))
	}
	if filter.DateFrom != nil {
		args = append(args, *filter.DateFrom)
		where += fmt.Sprintf(" AND t.created_at >= $%d", len(args))
	}
	if filter.DateTo != nil {
		args = append(args, *filter.DateTo)
		where += fmt.Sprintf(" AND t.created_at <= $%d", len(args))
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 10
	}

	sortCol := "total_qty_sold"
	if filter.SortBy == "revenue" {
		sortCol = "total_revenue"
	}

	order := "DESC"
	if ascending {
		order = "ASC"
	}

	args = append(args, limit)
	limitIdx := len(args)

	query := fmt.Sprintf(`
		SELECT it.id, it.name, it.sku,
		       COALESCE(SUM(td.quantity), 0)          AS total_qty_sold,
		       COALESCE(SUM(td.subtotal), 0)           AS total_revenue,
		       COALESCE(SUM(td.cogs * td.quantity), 0) AS total_cogs
		FROM transaction_detail td
		JOIN branch_items bi ON bi.id = td.branch_item_id
		JOIN items it ON it.id = bi.item_id
		JOIN transactions t ON t.id = td.transaction_id
		WHERE t.trans_type = 'SALE' %s
		GROUP BY it.id, it.name, it.sku
		ORDER BY %s %s
		LIMIT $%d
	`, where, sortCol, order, limitIdx)

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("multidb.ReportRepo.GetItemPerformance: %w", err)
	}
	defer rows.Close()

	var list []model.ItemPerformance
	for rows.Next() {
		var item model.ItemPerformance
		if err := rows.Scan(&item.ItemID, &item.ItemName, &item.SKU,
			&item.TotalQtySold, &item.TotalRevenue, &item.TotalCOGS); err != nil {
			return nil, fmt.Errorf("multidb.ReportRepo.GetItemPerformance scan: %w", err)
		}
		list = append(list, item)
	}
	return list, rows.Err()
}

// GetSalesReport returns sales grouped by day or month within the given period.
func (r *ReportRepo) GetSalesReport(ctx context.Context, tenantID int, filter dto.SalesReportFilter) ([]model.SalesReportEntry, error) {
	pool, err := r.mgr.Pool(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	args := []any{*filter.DateFrom, *filter.DateTo}
	where := ""

	if filter.BranchID != nil {
		args = append(args, *filter.BranchID)
		where += fmt.Sprintf(" AND t.branch_id = $%d", len(args))
	}

	trunc := "day"
	if filter.GroupBy == "month" {
		trunc = "month"
	}

	dateFmt := "YYYY-MM-DD"
	if trunc == "month" {
		dateFmt = "YYYY-MM"
	}

	query := fmt.Sprintf(`
		SELECT
		    TO_CHAR(DATE_TRUNC('%s', t.created_at), '%s')        AS date,
		    COUNT(DISTINCT t.id)                                  AS total_transactions,
		    COALESCE(SUM(td.subtotal), 0)                        AS total_revenue,
		    COALESCE(SUM(td.cogs * td.quantity), 0)              AS total_cogs,
		    COALESCE(SUM(td.quantity), 0)                        AS total_items_sold
		FROM transactions t
		LEFT JOIN transaction_detail td ON td.transaction_id = t.id
		WHERE t.trans_type = 'SALE'
		  AND t.created_at >= $1
		  AND t.created_at <= $2
		  %s
		GROUP BY DATE_TRUNC('%s', t.created_at)
		ORDER BY DATE_TRUNC('%s', t.created_at) ASC
	`, trunc, dateFmt, where, trunc, trunc)

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("multidb.ReportRepo.GetSalesReport: %w", err)
	}
	defer rows.Close()

	var list []model.SalesReportEntry
	for rows.Next() {
		var entry model.SalesReportEntry
		if err := rows.Scan(&entry.Date, &entry.TotalTransactions,
			&entry.TotalRevenue, &entry.TotalCOGS, &entry.TotalItemsSold); err != nil {
			return nil, fmt.Errorf("multidb.ReportRepo.GetSalesReport scan: %w", err)
		}
		list = append(list, entry)
	}
	return list, rows.Err()
}
