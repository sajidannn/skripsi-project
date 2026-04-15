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

type SupplierHandler struct {
	svc *service.SupplierService
}

func NewSupplierHandler(svc *service.SupplierService) *SupplierHandler {
	return &SupplierHandler{svc: svc}
}

func (h *SupplierHandler) Create(c *gin.Context) {
	var req dto.CreateSupplierRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apierr.ValidationFailed(validator.ParseBindingError(err)))
		return
	}

	supplier, err := h.svc.Create(c.Request.Context(), req)
	if err != nil {
		_ = c.Error(apierr.Wrap(err, "failed to create supplier"))
		return
	}

	c.JSON(http.StatusCreated, dto.Success(toSupplierResponse(supplier)))
}

func (h *SupplierHandler) GetByID(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		_ = c.Error(apierr.BadRequest("invalid supplier id"))
		return
	}

	supplier, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		_ = c.Error(apierr.Wrap(err, "supplier not found"))
		return
	}

	c.JSON(http.StatusOK, dto.Success(toSupplierResponse(supplier)))
}

// List handles GET /suppliers
func (h *SupplierHandler) List(c *gin.Context) {
	var rawQ dto.PageQuery
	if err := c.ShouldBindQuery(&rawQ); err != nil {
		_ = c.Error(apierr.BadRequest("invalid query parameters"))
		return
	}
	q := rawQ.Validate(
		[]string{"id", "name", "phone", "address", "created_at"},
		"name", // default sort
	)

	var f dto.SupplierFilter
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

	suppliers, total, err := h.svc.List(c.Request.Context(), q, f)
	if err != nil {
		_ = c.Error(apierr.Wrap(err, "failed to list suppliers"))
		return
	}

	resp := make([]dto.SupplierResponse, len(suppliers))
	for i, s := range suppliers {
		resp[i] = toSupplierResponse(&s)
	}

	c.JSON(http.StatusOK, dto.PagedOK(resp, dto.NewPageMeta(q, total)))
}

func (h *SupplierHandler) Update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		_ = c.Error(apierr.BadRequest("invalid supplier id"))
		return
	}

	var req dto.UpdateSupplierRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apierr.ValidationFailed(validator.ParseBindingError(err)))
		return
	}

	supplier, err := h.svc.Update(c.Request.Context(), id, req)
	if err != nil {
		_ = c.Error(apierr.Wrap(err, "failed to update supplier"))
		return
	}

	c.JSON(http.StatusOK, dto.Success(toSupplierResponse(supplier)))
}

func (h *SupplierHandler) Delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		_ = c.Error(apierr.BadRequest("invalid supplier id"))
		return
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		_ = c.Error(apierr.Wrap(err, "failed to delete supplier"))
		return
	}

	c.JSON(http.StatusOK, dto.Success[any](nil))
}

func toSupplierResponse(s *model.Supplier) dto.SupplierResponse {
	return dto.SupplierResponse{
		ID:        s.ID,
		Name:      s.Name,
		Phone:     s.Phone,
		Address:   s.Address,
		CreatedAt: s.CreatedAt,
	}
}
