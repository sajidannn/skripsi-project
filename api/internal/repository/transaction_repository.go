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
}
