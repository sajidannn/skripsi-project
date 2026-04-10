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

	// ExecuteTransferTx handles the DB transaction and coordination for TRANSFERS.
	ExecuteTransferTx(
		ctx context.Context,
		tenantID int,
		userID int,
		req dto.CreateTransferRequest,
		processFn func(loadedItems map[int]model.ProcessTransferItem) (model.FinalTransferAggregate, error),
	) (*model.Transaction, error)
}
