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

// BranchHandler handles HTTP requests for the /branches resource.
type BranchHandler struct {
	svc *service.BranchService
}

// NewBranchHandler returns a new BranchHandler.
func NewBranchHandler(svc *service.BranchService) *BranchHandler {
	return &BranchHandler{svc: svc}
}

// Create handles POST /branches
func (h *BranchHandler) Create(c *gin.Context) {
	var req dto.CreateBranchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apierr.ValidationFailed(validator.ParseBindingError(err)))
		return
	}

	branch, err := h.svc.Create(c.Request.Context(), req)
	if err != nil {
		_ = c.Error(apierr.Wrap(err, "branch not found"))
		return
	}

	c.JSON(http.StatusCreated, dto.Success(toBranchResponse(branch)))
}

// GetByID handles GET /branches/:id
func (h *BranchHandler) GetByID(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		_ = c.Error(apierr.BadRequest("invalid branch id"))
		return
	}

	branch, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		_ = c.Error(apierr.Wrap(err, "branch not found"))
		return
	}

	c.JSON(http.StatusOK, dto.Success(toBranchResponse(branch)))
}

// List handles GET /branches
func (h *BranchHandler) List(c *gin.Context) {
	branches, err := h.svc.List(c.Request.Context())
	if err != nil {
		_ = c.Error(apierr.Wrap(err, "branch not found"))
		return
	}

	resp := make([]dto.BranchResponse, len(branches))
	for i, b := range branches {
		resp[i] = toBranchResponse(&b)
	}
	c.JSON(http.StatusOK, dto.Success(resp))
}

func (h *BranchHandler) Update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		_ = c.Error(apierr.BadRequest("invalid branch id"))
		return
	}

	var req dto.UpdateBranchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apierr.ValidationFailed(validator.ParseBindingError(err)))
		return
	}

	branch, err := h.svc.Update(c.Request.Context(), id, req)
	if err != nil {
		_ = c.Error(apierr.Wrap(err, "failed to update branch"))
		return
	}

	c.JSON(http.StatusOK, dto.Success(toBranchResponse(branch)))
}

// toBranchResponse maps a domain model to the HTTP response DTO.
func toBranchResponse(b *model.Branch) dto.BranchResponse {
	return dto.BranchResponse{
		ID:             b.ID,
		Name:           b.Name,
		Phone:          b.Phone,
		Address:        b.Address,
		OpeningBalance: b.OpeningBalance,
		CreatedAt:      b.CreatedAt,
	}
}
