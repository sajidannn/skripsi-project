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
	User        *handler.UserHandler
	Customer    *handler.CustomerHandler
	Supplier    *handler.SupplierHandler
	Transaction *handler.TransactionHandler
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

	// Login — public, produces a JWT.
	r.POST("/api/v1/auth/login", h.User.Login)

	// All API routes below require a valid JWT carrying tenant_id.
	api := r.Group("/api/v1", middleware.Auth(jwtSecret))
	{
		// Warehouses
		warehouses := api.Group("/warehouses")
		{
			warehouses.POST("", h.Warehouse.Create)
			warehouses.GET("", h.Warehouse.List)
			warehouses.GET("/:id", h.Warehouse.GetByID)
			warehouses.PUT("/:id", h.Warehouse.Update)
		}

		// Branches
		branches := api.Group("/branches")
		{
			branches.POST("", h.Branch.Create)
			branches.GET("", h.Branch.List)
			branches.GET("/:id", h.Branch.GetByID)
			branches.PUT("/:id", h.Branch.Update)
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

		// Users (employee management)
		users := api.Group("/users")
		{
			users.POST("", middleware.RequireRole("owner"), h.User.Create)
			users.GET("", middleware.RequireRole("owner", "manager"), h.User.List)
			users.GET("/:id", middleware.RequireRole("owner", "manager"), h.User.GetByID)
			users.PUT("/:id", middleware.RequireRole("owner"), h.User.Update)
			users.DELETE("/:id", middleware.RequireRole("owner"), h.User.Delete)
		}

		// Customers
		customers := api.Group("/customers")
		{
			customers.POST("", middleware.RequireRole("owner", "manager", "cashier"), h.Customer.Create)
			customers.GET("", middleware.RequireRole("owner", "manager", "cashier"), h.Customer.List)
			customers.GET("/:id", middleware.RequireRole("owner", "manager", "cashier"), h.Customer.GetByID)
			customers.PUT("/:id", middleware.RequireRole("owner", "manager", "cashier"), h.Customer.Update)
			customers.DELETE("/:id", middleware.RequireRole("owner", "manager"), h.Customer.Delete)
		}

		// Suppliers
		suppliers := api.Group("/suppliers")
		{
			suppliers.POST("", middleware.RequireRole("owner", "manager"), h.Supplier.Create)
			suppliers.GET("", middleware.RequireRole("owner", "manager", "cashier"), h.Supplier.List)
			suppliers.GET("/:id", middleware.RequireRole("owner", "manager"), h.Supplier.GetByID)
			suppliers.PUT("/:id", middleware.RequireRole("owner", "manager"), h.Supplier.Update)
			suppliers.DELETE("/:id", middleware.RequireRole("owner", "manager"), h.Supplier.Delete)
		}

		// Transactions
		transactions := api.Group("/transactions")
		{
			transactions.GET("", h.Transaction.List)
			transactions.GET("/:id", h.Transaction.GetByID)
			transactions.POST("/sale", middleware.RequireRole("owner", "manager", "cashier"), h.Transaction.CreateSale)
			transactions.POST("/purchase", middleware.RequireRole("owner", "manager"), h.Transaction.CreatePurchase)
			transactions.POST("/transfer", middleware.RequireRole("owner", "manager"), h.Transaction.CreateTransfer)
			transactions.POST("/return", middleware.RequireRole("owner", "manager", "cashier"), h.Transaction.CreateReturn)
			transactions.POST("/purchase-return", middleware.RequireRole("owner", "manager"), h.Transaction.CreatePurchaseReturn)
			transactions.POST("/adjust", middleware.RequireRole("owner", "manager"), h.Transaction.AdjustStock)
			transactions.POST("/:id/void", middleware.RequireRole("owner", "manager"), h.Transaction.Void)
		}
	}

	return r
}
