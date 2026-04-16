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
)

// InventoryHandler handles HTTP requests for the /inventory resource.
type InventoryHandler struct {
	svc *service.InventoryService
}

// NewInventoryHandler returns a new InventoryHandler.
func NewInventoryHandler(svc *service.InventoryService) *InventoryHandler {
	return &InventoryHandler{svc: svc}
}

// ListByBranch handles GET /inventory/branch/:id
func (h *InventoryHandler) ListByBranch(c *gin.Context) {
	branchID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		_ = c.Error(apierr.BadRequest("invalid branch id"))
		return
	}

	var rawQ dto.PageQuery
	if err := c.ShouldBindQuery(&rawQ); err != nil {
		_ = c.Error(apierr.BadRequest("invalid query parameters"))
		return
	}
	q := rawQ.Validate(
		[]string{"id", "item_id", "stock", "updated_at"},
		"id", // default sort
	)

	var f dto.InventoryFilter
	if err := c.ShouldBindQuery(&f); err != nil {
		_ = c.Error(apierr.BadRequest("invalid filter parameters"))
		return
	}

	if s := c.Query("date_from"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			f.DateFrom = &t
		} else {
			_ = c.Error(apierr.BadRequest("date_from must be in YYYY-MM-DD format"))
			return
		}
	}
	if s := c.Query("date_to"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			end := t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
			f.DateTo = &end
		} else {
			_ = c.Error(apierr.BadRequest("date_to must be in YYYY-MM-DD format"))
			return
		}
	}

	items, total, err := h.svc.ListByBranch(c.Request.Context(), branchID, q, f)
	if err != nil {
		_ = c.Error(apierr.Wrap(err, "failed to list branch inventory"))
		return
	}

	resp := make([]dto.BranchItemResponse, len(items))
	for i, bi := range items {
		resp[i] = toBranchItemResponse(&bi)
	}
	c.JSON(http.StatusOK, dto.PagedOK(resp, dto.NewPageMeta(q, total)))
}

// ListByWarehouse handles GET /inventory/warehouse/:id
func (h *InventoryHandler) ListByWarehouse(c *gin.Context) {
	warehouseID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		_ = c.Error(apierr.BadRequest("invalid warehouse id"))
		return
	}

	var rawQ dto.PageQuery
	if err := c.ShouldBindQuery(&rawQ); err != nil {
		_ = c.Error(apierr.BadRequest("invalid query parameters"))
		return
	}
	q := rawQ.Validate(
		[]string{"id", "item_id", "stock", "updated_at"},
		"id", // default sort
	)

	var f dto.InventoryFilter
	if err := c.ShouldBindQuery(&f); err != nil {
		_ = c.Error(apierr.BadRequest("invalid filter parameters"))
		return
	}

	if s := c.Query("date_from"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			f.DateFrom = &t
		} else {
			_ = c.Error(apierr.BadRequest("date_from must be in YYYY-MM-DD format"))
			return
		}
	}
	if s := c.Query("date_to"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			end := t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
			f.DateTo = &end
		} else {
			_ = c.Error(apierr.BadRequest("date_to must be in YYYY-MM-DD format"))
			return
		}
	}

	items, total, err := h.svc.ListByWarehouse(c.Request.Context(), warehouseID, q, f)
	if err != nil {
		_ = c.Error(apierr.Wrap(err, "failed to list warehouse inventory"))
		return
	}

	resp := make([]dto.WarehouseItemResponse, len(items))
	for i, wi := range items {
		resp[i] = toWarehouseItemResponse(&wi)
	}
	c.JSON(http.StatusOK, dto.PagedOK(resp, dto.NewPageMeta(q, total)))
}

// toBranchItemResponse maps a domain model to the HTTP response DTO.
func toBranchItemResponse(bi *model.BranchItem) dto.BranchItemResponse {
	// Calculate margin
	finalPrice := bi.BasePrice
	if bi.BranchPrice != nil {
		finalPrice = *bi.BranchPrice
	}

	marginPercent := 0.0
	costF, _ := bi.Cost.Float64()
	if costF > 0 {
		finalPriceF, _ := finalPrice.Float64()
		marginPercent = ((finalPriceF - costF) / costF) * 100.0
	}

	thresholdF, _ := bi.MarginThreshold.Float64()
	marginWarning := false
	if thresholdF > 0 && marginPercent <= thresholdF {
		marginWarning = true
	}

	return dto.BranchItemResponse{
		ID:              bi.ID,
		BranchID:        bi.BranchID,
		ItemID:          bi.ItemID,
		ItemName:        bi.ItemName,
		SKU:             bi.SKU,
		Stock:           bi.Stock,
		Cost:            bi.Cost,
		BasePrice:       bi.BasePrice,
		BranchPrice:     bi.BranchPrice,
		FinalPrice:      finalPrice,
		MarginPercent:   marginPercent,
		MarginWarning:   marginWarning,
		MarginThreshold: bi.MarginThreshold,
		UpdatedAt:       bi.UpdatedAt,
	}
}

// toWarehouseItemResponse maps a domain model to the HTTP response DTO.
func toWarehouseItemResponse(wi *model.WarehouseItem) dto.WarehouseItemResponse {
	return dto.WarehouseItemResponse{
		ID:          wi.ID,
		WarehouseID: wi.WarehouseID,
		ItemID:      wi.ItemID,
		ItemName:    wi.ItemName,
		SKU:         wi.SKU,
		Cost:        wi.Cost,
		Price:       wi.Price,
		Stock:       wi.Stock,
		UpdatedAt:   wi.UpdatedAt,
	}
}

// UpdateBranchItemPrice handles PUT /inventory/branch/:branch_id/item/:item_id/price
func (h *InventoryHandler) UpdateBranchItemPrice(c *gin.Context) {
	branchID, err := strconv.Atoi(c.Param("branch_id"))
	if err != nil {
		_ = c.Error(apierr.BadRequest("invalid branch id"))
		return
	}
	itemID, err := strconv.Atoi(c.Param("item_id"))
	if err != nil {
		_ = c.Error(apierr.BadRequest("invalid item id"))
		return
	}

	var req dto.UpdateBranchItemPriceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apierr.BadRequest("invalid request payload"))
		return
	}

	bi, err := h.svc.UpdateBranchItemPrice(c.Request.Context(), branchID, itemID, req)
	if err != nil {
		_ = c.Error(apierr.Wrap(err, "failed to update branch item price"))
		return
	}

	c.JSON(http.StatusOK, dto.Success(toBranchItemResponse(bi)))
}
