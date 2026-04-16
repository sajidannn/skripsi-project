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

// ItemHandler handles HTTP requests for the /items resource.
type ItemHandler struct {
	svc *service.ItemService
}

// NewItemHandler returns a new ItemHandler.
func NewItemHandler(svc *service.ItemService) *ItemHandler {
	return &ItemHandler{svc: svc}
}

// Create handles POST /items
func (h *ItemHandler) Create(c *gin.Context) {
	var req dto.CreateItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apierr.ValidationFailed(validator.ParseBindingError(err)))
		return
	}

	item, err := h.svc.Create(c.Request.Context(), req)
	if err != nil {
		_ = c.Error(apierr.Wrap(err, "failed to create item"))
		return
	}

	c.JSON(http.StatusCreated, dto.Success(toItemResponse(item)))
}

// GetByID handles GET /items/:id
func (h *ItemHandler) GetByID(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		_ = c.Error(apierr.BadRequest("invalid item id"))
		return
	}

	item, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		_ = c.Error(apierr.Wrap(err, "item not found"))
		return
	}

	c.JSON(http.StatusOK, dto.Success(toItemResponse(item)))
}

// List handles GET /items
//
// Query params (all optional):
//
//	page, limit, sort, order          — pagination
//	search                            — ILIKE across name, sku, description
//	sku                               — exact match
//	min_price, max_price              — price range
//	date_from, date_to                — created_at range (YYYY-MM-DD)
func (h *ItemHandler) List(c *gin.Context) {
	// --- parse & validate pagination ---
	var rawQ dto.PageQuery
	if err := c.ShouldBindQuery(&rawQ); err != nil {
		_ = c.Error(apierr.BadRequest("invalid query parameters"))
		return
	}
	// Whitelist of sortable columns; handler owns this knowledge to prevent SQLI.
	q := rawQ.Validate(
		[]string{"id", "name", "sku", "cost", "price", "created_at", "updated_at"},
		"id", // default sort
	)

	// --- parse & validate filter ---
	var f dto.ItemFilter
	if err := c.ShouldBindQuery(&f); err != nil {
		_ = c.Error(apierr.BadRequest("invalid filter parameters"))
		return
	}

	// Parse date strings manually (gin form binding doesn't auto-parse time.Time).
	if s := c.Query("date_from"); s != "" {
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			_ = c.Error(apierr.BadRequest("date_from must be in YYYY-MM-DD format"))
			return
		}
		f.DateFrom = &t
	}
	if s := c.Query("date_to"); s != "" {
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			_ = c.Error(apierr.BadRequest("date_to must be in YYYY-MM-DD format"))
			return
		}
		// include the full day
		end := t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
		f.DateTo = &end
	}

	// --- call service ---
	items, total, err := h.svc.List(c.Request.Context(), q, f)
	if err != nil {
		_ = c.Error(apierr.Wrap(err, "failed to list items"))
		return
	}

	resp := make([]dto.ItemResponse, len(items))
	for i, it := range items {
		resp[i] = toItemResponse(&it)
	}

	c.JSON(http.StatusOK, dto.PagedOK(resp, dto.NewPageMeta(q, total)))
}

// Update handles PUT /items/:id
func (h *ItemHandler) Update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		_ = c.Error(apierr.BadRequest("invalid item id"))
		return
	}

	var req dto.UpdateItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apierr.ValidationFailed(validator.ParseBindingError(err)))
		return
	}

	item, err := h.svc.Update(c.Request.Context(), id, req)
	if err != nil {
		_ = c.Error(apierr.Wrap(err, "item not found"))
		return
	}

	c.JSON(http.StatusOK, dto.Success(toItemResponse(item)))
}

// Delete handles DELETE /items/:id
func (h *ItemHandler) Delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		_ = c.Error(apierr.BadRequest("invalid item id"))
		return
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		_ = c.Error(apierr.Wrap(err, "item not found"))
		return
	}

	c.JSON(http.StatusOK, dto.Success(gin.H{"deleted": id}))
}

// toItemResponse maps a domain model to the HTTP response DTO.
func toItemResponse(it *model.Item) dto.ItemResponse {
	return dto.ItemResponse{
		ID:          it.ID,
		Name:        it.Name,
		SKU:         it.SKU,
		Cost:        it.Cost,
		Price:           it.Price,
		MarginThreshold: it.MarginThreshold,
		Description:     it.Description,
		CreatedAt:       it.CreatedAt,
		UpdatedAt:       it.UpdatedAt,
	}
}
