package handler

import (
	"net/http"
	"strconv"

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
// Optional query param: ?low_stock=true
func (h *InventoryHandler) ListByBranch(c *gin.Context) {
	branchID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		_ = c.Error(apierr.BadRequest("invalid branch id"))
		return
	}

	lowStock := c.Query("low_stock") == "true"

	items, err := h.svc.ListByBranch(c.Request.Context(), branchID, lowStock)
	if err != nil {
		_ = c.Error(apierr.Wrap(err, "failed to list branch inventory"))
		return
	}

	resp := make([]dto.BranchItemResponse, len(items))
	for i, bi := range items {
		resp[i] = toBranchItemResponse(&bi)
	}
	c.JSON(http.StatusOK, dto.Success(resp))
}

// ListByWarehouse handles GET /inventory/warehouse/:id
// Optional query param: ?low_stock=true
func (h *InventoryHandler) ListByWarehouse(c *gin.Context) {
	warehouseID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		_ = c.Error(apierr.BadRequest("invalid warehouse id"))
		return
	}

	lowStock := c.Query("low_stock") == "true"

	items, err := h.svc.ListByWarehouse(c.Request.Context(), warehouseID, lowStock)
	if err != nil {
		_ = c.Error(apierr.Wrap(err, "failed to list warehouse inventory"))
		return
	}

	resp := make([]dto.WarehouseItemResponse, len(items))
	for i, wi := range items {
		resp[i] = toWarehouseItemResponse(&wi)
	}
	c.JSON(http.StatusOK, dto.Success(resp))
}

// toBranchItemResponse maps a domain model to the HTTP response DTO.
func toBranchItemResponse(bi *model.BranchItem) dto.BranchItemResponse {
	return dto.BranchItemResponse{
		ID:        bi.ID,
		BranchID:  bi.BranchID,
		ItemID:    bi.ItemID,
		ItemName:  bi.ItemName,
		SKU:       bi.SKU,
		Price:     bi.Price,
		Stock:     bi.Stock,
		UpdatedAt: bi.UpdatedAt,
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
		Price:       wi.Price,
		Stock:       wi.Stock,
		UpdatedAt:   wi.UpdatedAt,
	}
}
