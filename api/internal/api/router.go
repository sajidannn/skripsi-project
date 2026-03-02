// Package api sets up the Gin HTTP server and registers all routes.
package api

import (
	"github.com/gin-gonic/gin"
	"github.com/sajidannn/pos-api/internal/handler"
	"github.com/sajidannn/pos-api/internal/middleware"
)

// Handlers groups all HTTP handlers used by the router.
type Handlers struct {
	Warehouse *handler.WarehouseHandler
	Branch    *handler.BranchHandler
}

// NewRouter builds and returns the Gin engine with all routes registered.
func NewRouter(jwtSecret string, h Handlers) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery(), middleware.RequestTiming(), gin.Logger())

	// Health check — no auth required.
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// All API routes require a valid JWT carrying tenant_id.
	api := r.Group("/api/v1", middleware.Auth(jwtSecret))
	{
		// Warehouses
		warehouses := api.Group("/warehouses")
		{
			warehouses.POST("", h.Warehouse.Create)
			warehouses.GET("", h.Warehouse.List)
			warehouses.GET("/:id", h.Warehouse.GetByID)
		}

		// Branches
		branches := api.Group("/branches")
		{
			branches.POST("", h.Branch.Create)
			branches.GET("", h.Branch.List)
			branches.GET("/:id", h.Branch.GetByID)
		}
	}

	return r
}
