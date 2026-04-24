package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/sajidannn/pos-api/internal/apierr"
	"github.com/sajidannn/pos-api/internal/dto"
	"github.com/sajidannn/pos-api/internal/service"
)

// ReportHandler handles HTTP requests for the /reports resource.
type ReportHandler struct {
	svc *service.ReportService
}

// NewReportHandler returns a new ReportHandler.
func NewReportHandler(svc *service.ReportService) *ReportHandler {
	return &ReportHandler{svc: svc}
}

// GetBranchBalance handles GET /reports/balance/branch/:id
func (h *ReportHandler) GetBranchBalance(c *gin.Context) {
	branchID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		_ = c.Error(apierr.BadRequest("invalid branch id"))
		return
	}

	var f dto.ReportFilter
	if err := c.ShouldBindQuery(&f); err != nil {
		_ = c.Error(apierr.BadRequest("invalid query parameters"))
		return
	}

	resp, err := h.svc.GetBranchBalance(c.Request.Context(), branchID, f)
	if err != nil {
		_ = c.Error(apierr.Wrap(err, "failed to get branch balance"))
		return
	}

	c.JSON(http.StatusOK, dto.Success(resp))
}

// GetTenantBalance handles GET /reports/balance/tenant
func (h *ReportHandler) GetTenantBalance(c *gin.Context) {
	var f dto.ReportFilter
	if err := c.ShouldBindQuery(&f); err != nil {
		_ = c.Error(apierr.BadRequest("invalid query parameters"))
		return
	}

	resp, err := h.svc.GetTenantBalance(c.Request.Context(), f)
	if err != nil {
		_ = c.Error(apierr.Wrap(err, "failed to get tenant balance"))
		return
	}

	c.JSON(http.StatusOK, dto.Success(resp))
}

// InjectCapital handles POST /reports/balance/tenant/capital
func (h *ReportHandler) InjectCapital(c *gin.Context) {
	var req dto.CapitalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apierr.BadRequest("invalid request payload"))
		return
	}

	if err := h.svc.InjectCapital(c.Request.Context(), req); err != nil {
		_ = c.Error(apierr.Wrap(err, "failed to inject capital"))
		return
	}

	// Fetch updated balance to return to the user
	resp, err := h.svc.GetTenantBalance(c.Request.Context(), dto.ReportFilter{})
	if err != nil {
		c.JSON(http.StatusOK, dto.Success(gin.H{"message": "capital updated successfully"}))
		return
	}

	c.JSON(http.StatusOK, dto.Success(resp))
}

// GetSummary handles GET /reports/summary
func (h *ReportHandler) GetSummary(c *gin.Context) {
	var f dto.SummaryFilter
	if err := c.ShouldBindQuery(&f); err != nil {
		_ = c.Error(apierr.BadRequest("invalid query parameters"))
		return
	}

	resp, err := h.svc.GetSummary(c.Request.Context(), f)
	if err != nil {
		_ = c.Error(apierr.Wrap(err, "failed to get summary"))
		return
	}

	c.JSON(http.StatusOK, dto.Success(resp))
}

// GetTopItems handles GET /reports/top-items
func (h *ReportHandler) GetTopItems(c *gin.Context) {
	var f dto.ItemsFilter
	if err := c.ShouldBindQuery(&f); err != nil {
		_ = c.Error(apierr.BadRequest("invalid query parameters"))
		return
	}

	items, err := h.svc.GetTopItems(c.Request.Context(), f)
	if err != nil {
		_ = c.Error(apierr.Wrap(err, "failed to get top items"))
		return
	}

	c.JSON(http.StatusOK, dto.Success(items))
}

// GetLowItems handles GET /reports/low-items
func (h *ReportHandler) GetLowItems(c *gin.Context) {
	var f dto.ItemsFilter
	if err := c.ShouldBindQuery(&f); err != nil {
		_ = c.Error(apierr.BadRequest("invalid query parameters"))
		return
	}

	items, err := h.svc.GetLowItems(c.Request.Context(), f)
	if err != nil {
		_ = c.Error(apierr.Wrap(err, "failed to get low items"))
		return
	}

	c.JSON(http.StatusOK, dto.Success(items))
}

// GetSalesReport handles GET /reports/sales
func (h *ReportHandler) GetSalesReport(c *gin.Context) {
	var f dto.SalesReportFilter
	if err := c.ShouldBindQuery(&f); err != nil {
		_ = c.Error(apierr.BadRequest("invalid query parameters"))
		return
	}

	entries, err := h.svc.GetSalesReport(c.Request.Context(), f)
	if err != nil {
		_ = c.Error(apierr.Wrap(err, "failed to get sales report"))
		return
	}

	c.JSON(http.StatusOK, dto.Success(entries))
}
