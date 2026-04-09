package service

import (
	"context"
	"fmt"

	"github.com/sajidannn/pos-api/internal/dto"
	"github.com/sajidannn/pos-api/internal/model"
	"github.com/sajidannn/pos-api/internal/repository"
	"github.com/sajidannn/pos-api/internal/tenant"
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
		var trxTotalAmount float64
		var finalDetails []model.TransactionDetail

		// 1. Process Mathematical Validations & Subtotals
		for _, reqItem := range req.Items {
			dbItem, exists := loadedDbItems[reqItem.BranchItemID]
			if !exists {
				return model.FinalSaleAggregate{}, fmt.Errorf("item with id %d not found in loaded data", reqItem.BranchItemID)
			}

			// Validate Stock domain rule
			if dbItem.AvailableQty < reqItem.Qty {
				return model.FinalSaleAggregate{}, fmt.Errorf("insufficient stock for item %d: available %d, requested %d", dbItem.BranchItemID, dbItem.AvailableQty, reqItem.Qty)
			}

			// Calculate Subtotal
			subtotal := float64(reqItem.Qty) * dbItem.Price
			trxTotalAmount += subtotal

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
		trxTotalAmount = trxTotalAmount + req.Tax - req.Discount
		
		// Optional Business Rule: Prevent total from going negative
		if trxTotalAmount < 0 {
			trxTotalAmount = 0
		}

		// Optional Business Rule: Discount cannot be more than 50% of the raw subtotal sum
		// (Example of logic that easily belongs in Service)
		// rawSubtotal := float64(0)
		// for _, d := range finalDetails { rawSubtotal += d.Subtotal }
		// if req.Discount > rawSubtotal * 0.5 { return ... error }

		return model.FinalSaleAggregate{
			TotalAmount: trxTotalAmount,
			Details:     finalDetails,
		}, nil
	}

	// 3. Delegate execution to Repository using the defined logic
	return s.repo.ExecuteSaleTx(ctx, tenantID, userID, req, calculateSaleFn)
}
