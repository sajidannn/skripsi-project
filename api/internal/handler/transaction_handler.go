package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/sajidannn/pos-api/internal/apierr"
	"github.com/sajidannn/pos-api/internal/dto"
	"github.com/sajidannn/pos-api/internal/model"
	"github.com/sajidannn/pos-api/internal/service"
	"github.com/sajidannn/pos-api/internal/validator"
)

// TransactionHandler handles HTTP requests for /transactions resources.
type TransactionHandler struct {
	svc *service.TransactionService
}

// NewTransactionHandler returns a new TransactionHandler.
func NewTransactionHandler(svc *service.TransactionService) *TransactionHandler {
	return &TransactionHandler{svc: svc}
}

// CreateSale handles POST /transactions/sale.
func (h *TransactionHandler) CreateSale(c *gin.Context) {
	var req dto.CreateSaleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apierr.ValidationFailed(validator.ParseBindingError(err)))
		return
	}

	// Retrieve user_id from the JWT claims to record who processed the real sale
	userID := 0
	claims, exists := c.Get("claims")
	if exists {
		if mapClaims, ok := claims.(jwt.MapClaims); ok {
			if idFloat, ok := mapClaims["user_id"].(float64); ok {
				userID = int(idFloat)
			}
		}
	}

	if userID == 0 {
		_ = c.Error(apierr.Unauthorized("unable to resolve user_id from token"))
		return
	}

	trx, err := h.svc.CreateSale(c.Request.Context(), userID, req)
	if err != nil {
		_ = c.Error(apierr.Wrap(err, "failed to process sale transaction"))
		return
	}

	c.JSON(http.StatusCreated, dto.Success(toTransactionResponse(trx)))
}

// toTransactionResponse maps internal model to DTO response
func toTransactionResponse(trx *model.Transaction) dto.TransactionResponse {
	resp := dto.TransactionResponse{
		ID:          trx.ID,
		TrxNo:       trx.TrxNo,
		TransType:   string(trx.TransType),
		BranchID:    trx.BranchID,
		WarehouseID: trx.WarehouseID,
		CustomerID:  trx.CustomerID,
		SupplierID:  trx.SupplierID,
		UserID:      trx.UserID,
		Tax:         trx.Tax,
		Discount:    trx.Discount,
		TotalAmount: trx.TotalAmount,
		Note:        trx.Note,
		CreatedAt:   trx.CreatedAt,
		Details:     make([]dto.TransactionItemResponse, 0, len(trx.Details)),
	}

	for _, d := range trx.Details {
		resp.Details = append(resp.Details, dto.TransactionItemResponse{
			BranchItemID:    d.BranchItemID,
			WarehouseItemID: d.WarehouseItemID,
			Quantity:        d.Quantity,
			Price:           d.Price,
			Subtotal:        d.Subtotal,
		})
	}

	return resp
}
