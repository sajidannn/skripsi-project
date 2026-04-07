// Package api sets up the Gin HTTP server and registers all routes.
package api

import (
	"github.com/gin-gonic/gin"
	"github.com/sajidannn/pos-api/internal/handler"
	"github.com/sajidannn/pos-api/internal/middleware"
)

// Handlers groups all HTTP handlers used by the router.
type Handlers struct {
	Tenant    *handler.TenantHandler
	Warehouse *handler.WarehouseHandler
	Branch    *handler.BranchHandler
	Item      *handler.ItemHandler
	Inventory *handler.InventoryHandler
}

// NewRouter builds and returns the Gin engine with all routes registered.
func NewRouter(jwtSecret string, debug bool, h Handlers) *gin.Engine {
	r := gin.New()
	// ErrorHandler must be outermost so it executes last (after all handlers).
	r.Use(middleware.ErrorHandler(debug), gin.Recovery(), gin.Logger())

	// Health check — no auth required.
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Unprotected endpoint for listing tenants (useful for testing/Locust benchmark).
	r.GET("/api/v1/tenants", h.Tenant.List)

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

		// Items (master catalogue)
		items := api.Group("/items")
		{
			items.POST("", h.Item.Create)
			items.GET("", h.Item.List)
			items.GET("/:id", h.Item.GetByID)
			items.PUT("/:id", h.Item.Update)
			items.DELETE("/:id", h.Item.Delete)
		}

		// Inventory (read-only stock view per location)
		inventory := api.Group("/inventory")
		{
			inventory.GET("/branch/:id", h.Inventory.ListByBranch)
			inventory.GET("/warehouse/:id", h.Inventory.ListByWarehouse)
		}
	}

	return r
}
