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
		warehouseRepo repository.WarehouseRepository
		branchRepo    repository.BranchRepository
	)

	switch cfg.DBMode {
	case config.DBModeSingle:
		log.Println("[mode] single-db")

		pool, err := singledb.NewPool(ctx, cfg.SingleDSN)
		if err != nil {
			log.Fatalf("failed to connect to single DB: %v", err)
		}
		defer pool.Close()

		warehouseRepo = repoSingle.NewWarehouseRepo(pool)
		branchRepo = repoSingle.NewBranchRepo(pool)

	case config.DBModeMulti:
		log.Println("[mode] multi-db")

		mgr, err := multidb.NewManager(
			ctx,
			cfg.MasterDSN,
			os.Getenv("TENANT_DB_HOST"), // PGBouncer / Postgres host for tenant pools
			os.Getenv("TENANT_DB_PORT"),
		)
		if err != nil {
			log.Fatalf("failed to connect to master DB: %v", err)
		}
		defer mgr.Close()

		warehouseRepo = repoMulti.NewWarehouseRepo(mgr)
		branchRepo = repoMulti.NewBranchRepo(mgr)

	default:
		log.Fatalf("unknown DB_MODE: %s", cfg.DBMode)
	}

	// -----------------------------------------------------------------------
	// Build service and handler layers (same for both modes).
	// -----------------------------------------------------------------------
	warehouseSvc := service.NewWarehouseService(warehouseRepo)
	branchSvc := service.NewBranchService(branchRepo)

	handlers := api.Handlers{
		Warehouse: handler.NewWarehouseHandler(warehouseSvc),
		Branch:    handler.NewBranchHandler(branchSvc),
	}

	router := api.NewRouter(cfg.JWTSecret, handlers)

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
