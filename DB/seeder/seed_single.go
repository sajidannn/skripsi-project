package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SeedSingle populates a single-DB (shared-schema) Postgres database.
// It TRUNCATES all tables first (cascade), then bulk-inserts all data.
func SeedSingle(ctx context.Context, pool *pgxpool.Pool, gen *Generator) error {
	log.Println("[single] Truncating all tables...")
	if err := truncateSingle(ctx, pool); err != nil {
		return fmt.Errorf("truncate: %w", err)
	}

	log.Println("[single] Inserting tenants...")
	if err := insertTenantsSingle(ctx, pool, gen); err != nil {
		return fmt.Errorf("tenants: %w", err)
	}

	log.Println("[single] Inserting warehouses...")
	if err := insertWarehousesSingle(ctx, pool, gen); err != nil {
		return fmt.Errorf("warehouses: %w", err)
	}

	log.Println("[single] Inserting branches...")
	if err := insertBranchesSingle(ctx, pool, gen); err != nil {
		return fmt.Errorf("branches: %w", err)
	}

	log.Println("[single] Inserting suppliers...")
	if err := insertSuppliersSingle(ctx, pool, gen); err != nil {
		return fmt.Errorf("suppliers: %w", err)
	}

	log.Println("[single] Inserting items...")
	if err := insertItemsSingle(ctx, pool, gen); err != nil {
		return fmt.Errorf("items: %w", err)
	}

	log.Println("[single] Inserting users...")
	if err := insertUsersSingle(ctx, pool, gen); err != nil {
		return fmt.Errorf("users: %w", err)
	}

	log.Println("[single] Inserting warehouse_items...")
	if err := insertWarehouseItemsSingle(ctx, pool, gen); err != nil {
		return fmt.Errorf("warehouse_items: %w", err)
	}

	log.Println("[single] Inserting branch_items...")
	if err := insertBranchItemsSingle(ctx, pool, gen); err != nil {
		return fmt.Errorf("branch_items: %w", err)
	}

	log.Println("[single] Inserting customers...")
	if err := insertCustomersSingle(ctx, pool, gen); err != nil {
		return fmt.Errorf("customers: %w", err)
	}

	log.Println("[single] Inserting tenant_cashflow (opening balances)...")
	if err := insertTenantCashflowSingle(ctx, pool, gen); err != nil {
		return fmt.Errorf("tenant_cashflow: %w", err)
	}

	return nil
}

// ─── Truncate ─────────────────────────────────────────────────────────────────

func truncateSingle(ctx context.Context, pool *pgxpool.Pool) error {
	// Order matters: child tables first, or use CASCADE
	_, err := pool.Exec(ctx, `
		TRUNCATE TABLE
			tenant_cashflow,
			branch_cashflow,
			audit_stock,
			transaction_detail,
			transactions,
			customers,
			users,
			branch_items,
			warehouse_items,
			items,
			suppliers,
			branches,
			warehouses,
			tenants
		RESTART IDENTITY CASCADE
	`)
	return err
}

// ─── Insert functions (using pgx CopyFrom for bulk performance) ───────────────

func insertTenantsSingle(ctx context.Context, pool *pgxpool.Pool, gen *Generator) error {
	ts := now()
	rows := make([][]any, len(gen.Tenants))
	for i, t := range gen.Tenants {
		rows[i] = []any{t.ID, t.Name, ts}
	}
	_, err := pool.CopyFrom(ctx,
		pgx.Identifier{"tenants"},
		[]string{"id", "name", "created_at"},
		pgx.CopyFromRows(rows),
	)
	return err
}

func insertWarehousesSingle(ctx context.Context, pool *pgxpool.Pool, gen *Generator) error {
	ts := now()
	rows := make([][]any, len(gen.Warehouses))
	for i, w := range gen.Warehouses {
		rows[i] = []any{w.ID, w.TenantID, w.Name, ts}
	}
	_, err := pool.CopyFrom(ctx,
		pgx.Identifier{"warehouses"},
		[]string{"id", "tenant_id", "name", "created_at"},
		pgx.CopyFromRows(rows),
	)
	return err
}

func insertBranchesSingle(ctx context.Context, pool *pgxpool.Pool, gen *Generator) error {
	ts := now()
	rows := make([][]any, len(gen.Branches))
	for i, b := range gen.Branches {
		rows[i] = []any{b.ID, b.TenantID, b.Phone, b.Name, b.Address, fmt.Sprintf("%d.00", b.OpeningBalance), ts}
	}
	_, err := pool.CopyFrom(ctx,
		pgx.Identifier{"branches"},
		[]string{"id", "tenant_id", "phone", "name", "address", "opening_balance", "created_at"},
		pgx.CopyFromRows(rows),
	)
	return err
}

func insertSuppliersSingle(ctx context.Context, pool *pgxpool.Pool, gen *Generator) error {
	ts := now()
	rows := make([][]any, len(gen.Suppliers))
	for i, s := range gen.Suppliers {
		rows[i] = []any{s.ID, s.TenantID, s.Name, s.Phone, s.Address, ts}
	}
	_, err := pool.CopyFrom(ctx,
		pgx.Identifier{"suppliers"},
		[]string{"id", "tenant_id", "name", "phone", "address", "created_at"},
		pgx.CopyFromRows(rows),
	)
	return err
}

func insertItemsSingle(ctx context.Context, pool *pgxpool.Pool, gen *Generator) error {
	ts := now()
	rows := make([][]any, len(gen.Items))
	for i, it := range gen.Items {
		rows[i] = []any{
			it.ID, it.TenantID, it.Name, it.SKU,
			fmt.Sprintf("%d.00", it.Cost),
			fmt.Sprintf("%d.00", it.Price),
			it.MarginThreshold,
			it.Description, ts, ts,
		}
	}
	_, err := pool.CopyFrom(ctx,
		pgx.Identifier{"items"},
		[]string{"id", "tenant_id", "name", "sku", "cost", "price", "margin_threshold", "description", "created_at", "updated_at"},
		pgx.CopyFromRows(rows),
	)
	return err
}

func insertUsersSingle(ctx context.Context, pool *pgxpool.Pool, gen *Generator) error {
	ts := now()
	rows := make([][]any, len(gen.Users))
	for i, u := range gen.Users {
		rows[i] = []any{u.ID, u.TenantID, u.Name, u.Email, u.Password, u.Role, ts}
	}
	_, err := pool.CopyFrom(ctx,
		pgx.Identifier{"users"},
		[]string{"id", "tenant_id", "name", "email", "password", "role", "created_at"},
		pgx.CopyFromRows(rows),
	)
	return err
}

func insertWarehouseItemsSingle(ctx context.Context, pool *pgxpool.Pool, gen *Generator) error {
	ts := now()
	const batchSize = 50_000
	for start := 0; start < len(gen.WarehouseItems); start += batchSize {
		end := start + batchSize
		if end > len(gen.WarehouseItems) {
			end = len(gen.WarehouseItems)
		}
		batch := gen.WarehouseItems[start:end]
		rows := make([][]any, len(batch))
		for i, wi := range batch {
			rows[i] = []any{wi.ID, wi.TenantID, wi.WarehouseID, wi.ItemID, wi.Stock, ts}
		}
		_, err := pool.CopyFrom(ctx,
			pgx.Identifier{"warehouse_items"},
			[]string{"id", "tenant_id", "warehouse_id", "item_id", "stock", "updated_at"},
			pgx.CopyFromRows(rows),
		)
		if err != nil {
			return err
		}
		log.Printf("[single]   warehouse_items batch %d/%d done", end, len(gen.WarehouseItems))
	}
	return nil
}

func insertBranchItemsSingle(ctx context.Context, pool *pgxpool.Pool, gen *Generator) error {
	ts := now()
	const batchSize = 50_000
	for start := 0; start < len(gen.BranchItems); start += batchSize {
		end := start + batchSize
		if end > len(gen.BranchItems) {
			end = len(gen.BranchItems)
		}
		batch := gen.BranchItems[start:end]
		rows := make([][]any, len(batch))
		for i, bi := range batch {
			rows[i] = []any{bi.ID, bi.TenantID, bi.BranchID, bi.ItemID, bi.Stock, bi.Price, bi.MarginThreshold, ts}
		}
		_, err := pool.CopyFrom(ctx,
			pgx.Identifier{"branch_items"},
			[]string{"id", "tenant_id", "branch_id", "item_id", "stock", "price", "margin_threshold", "updated_at"},
			pgx.CopyFromRows(rows),
		)
		if err != nil {
			return err
		}
		log.Printf("[single]   branch_items batch %d/%d done", end, len(gen.BranchItems))
	}
	return nil
}

func insertCustomersSingle(ctx context.Context, pool *pgxpool.Pool, gen *Generator) error {
	ts := now()
	const batchSize = 50_000
	for start := 0; start < len(gen.Customers); start += batchSize {
		end := start + batchSize
		if end > len(gen.Customers) {
			end = len(gen.Customers)
		}
		batch := gen.Customers[start:end]
		rows := make([][]any, len(batch))
		for i, c := range batch {
			rows[i] = []any{c.ID, c.TenantID, c.BranchID, c.Name, c.Phone, c.Email, ts}
		}
		_, err := pool.CopyFrom(ctx,
			pgx.Identifier{"customers"},
			[]string{"id", "tenant_id", "branch_id", "name", "phone", "email", "created_at"},
			pgx.CopyFromRows(rows),
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func insertTenantCashflowSingle(ctx context.Context, pool *pgxpool.Pool, gen *Generator) error {
	ts := now()
	// Insert opening balance adjustment for each branch (as branch cashflow)
	branchRows := make([][]any, len(gen.Branches))
	for i, br := range gen.Branches {
		branchRows[i] = []any{br.TenantID, br.ID, "ADJUSTMENT", "IN", fmt.Sprintf("%d.00", br.OpeningBalance), ts}
	}
	_, err := pool.CopyFrom(ctx,
		pgx.Identifier{"branch_cashflow"},
		[]string{"tenant_id", "branch_id", "flow_type", "direction", "amount", "created_at"},
		pgx.CopyFromRows(branchRows),
	)
	if err != nil {
		return fmt.Errorf("branch_cashflow: %w", err)
	}

	// Aggregate opening balance per tenant for tenant_cashflow
	tenantBalance := make(map[int]int64)
	for _, br := range gen.Branches {
		tenantBalance[br.TenantID] += br.OpeningBalance
	}
	tenantRows := make([][]any, 0, len(gen.Tenants))
	for _, t := range gen.Tenants {
		tenantRows = append(tenantRows, []any{t.ID, "ADJUSTMENT", "IN", fmt.Sprintf("%d.00", tenantBalance[t.ID]), ts})
	}
	_, err = pool.CopyFrom(ctx,
		pgx.Identifier{"tenant_cashflow"},
		[]string{"tenant_id", "flow_type", "direction", "amount", "created_at"},
		pgx.CopyFromRows(tenantRows),
	)
	return err
}

// ─── Sequence reset ───────────────────────────────────────────────────────────

// resetSequencesSingle resets all sequences to the current max(id)+1 so that
// subsequent API inserts don't collide with the seeded IDs.
func resetSequencesSingle(ctx context.Context, pool *pgxpool.Pool) error {
	tables := []string{
		"tenants", "warehouses", "branches", "suppliers", "items",
		"users", "customers", "warehouse_items", "branch_items",
		"transactions", "transaction_detail", "audit_stock",
		"branch_cashflow", "tenant_cashflow",
	}
	for _, tbl := range tables {
		q := fmt.Sprintf(`SELECT setval(pg_get_serial_sequence('%s','id'), COALESCE((SELECT MAX(id) FROM %s),0)+1, false)`, tbl, tbl)
		if _, err := pool.Exec(ctx, q); err != nil {
			return fmt.Errorf("reset seq %s: %w", tbl, err)
		}
	}
	return nil
}

func waitForDB(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	for {
		pool, err := pgxpool.NewWithConfig(ctx, cfg)
		if err == nil {
			if pingErr := pool.Ping(ctx); pingErr == nil {
				return pool, nil
			}
			pool.Close()
		}
		log.Println("[seeder] Waiting for database to be ready...")
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}
