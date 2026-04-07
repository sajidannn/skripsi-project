package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sajidannn/pos-api/internal/service"
)

type TenantHandler struct {
	svc *service.TenantService
}

func NewTenantHandler(svc *service.TenantService) *TenantHandler {
	return &TenantHandler{svc: svc}
}

func (h *TenantHandler) List(c *gin.Context) {
	tenants, err := h.svc.ListTenants(c.Request.Context())
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": tenants,
	})
}
