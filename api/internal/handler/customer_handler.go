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

type CustomerHandler struct {
	svc *service.CustomerService
}

func NewCustomerHandler(svc *service.CustomerService) *CustomerHandler {
	return &CustomerHandler{svc: svc}
}

func (h *CustomerHandler) Create(c *gin.Context) {
	var req dto.CreateCustomerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apierr.ValidationFailed(validator.ParseBindingError(err)))
		return
	}

	customer, err := h.svc.Create(c.Request.Context(), req)
	if err != nil {
		_ = c.Error(apierr.Wrap(err, "failed to create customer"))
		return
	}

	c.JSON(http.StatusCreated, dto.Success(toCustomerResponse(customer)))
}

func (h *CustomerHandler) GetByID(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		_ = c.Error(apierr.BadRequest("invalid customer id"))
		return
	}

	customer, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		_ = c.Error(apierr.Wrap(err, "customer not found"))
		return
	}

	c.JSON(http.StatusOK, dto.Success(toCustomerResponse(customer)))
}

// List handles GET /customers
// Query params (optional): page, limit, sort, order, search, branch_id, date_from, date_to
func (h *CustomerHandler) List(c *gin.Context) {
	var rawQ dto.PageQuery
	if err := c.ShouldBindQuery(&rawQ); err != nil {
		_ = c.Error(apierr.BadRequest("invalid query parameters"))
		return
	}
	q := rawQ.Validate(
		[]string{"id", "branch_id", "name", "phone", "email", "created_at"},
		"name", // default sort
	)

	var f dto.CustomerFilter
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

	customers, total, err := h.svc.List(c.Request.Context(), q, f)
	if err != nil {
		_ = c.Error(apierr.Wrap(err, "failed to list customers"))
		return
	}

	resp := make([]dto.CustomerResponse, len(customers))
	for i, cust := range customers {
		resp[i] = toCustomerResponse(&cust)
	}

	c.JSON(http.StatusOK, dto.PagedOK(resp, dto.NewPageMeta(q, total)))
}

func (h *CustomerHandler) Update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		_ = c.Error(apierr.BadRequest("invalid customer id"))
		return
	}

	var req dto.UpdateCustomerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apierr.ValidationFailed(validator.ParseBindingError(err)))
		return
	}

	customer, err := h.svc.Update(c.Request.Context(), id, req)
	if err != nil {
		_ = c.Error(apierr.Wrap(err, "failed to update customer"))
		return
	}

	c.JSON(http.StatusOK, dto.Success(toCustomerResponse(customer)))
}

func (h *CustomerHandler) Delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		_ = c.Error(apierr.BadRequest("invalid customer id"))
		return
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		_ = c.Error(apierr.Wrap(err, "failed to delete customer"))
		return
	}

	c.JSON(http.StatusOK, dto.Success[any](nil))
}

func toCustomerResponse(c *model.Customer) dto.CustomerResponse {
	return dto.CustomerResponse{
		ID:        c.ID,
		BranchID:  c.BranchID,
		Name:      c.Name,
		Phone:     c.Phone,
		Email:     c.Email,
		CreatedAt: c.CreatedAt,
	}
}
