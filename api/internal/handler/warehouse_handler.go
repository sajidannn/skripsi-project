// Package handler provides HTTP handlers for the POS API.
// All handlers are mode-agnostic — they interact only with service interfaces.
package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/sajidannn/pos-api/internal/middleware"
	"github.com/sajidannn/pos-api/internal/model"
	"github.com/sajidannn/pos-api/internal/service"
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
	var req model.CreateWarehouseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	warehouse, err := h.svc.Create(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": warehouse})
}

// GetByID handles GET /warehouses/:id
func (h *WarehouseHandler) GetByID(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid warehouse id"})
		return
	}

	if t := middleware.TimingFromContext(c.Request.Context()); t != nil {
		t.Mark("handler_start")
	}

	warehouse, err := h.svc.GetByID(c.Request.Context(), id)

	if t := middleware.TimingFromContext(c.Request.Context()); t != nil {
		t.Mark("db_done")
	}

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "warehouse not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": warehouse})
}

// List handles GET /warehouses
func (h *WarehouseHandler) List(c *gin.Context) {
	if t := middleware.TimingFromContext(c.Request.Context()); t != nil {
		t.Mark("handler_start")
	}

	warehouses, err := h.svc.List(c.Request.Context())

	if t := middleware.TimingFromContext(c.Request.Context()); t != nil {
		t.Mark("db_done")
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": warehouses})
}
