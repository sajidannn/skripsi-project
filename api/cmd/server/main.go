package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sajidannn/pos-api/config"
	"github.com/sajidannn/pos-api/internal/api"
	"github.com/sajidannn/pos-api/internal/db/multidb"
	"github.com/sajidannn/pos-api/internal/db/singledb"
	"github.com/sajidannn/pos-api/internal/handler"
	"github.com/sajidannn/pos-api/internal/repository"
	repoMulti "github.com/sajidannn/pos-api/internal/repository/multidb"
	repoSingle "github.com/sajidannn/pos-api/internal/repository/singledb"
	"github.com/sajidannn/pos-api/internal/service"
)

func main() {
	cfg := config.Load()

	ctx := context.Background()

	// -----------------------------------------------------------------------
	// Wire up repositories based on DB_MODE.
	// Handler and Service layers are identical for both modes.
	// -----------------------------------------------------------------------
	var (
		tenantRepo    repository.TenantRepository
		warehouseRepo repository.WarehouseRepository
		branchRepo    repository.BranchRepository
		itemRepo      repository.ItemRepository
		inventoryRepo repository.InventoryRepository
		userRepo      repository.UserRepository
		customerRepo  repository.CustomerRepository
		supplierRepo  repository.SupplierRepository
		transRepo     repository.TransactionRepository
		reportRepo    repository.ReportRepository
	)

	switch cfg.DBMode {
	case config.DBModeSingle:
		log.Println("[mode] single-db")

		pool, err := singledb.NewPool(ctx, cfg.SingleDSN)
		if err != nil {
			log.Fatalf("failed to connect to single DB: %v", err)
		}
		defer pool.Close()

		tenantRepo = repoSingle.NewTenantRepo(pool)
		warehouseRepo = repoSingle.NewWarehouseRepo(pool)
		branchRepo = repoSingle.NewBranchRepo(pool)
		itemRepo = repoSingle.NewItemRepo(pool)
		inventoryRepo = repoSingle.NewInventoryRepo(pool)
		userRepo = repoSingle.NewUserRepo(pool)
		customerRepo = repoSingle.NewCustomerRepo(pool)
		supplierRepo = repoSingle.NewSupplierRepo(pool)
		transRepo = repoSingle.NewTransactionRepo(pool)
		reportRepo = repoSingle.NewReportRepo(pool)

	case config.DBModeMulti:
		log.Println("[mode] multi-db")

		mgr, err := multidb.NewManager(
			ctx,
			cfg.MasterDSN,
			os.Getenv("TENANT_DB_HOST"),
			os.Getenv("TENANT_DB_PORT"),
		)
		if err != nil {
			log.Fatalf("failed to connect to master DB: %v", err)
		}
		defer mgr.Close()

		tenantRepo = repoMulti.NewTenantRepo(mgr)
		warehouseRepo = repoMulti.NewWarehouseRepo(mgr)
		branchRepo = repoMulti.NewBranchRepo(mgr)
		itemRepo = repoMulti.NewItemRepo(mgr)
		inventoryRepo = repoMulti.NewInventoryRepo(mgr)
		userRepo = repoMulti.NewUserRepo(mgr)
		customerRepo = repoMulti.NewCustomerRepo(mgr)
		supplierRepo = repoMulti.NewSupplierRepo(mgr)
		transRepo = repoMulti.NewTransactionRepo(mgr)
		reportRepo = repoMulti.NewReportRepo(mgr)

	default:
		log.Fatalf("unknown DB_MODE: %s", cfg.DBMode)
	}

	// -----------------------------------------------------------------------
	// Build service and handler layers (same for both modes).
	// -----------------------------------------------------------------------
	tenantSvc := service.NewTenantService(tenantRepo)
	warehouseSvc := service.NewWarehouseService(warehouseRepo)
	branchSvc := service.NewBranchService(branchRepo)
	itemSvc := service.NewItemService(itemRepo)
	inventorySvc := service.NewInventoryService(inventoryRepo)
	userSvc := service.NewUserService(userRepo, cfg.JWTSecret)
	customerSvc := service.NewCustomerService(customerRepo)
	supplierSvc := service.NewSupplierService(supplierRepo)
	transSvc := service.NewTransactionService(transRepo)
	reportSvc := service.NewReportService(reportRepo)

	handlers := api.Handlers{
		Tenant:    handler.NewTenantHandler(tenantSvc),
		Warehouse: handler.NewWarehouseHandler(warehouseSvc),
		Branch:    handler.NewBranchHandler(branchSvc),
		Item:      handler.NewItemHandler(itemSvc),
		Inventory: handler.NewInventoryHandler(inventorySvc),
		User:      handler.NewUserHandler(userSvc),
		Customer:  handler.NewCustomerHandler(customerSvc),
		Supplier:  handler.NewSupplierHandler(supplierSvc),
		Transaction: handler.NewTransactionHandler(transSvc),
		Report:    handler.NewReportHandler(reportSvc),
	}

	router := api.NewRouter(cfg.JWTSecret, cfg.Debug, handlers)

	// -----------------------------------------------------------------------
	// Start the HTTP server with graceful shutdown.
	// -----------------------------------------------------------------------
	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	go func() {
		log.Printf("server listening on :%s (DB_MODE=%s)", cfg.Port, cfg.DBMode)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down server…")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("graceful shutdown failed: %v", err)
	}
	log.Println("server stopped")
}
