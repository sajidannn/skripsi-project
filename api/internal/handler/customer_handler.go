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

func (h *CustomerHandler) List(c *gin.Context) {
	var branchID int
	if b := c.Query("branch_id"); b != "" {
		if parsed, err := strconv.Atoi(b); err == nil {
			branchID = parsed
		}
	}

	customers, err := h.svc.List(c.Request.Context(), branchID)
	if err != nil {
		_ = c.Error(apierr.Wrap(err, "failed to list customers"))
		return
	}

	resp := make([]dto.CustomerResponse, len(customers))
	if len(customers) == 0 {
		resp = []dto.CustomerResponse{} // Output [] instead of null in JSON
	}
	for i, cust := range customers {
		resp[i] = toCustomerResponse(&cust)
	}

	c.JSON(http.StatusOK, dto.Success(resp))
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
