package repository

import (
	"context"

	"github.com/sajidannn/pos-api/internal/dto"
	"github.com/sajidannn/pos-api/internal/model"
	"github.com/shopspring/decimal"
)

// TransactionRepository is the data-access contract for all POS Transactions.
type TransactionRepository interface {
	// ExecuteSaleTx handles the DB transaction and coordination for SALES.
	// The processFn closure isolates all business logic (math calculations) to the caller.
	ExecuteSaleTx(
		ctx context.Context,
		tenantID int,
		userID int,
		req dto.CreateSaleRequest,
		processFn func(loadedItems map[int]model.ProcessSaleItem) (model.FinalSaleAggregate, error),
	) (*model.Transaction, error)

	// ExecutePurchaseTx handles the DB transaction and coordination for PURCHASES.
	ExecutePurchaseTx(
		ctx context.Context,
		tenantID int,
		userID int,
		req dto.CreatePurchaseRequest,
		processFn func(loadedItems map[int]model.ProcessPurchaseItem) (model.FinalPurchaseAggregate, error),
	) (*model.Transaction, error)

	ExecuteTransferTx(
		ctx context.Context,
		tenantID int,
		userID int,
		req dto.CreateTransferRequest,
		processFn func(loadedItems map[int]model.ProcessTransferItem) (model.FinalTransferAggregate, error),
	) (*model.Transaction, error)

	// ExecuteReturnTx handles the DB transaction and coordination for RETURNS.
	ExecuteReturnTx(
		ctx context.Context,
		tenantID int,
		userID int,
		req dto.CreateReturnRequest,
		processFn func(loadedItems map[int]model.ProcessReturnItem) (model.FinalReturnAggregate, error),
	) (*model.Transaction, error)

	// ExecuteAdjustmentTx performs a bulk stock adjustment.
	ExecuteAdjustmentTx(
		ctx context.Context,
		tenantID int,
		userID int,
		req dto.AdjustStockRequest,
		processFn func(currentStocks map[int]int) (map[int]int, error),
	) error

	// ExecutePurchaseReturnTx handles the DB transaction for returning goods to a supplier.
	ExecutePurchaseReturnTx(
		ctx context.Context,
		tenantID int,
		userID int,
		req dto.CreatePurchaseReturnRequest,
		processFn func(loadedItems map[int]model.ProcessPurchaseReturnItem) (model.FinalPurchaseReturnAggregate, error),
	) (*model.Transaction, error)

	// List returns a paginated, filtered list of transactions for the tenant.
	List(ctx context.Context, tenantID int, q dto.PageQuery, f dto.TransactionFilter) ([]model.Transaction, int, error)

	// GetByID fetches a single transaction with its details joined.
	GetByID(ctx context.Context, tenantID, id int) (*model.Transaction, error)

	// ExecuteVoidTx handles the DB transaction for voiding a transaction via closure.
	ExecuteVoidTx(
		ctx context.Context, 
		tenantID int, 
		userID int, 
		originalTrxID int, 
		reason string,
		processFn func(data model.ProcessVoidData) error,
	) (*model.Transaction, error)

	// GetTenantNetBalance returns the current net balance of the tenant from tenant_cashflow.
	// Used by the service layer to guard cash-out transactions (PURCHASE).
	GetTenantNetBalance(ctx context.Context, tenantID int) (decimal.Decimal, error)

	// GetBranchNetBalance returns the current net balance of a branch from branch_cashflow.
	// Used by the service layer to guard cash-out transactions (RETURN, REMIT).
	GetBranchNetBalance(ctx context.Context, tenantID, branchID int) (decimal.Decimal, error)

	// RemitBranchBalance atomically transfers amount from branch cashflow to tenant cashflow.
	// Models the real-world "setoran kas" (cash remittance) by a branch manager.
	RemitBranchBalance(ctx context.Context, tenantID, branchID int, req dto.RemitRequest) error
}
