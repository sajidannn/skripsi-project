package service

import (
	"context"
	"fmt"

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
	return s.repo.ExecuteSaleTx(ctx, tenantID, userID, req, calculateSaleFn)
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

	return s.repo.ExecutePurchaseTx(ctx, tenantID, userID, req, calculatePurchaseFn)
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

	return s.repo.ExecuteTransferTx(ctx, tenantID, userID, req, calculateTransferFn)
}
