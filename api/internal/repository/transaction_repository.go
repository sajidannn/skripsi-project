package repository

import (
	"context"

	"github.com/sajidannn/pos-api/internal/dto"
	"github.com/sajidannn/pos-api/internal/model"
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
		req dto.AdjustStockRequest,
		processFn func(currentStocks map[int]int) (map[int]int, error),
	) error

	// List returns a paginated, filtered list of transactions for the tenant.
	List(ctx context.Context, tenantID int, q dto.PageQuery, f dto.TransactionFilter) ([]model.Transaction, int, error)

	// GetByID fetches a single transaction with its details joined.
	GetByID(ctx context.Context, tenantID, id int) (*model.Transaction, error)
}
