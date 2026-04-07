// Package handler provides HTTP handlers for the POS API.
// All handlers are mode-agnostic — they interact only with service interfaces.
package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/sajidannn/pos-api/internal/apierr"
	"github.com/sajidannn/pos-api/internal/dto"
	"github.com/sajidannn/pos-api/internal/model"
	"github.com/sajidannn/pos-api/internal/service"
	"github.com/sajidannn/pos-api/internal/validator"
)

// WarehouseHandler handles HTTP requests for the /warehouses resource.
type WarehouseHandler struct {
	svc *service.WarehouseService
}

// NewWarehouseHandler returns a new WarehouseHandler.
func NewWarehouseHandler(svc *service.WarehouseService) *WarehouseHandler {
	return &WarehouseHandler{svc: svc}
}

// Create handles POST /warehouses
func (h *WarehouseHandler) Create(c *gin.Context) {
	var req dto.CreateWarehouseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apierr.ValidationFailed(validator.ParseBindingError(err)))
		return
	}

	warehouse, err := h.svc.Create(c.Request.Context(), req)
	if err != nil {
		_ = c.Error(apierr.Wrap(err, "warehouse not found"))
		return
	}

	c.JSON(http.StatusCreated, dto.Success(toWarehouseResponse(warehouse)))
}

// GetByID handles GET /warehouses/:id
func (h *WarehouseHandler) GetByID(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		_ = c.Error(apierr.BadRequest("invalid warehouse id"))
		return
	}

	warehouse, err := h.svc.GetByID(c.Request.Context(), id)

	if err != nil {
		_ = c.Error(apierr.Wrap(err, "warehouse not found"))
		return
	}

	c.JSON(http.StatusOK, dto.Success(toWarehouseResponse(warehouse)))
}

// List handles GET /warehouses
func (h *WarehouseHandler) List(c *gin.Context) {
	warehouses, err := h.svc.List(c.Request.Context())

	if err != nil {
		_ = c.Error(apierr.Wrap(err, "warehouse not found"))
		return
	}

	resp := make([]dto.WarehouseResponse, len(warehouses))
	for i, w := range warehouses {
		resp[i] = toWarehouseResponse(&w)
	}
	c.JSON(http.StatusOK, dto.Success(resp))
}

// toWarehouseResponse maps a domain model to the HTTP response DTO.
func toWarehouseResponse(w *model.Warehouse) dto.WarehouseResponse {
	return dto.WarehouseResponse{
		ID:        w.ID,
		Name:      w.Name,
		CreatedAt: w.CreatedAt,
	}
}
