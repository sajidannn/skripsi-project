package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
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

	userID, ok := h.getUserID(c)
	if !ok {
		return
	}

	trx, err := h.svc.CreateSale(c.Request.Context(), userID, req)
	if err != nil {
		_ = c.Error(apierr.Wrap(err, "failed to process sale transaction"))
		return
	}

	c.JSON(http.StatusCreated, dto.Success(toTransactionResponse(trx)))
}

// CreatePurchase handles POST /api/v1/transactions/purchase.
func (h *TransactionHandler) CreatePurchase(c *gin.Context) {
	var req dto.CreatePurchaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apierr.BadRequest(err.Error()))
		return
	}

	userID, ok := h.getUserID(c)
	if !ok {
		return
	}
	res, err := h.svc.CreatePurchase(c.Request.Context(), userID, req)
	if err != nil {
		_ = c.Error(apierr.Wrap(err, "failed to create purchase"))
		return
	}

	c.JSON(http.StatusCreated, dto.Success(toTransactionResponse(res)))
}

// CreateTransfer handles POST /api/v1/transactions/transfer.
func (h *TransactionHandler) CreateTransfer(c *gin.Context) {
	var req dto.CreateTransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apierr.BadRequest(err.Error()))
		return
	}

	userID, ok := h.getUserID(c)
	if !ok {
		return
	}
	res, err := h.svc.CreateTransfer(c.Request.Context(), userID, req)
	if err != nil {
		_ = c.Error(apierr.Wrap(err, "failed to create transfer"))
		return
	}

	c.JSON(http.StatusCreated, dto.Success(toTransactionResponse(res)))
}

// CreateReturn handles POST /api/v1/transactions/return.
func (h *TransactionHandler) CreateReturn(c *gin.Context) {
	var req dto.CreateReturnRequest
	if err := h.shouldBind(c, &req); err != nil {
		return
	}

	userID, ok := h.getUserID(c)
	if !ok {
		return
	}
	res, err := h.svc.CreateReturn(c.Request.Context(), userID, req)
	if err != nil {
		_ = c.Error(apierr.Wrap(err, "failed to create return"))
		return
	}

	c.JSON(http.StatusCreated, dto.Success(toTransactionResponse(res)))
}

// CreatePurchaseReturn handles POST /api/v1/transactions/purchase-return.
func (h *TransactionHandler) CreatePurchaseReturn(c *gin.Context) {
	var req dto.CreatePurchaseReturnRequest
	if err := h.shouldBind(c, &req); err != nil {
		return
	}

	userID, ok := h.getUserID(c)
	if !ok {
		return
	}

	res, err := h.svc.CreatePurchaseReturn(c.Request.Context(), userID, req)
	if err != nil {
		_ = c.Error(apierr.Wrap(err, "failed to create purchase return"))
		return
	}

	c.JSON(http.StatusCreated, dto.Success(toTransactionResponse(res)))
}

// AdjustStock handles POST /api/v1/transactions/adjust.
func (h *TransactionHandler) AdjustStock(c *gin.Context) {
	var req dto.AdjustStockRequest
	if err := h.shouldBind(c, &req); err != nil {
		return
	}

	userID, ok := h.getUserID(c)
	if !ok {
		return
	}

	err := h.svc.AdjustStock(c.Request.Context(), userID, req)
	if err != nil {
		_ = c.Error(apierr.Wrap(err, "failed to adjust stock"))
		return
	}

	c.JSON(http.StatusOK, dto.Success("stock adjusted successfully"))
}

func (h *TransactionHandler) getUserID(c *gin.Context) (int, bool) {
	userID := c.GetInt("user_id")
	if userID == 0 {
		_ = c.Error(apierr.Unauthorized("unable to resolve user_id from token or missing login session"))
		return 0, false
	}
	return userID, true
}

// shouldBind is a helper to bind and handle validation errors.
func (h *TransactionHandler) shouldBind(c *gin.Context, req interface{}) error {
	if err := c.ShouldBindJSON(req); err != nil {
		_ = c.Error(apierr.BadRequest(err.Error()))
		return err
	}
	return nil
}

// List handles GET /api/v1/transactions.
func (h *TransactionHandler) List(c *gin.Context) {
	var rawQ dto.PageQuery
	if err := c.ShouldBindQuery(&rawQ); err != nil {
		_ = c.Error(apierr.BadRequest("invalid query parameters"))
		return
	}
	q := rawQ.Validate(
		[]string{"id", "trxno", "trans_type", "total_amount", "created_at"},
		"created_at",
	)
	if rawQ.Order == "" {
		q.Order = "DESC" // default to newest first
	}

	var f dto.TransactionFilter
	if err := c.ShouldBindQuery(&f); err != nil {
		_ = c.Error(apierr.BadRequest("invalid filter parameters"))
		return
	}

	if s := c.Query("date_from"); s != "" {
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			_ = c.Error(apierr.BadRequest("date_from must be YYYY-MM-DD"))
			return
		}
		f.DateFrom = &t
	}
	if s := c.Query("date_to"); s != "" {
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			_ = c.Error(apierr.BadRequest("date_to must be YYYY-MM-DD"))
			return
		}
		end := t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
		f.DateTo = &end
	}

	list, total, err := h.svc.List(c.Request.Context(), q, f)
	if err != nil {
		_ = c.Error(apierr.Wrap(err, "failed to list transactions"))
		return
	}

	resp := make([]dto.TransactionListResponse, len(list))
	for i, trx := range list {
		resp[i] = toTransactionListResponse(&trx)
	}

	c.JSON(http.StatusOK, dto.PagedOK(resp, dto.NewPageMeta(q, total)))
}

// GetByID handles GET /api/v1/transactions/:id.
func (h *TransactionHandler) GetByID(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		_ = c.Error(apierr.BadRequest("invalid transaction id"))
		return
	}

	res, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		_ = c.Error(apierr.Wrap(err, "transaction not found"))
		return
	}

	c.JSON(http.StatusOK, dto.Success(toTransactionResponse(res)))
}

// toTransactionResponse maps internal model to DTO response
func toTransactionResponse(trx *model.Transaction) dto.TransactionResponse {
	resp := dto.TransactionResponse{
		ID:                     trx.ID,
		TrxNo:                  trx.TrxNo,
		TransType:              string(trx.TransType),
		BranchID:               trx.BranchID,
		WarehouseID:            trx.WarehouseID,
		CustomerID:             trx.CustomerID,
		SupplierID:             trx.SupplierID,
		UserID:                 trx.UserID,
		Tax:                    trx.Tax,
		Discount:               trx.Discount,
		TotalAmount:            trx.TotalAmount,
		Note:                   trx.Note,
		ReferenceTransactionID: trx.ReferenceTransactionID,
		CreatedAt:              trx.CreatedAt,
		Details:                make([]dto.TransactionItemResponse, 0, len(trx.Details)),
	}

	for _, d := range trx.Details {
		resp.Details = append(resp.Details, dto.TransactionItemResponse{
			BranchItemID:    d.BranchItemID,
			WarehouseItemID: d.WarehouseItemID,
			Quantity:        d.Quantity,
			COGS:            d.COGS,
			Price:           d.Price,
			Subtotal:        d.Subtotal,
		})
	}

	return resp
}

func toTransactionListResponse(trx *model.Transaction) dto.TransactionListResponse {
	return dto.TransactionListResponse{
		ID:          trx.ID,
		TrxNo:       trx.TrxNo,
		TransType:   string(trx.TransType),
		BranchID:    trx.BranchID,
		WarehouseID: trx.WarehouseID,
		CustomerID:  trx.CustomerID,
		SupplierID:  trx.SupplierID,
		TotalAmount: trx.TotalAmount,
		CreatedAt:   trx.CreatedAt,
	}
}

// Void handles POST /transactions/:id/void.
func (h *TransactionHandler) Void(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		_ = c.Error(apierr.BadRequest("invalid transaction id format"))
		return
	}

	var req dto.VoidRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apierr.ValidationFailed(validator.ParseBindingError(err)))
		return
	}

	userID, ok := h.getUserID(c)
	if !ok {
		return
	}
	trx, err := h.svc.Void(c.Request.Context(), userID, id, req)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "transaction voided successfully",
		"data": gin.H{
			"void_transaction_id": trx.ID,
			"void_trxno":          trx.TrxNo,
		},
	})
}
