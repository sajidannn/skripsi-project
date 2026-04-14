package service

import (
	"context"
	"fmt"
	"strings"

	"errors"

	"github.com/sajidannn/pos-api/internal/apierr"
	"github.com/sajidannn/pos-api/internal/dto"
	"github.com/sajidannn/pos-api/internal/model"
	"github.com/sajidannn/pos-api/internal/repository"
	"github.com/sajidannn/pos-api/internal/tenant"
	"github.com/shopspring/decimal"
)

// TransactionService handles business logic for all POS transactions.
type TransactionService struct {
	repo repository.TransactionRepository
}

// NewTransactionService constructs a TransactionService.
func NewTransactionService(repo repository.TransactionRepository) *TransactionService {
	return &TransactionService{repo: repo}
}

// CreateSale delegates the Sale creation logic to the repository using a Closure.
func (s *TransactionService) CreateSale(ctx context.Context, userID int, req dto.CreateSaleRequest) (*model.Transaction, error) {
	tenantID, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("TransactionService.CreateSale: %w", err)
	}

	// Definition of Business Rule / Calculation Closure.
	// This function receives raw data from DB and returns pure Final Output without knowing about Postgres.
	calculateSaleFn := func(loadedDbItems map[int]model.ProcessSaleItem) (model.FinalSaleAggregate, error) {
		var trxTotalAmount decimal.Decimal
		var finalDetails []model.TransactionDetail

		// 1. Process Mathematical Validations & Subtotals
		for _, reqItem := range req.Items {
			dbItem, exists := loadedDbItems[reqItem.BranchItemID]
			if !exists {
				return model.FinalSaleAggregate{}, apierr.NotFound(fmt.Sprintf("item with id %d not found in loaded data", reqItem.BranchItemID))
			}

			// Validate Stock domain rule
			if dbItem.AvailableQty < reqItem.Qty {
				return model.FinalSaleAggregate{}, apierr.BadRequest(fmt.Sprintf("insufficient stock for item %d: available %d, requested %d", dbItem.BranchItemID, dbItem.AvailableQty, reqItem.Qty))
			}

			// Calculate Subtotal
			subtotal := decimal.NewFromInt(int64(reqItem.Qty)).Mul(dbItem.Price)
			trxTotalAmount = trxTotalAmount.Add(subtotal)

			// Store Detail for saving
			branchItemIDProxy := reqItem.BranchItemID
			finalDetails = append(finalDetails, model.TransactionDetail{
				BranchItemID: &branchItemIDProxy,
				Quantity:     reqItem.Qty,
				COGS:         dbItem.COGS,
				Price:        dbItem.Price,
				Subtotal:     subtotal,
			})
		}

		// 2. Finalize Master Level Calculations
		trxTotalAmount = trxTotalAmount.Add(req.Tax).Sub(req.Discount)

		// Optional Business Rule: Prevent total from going negative
		if trxTotalAmount.IsNegative() {
			trxTotalAmount = decimal.Zero
		}

		// Optional Business Rule: Discount cannot be more than 50% of the raw subtotal sum
		// (Example of logic that easily belongs in Service)
		// rawSubtotal := decimal.Zero
		// for _, d := range finalDetails { rawSubtotal = rawSubtotal.Add(d.Subtotal) }
		// if req.Discount.GreaterThan(rawSubtotal.Mul(decimal.NewFromFloat(0.5))) { return ... error }

		return model.FinalSaleAggregate{
			TotalAmount: trxTotalAmount,
			Details:     finalDetails,
		}, nil
	}

	// 3. Delegate execution to Repository using the defined logic
	res, err := s.repo.ExecuteSaleTx(ctx, tenantID, userID, req, calculateSaleFn)
	if err != nil {
		// If it's already an AppError (e.g. from closure), return as is
		var appErr *apierr.AppError
		if errors.As(err, &appErr) {
			return nil, appErr
		}
		// Otherwise, treat repo validation errors as NotFound/BadRequest
		return nil, apierr.NotFound(err.Error())
	}
	return res, nil
}

// CreatePurchase handles the business logic for purchasing items from suppliers.
func (s *TransactionService) CreatePurchase(ctx context.Context, userID int, req dto.CreatePurchaseRequest) (*model.Transaction, error) {
	tenantID, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, apierr.Internal(err, "failed to resolve tenant")
	}

	// Validation: branch_id and warehouse_id are mutually exclusive
	if req.BranchID != nil && req.WarehouseID != nil {
		return nil, apierr.BadRequest("destination must be either branch_id or warehouse_id, not both")
	}

	// Closure for Weighted Average Cost calculation
	calculatePurchaseFn := func(loadedDbItems map[int]model.ProcessPurchaseItem) (model.FinalPurchaseAggregate, error) {
		var trxTotalAmount decimal.Decimal
		var finalDetails []model.TransactionDetail
		newCosts := make(map[int]decimal.Decimal)

		for _, reqItem := range req.Items {
			dbItem, exists := loadedDbItems[reqItem.ItemID]
			if !exists {
				return model.FinalPurchaseAggregate{}, apierr.NotFound(fmt.Sprintf("item %d not found", reqItem.ItemID))
			}

			// 1. Logic Weighted Average Cost (WAC)
			qtyOld := decimal.NewFromInt(int64(dbItem.GlobalStock))
			totalCostOld := qtyOld.Mul(dbItem.ExistingCost)

			qtyNew := decimal.NewFromInt(int64(reqItem.Qty))
			totalCostNew := qtyNew.Mul(reqItem.Cost)

			newWAC := totalCostOld.Add(totalCostNew).Div(qtyOld.Add(qtyNew))
			newCosts[reqItem.ItemID] = newWAC

			// 2. Subtotal and Details
			subtotal := qtyNew.Mul(reqItem.Cost)
			trxTotalAmount = trxTotalAmount.Add(subtotal)

			detail := model.TransactionDetail{
				Quantity: reqItem.Qty,
				COGS:     newWAC, // New average cost becomes the reference COGS
				Price:    reqItem.Cost,
				Subtotal: subtotal,
			}

			// Map item_id to the pointer so repo can resolve locID
			itemIDProxy := reqItem.ItemID
			if req.BranchID != nil {
				detail.BranchItemID = &itemIDProxy
			} else {
				detail.WarehouseItemID = &itemIDProxy
			}

			finalDetails = append(finalDetails, detail)
		}

		// 3. Finalize Master Level Calculations
		trxTotalAmount = trxTotalAmount.Add(req.Tax).Sub(req.Discount)
		if trxTotalAmount.IsNegative() {
			trxTotalAmount = decimal.Zero
		}

		return model.FinalPurchaseAggregate{
			TotalAmount: trxTotalAmount,
			Details:     finalDetails,
			NewCosts:    newCosts,
		}, nil
	}

	res, err := s.repo.ExecutePurchaseTx(ctx, tenantID, userID, req, calculatePurchaseFn)
	if err != nil {
		var appErr *apierr.AppError
		if errors.As(err, &appErr) {
			return nil, appErr
		}
		return nil, apierr.NotFound(err.Error())
	}
	return res, nil
}

// CreateTransfer handles the business logic for moving stock between locations.
func (s *TransactionService) CreateTransfer(ctx context.Context, userID int, req dto.CreateTransferRequest) (*model.Transaction, error) {
	tenantID, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, apierr.Internal(err, "failed to resolve tenant")
	}

	// 1. Validation: Prevent same source and destination
	if req.SourceType == req.DestType && req.SourceID == req.DestID {
		return nil, apierr.BadRequest("source and destination cannot be the same")
	}

	// 2. Closure for Stock Mutation
	calculateTransferFn := func(loadedItems map[int]model.ProcessTransferItem) (model.FinalTransferAggregate, error) {
		var sourceDetails []model.TransactionDetail
		var destDetails []model.TransactionDetail

		for _, reqItem := range req.Items {
			dbItem, exists := loadedItems[reqItem.ItemID]
			if !exists {
				return model.FinalTransferAggregate{}, apierr.NotFound(fmt.Sprintf("item %d not found", reqItem.ItemID))
			}

			// Domain Rule: Validate source availability
			if dbItem.SourceStock < reqItem.Qty {
				return model.FinalTransferAggregate{}, apierr.BadRequest(fmt.Sprintf("insufficient stock for item %d at source: available %d, requested %d", reqItem.ItemID, dbItem.SourceStock, reqItem.Qty))
			}

			// Value of transfer inherited from master cost
			valuation := dbItem.ExistingCost
			subtotal := decimal.NewFromInt(int64(reqItem.Qty)).Mul(valuation)

			// Prepare TRFO (Transfer Out)
			outDetail := model.TransactionDetail{
				Quantity: reqItem.Qty,
				COGS:     valuation,
				Price:    valuation,
				Subtotal: subtotal,
			}
			if req.SourceType == "branch" {
				outDetail.BranchItemID = &dbItem.SourceItemLocID
			} else {
				outDetail.WarehouseItemID = &dbItem.SourceItemLocID
			}
			sourceDetails = append(sourceDetails, outDetail)

			// Prepare TRFI (Transfer In)
			inDetail := model.TransactionDetail{
				Quantity: reqItem.Qty,
				COGS:     valuation,
				Price:    valuation,
				Subtotal: subtotal,
			}
			if req.DestType == "branch" {
				inDetail.BranchItemID = &dbItem.DestItemLocID
			} else {
				inDetail.WarehouseItemID = &dbItem.DestItemLocID
			}
			destDetails = append(destDetails, inDetail)
		}

		return model.FinalTransferAggregate{
			SourceDetails: sourceDetails,
			DestDetails:   destDetails,
		}, nil
	}

	res, err := s.repo.ExecuteTransferTx(ctx, tenantID, userID, req, calculateTransferFn)
	if err != nil {
		var appErr *apierr.AppError
		if errors.As(err, &appErr) {
			return nil, appErr
		}
		return nil, apierr.NotFound(err.Error())
	}
	return res, nil
}

// CreateReturn handles the business logic for customer returns to the branch.
func (s *TransactionService) CreateReturn(ctx context.Context, userID int, req dto.CreateReturnRequest) (*model.Transaction, error) {
	tenantID, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, apierr.Internal(err, "failed to resolve tenant")
	}

	calculateReturnFn := func(loadedItems map[int]model.ProcessReturnItem) (model.FinalReturnAggregate, error) {
		var trxTotalAmount decimal.Decimal
		var finalDetails []model.TransactionDetail

		for _, reqItem := range req.Items {
			_, exists := loadedItems[reqItem.BranchItemID]
			if !exists {
				return model.FinalReturnAggregate{}, apierr.NotFound(fmt.Sprintf("item %d not found at branch", reqItem.BranchItemID))
			}

			// Subtotal logic: Quantity to return * refund price
			subtotal := decimal.NewFromInt(int64(reqItem.Qty)).Mul(reqItem.Price)
			trxTotalAmount = trxTotalAmount.Add(subtotal)

			branchItemIDProxy := reqItem.BranchItemID
			finalDetails = append(finalDetails, model.TransactionDetail{
				BranchItemID: &branchItemIDProxy,
				Quantity:     reqItem.Qty,
				COGS:         decimal.Zero, // Retur tidak mengubah HPP (WAC) master, hanya nambah stok
				Price:        reqItem.Price,
				Subtotal:     subtotal,
			})
		}

		return model.FinalReturnAggregate{
			TotalAmount: trxTotalAmount,
			Details:     finalDetails,
		}, nil
	}

	res, err := s.repo.ExecuteReturnTx(ctx, tenantID, userID, req, calculateReturnFn)
	if err != nil {
		var appErr *apierr.AppError
		if errors.As(err, &appErr) {
			return nil, appErr
		}
		return nil, apierr.NotFound(err.Error())
	}
	return res, nil
}

// AdjustStock handles bulk inventory reconciliation.
func (s *TransactionService) AdjustStock(ctx context.Context, req dto.AdjustStockRequest) error {
	tenantID, err := tenant.FromContext(ctx)
	if err != nil {
		return apierr.Internal(err, "failed to resolve tenant")
	}

	processAdjustmentFn := func(currentStocks map[int]int) (map[int]int, error) {
		diffs := make(map[int]int)

		for _, reqItem := range req.Items {
			// If not found in currentStocks, we assume it's currently 0
			current := currentStocks[reqItem.ItemID]
			diff := reqItem.ActualStock - current
			diffs[reqItem.ItemID] = diff
		}

		return diffs, nil
	}

	err = s.repo.ExecuteAdjustmentTx(ctx, tenantID, req, processAdjustmentFn)
	if err != nil {
		var appErr *apierr.AppError
		if errors.As(err, &appErr) {
			return appErr
		}
		return apierr.BadRequest(err.Error())
	}
	return nil
}

// List returns a paginated, filtered list of transactions.
func (s *TransactionService) List(ctx context.Context, q dto.PageQuery, f dto.TransactionFilter) ([]model.Transaction, int, error) {
	tenantID, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("TransactionService.List: %w", err)
	}
	return s.repo.List(ctx, tenantID, q, f)
}

// GetByID retrieves a single transaction with details.
func (s *TransactionService) GetByID(ctx context.Context, id int) (*model.Transaction, error) {
	tenantID, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("TransactionService.GetByID: %w", err)
	}
	return s.repo.GetByID(ctx, tenantID, id)
}

// Void handles the business logic and reversal activation for a transaction using the closure pattern.
func (s *TransactionService) Void(ctx context.Context, userID int, id int, req dto.VoidRequest) (*model.Transaction, error) {
	tenantID, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, apierr.Internal(err, "failed to resolve tenant context")
	}

	// Delegate execution to Repo, passing the closure for business rule validation.
	// The atomicity and fetching is completely handled by the repo.
	return s.repo.ExecuteVoidTx(ctx, tenantID, userID, id, req.Reason, func(data model.ProcessVoidData) error {
		// BL Rule: Cannot void PURCHASE (WAC concerns)
		if data.OriginalHeader.TransType == model.TxPurchase {
			return apierr.BadRequest("VOID PURCHASE is forbidden to maintain WAC integrity. Use Purchase Return instead.")
		}

		// BL Rule: Cannot void a VOID transaction (Double negative prevention)
		if data.OriginalHeader.TransType == model.TxVoid {
			return apierr.BadRequest("cannot void a transaction that is already a VOID type")
		}

		// BL Rule: Cannot void if already voided
		if data.AlreadyVoided {
			return apierr.BadRequest("this transaction has already been voided")
		}

		// BL Rule: Prevent negative stock during void
		for _, d := range data.Details {
			// TRFI and RETURN take stock OUT during void. We must ensure sufficient stock.
			isTRFI := data.OriginalHeader.TransType == model.TxTransfer && strings.Contains(data.OriginalHeader.TrxNo, "TRFI")
			isReturn := data.OriginalHeader.TransType == model.TxReturn
			
			if isTRFI || isReturn {
				if d.CurrentStock < d.Detail.Quantity {
					itemID := 0
					if d.Detail.BranchItemID != nil {
						itemID = *d.Detail.BranchItemID
					} else if d.Detail.WarehouseItemID != nil {
						itemID = *d.Detail.WarehouseItemID
					}
					return apierr.BadRequest(fmt.Sprintf("cannot void transaction: insufficient stock to reverse item_id %d. available: %d, needed: %d", itemID, d.CurrentStock, d.Detail.Quantity))
				}
			}
		}

		return nil
	})
}
