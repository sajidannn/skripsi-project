package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SeedMulti populates a multi-DB (per-tenant) Postgres setup.
// It uses the master DSN to:
//  1. Truncate + re-seed the master tenants routing table
//  2. For each tenant: DROP + CREATE database, apply tenant schema, bulk insert data
func SeedMulti(ctx context.Context, masterPool *pgxpool.Pool, masterDSN, tenantSchemaPath string, gen *Generator) error {
	log.Println("[multi] Truncating master tenants table...")
	if _, err := masterPool.Exec(ctx, `TRUNCATE TABLE tenants RESTART IDENTITY CASCADE`); err != nil {
		return fmt.Errorf("truncate master tenants: %w", err)
	}

	// Parse master DSN to derive tenant DSNs
	baseDSN := stripDBFromDSN(masterDSN) // e.g. postgres://user:pass@host:port

	// Read tenant schema SQL once
	tenantSchema, err := os.ReadFile(tenantSchemaPath)
	if err != nil {
		return fmt.Errorf("read tenant schema '%s': %w", tenantSchemaPath, err)
	}

	for _, t := range gen.Tenants {
		dbName := fmt.Sprintf("pos_t%03d", t.ID)
		dbUser := "postgres"
		dbPass := "supersecret"

		log.Printf("[multi] Tenant %d/%d — database: %s", t.ID, len(gen.Tenants), dbName)

		// Register tenant in master
		if _, err := masterPool.Exec(ctx,
			`INSERT INTO tenants (name, db_name, db_user, db_password) VALUES ($1, $2, $3, $4)`,
			t.Name, dbName, dbUser, dbPass,
		); err != nil {
			return fmt.Errorf("insert master tenant %d: %w", t.ID, err)
		}

		// Drop + recreate tenant database (requires superuser; connect to postgres)
		adminDSN := baseDSN + "/postgres"
		if err := recreateDB(ctx, adminDSN, dbName); err != nil {
			return fmt.Errorf("recreate db %s: %w", dbName, err)
		}

		// Connect to the new tenant database
		tenantDSN := baseDSN + "/" + dbName + "?sslmode=disable"
		tenantPool, err := waitForDB(ctx, tenantDSN)
		if err != nil {
			return fmt.Errorf("connect tenant %s: %w", dbName, err)
		}

		// Apply tenant schema
		if _, err := tenantPool.Exec(ctx, string(tenantSchema)); err != nil {
			tenantPool.Close()
			return fmt.Errorf("apply schema %s: %w", dbName, err)
		}

		// Seed tenant data
		if err := seedTenantDB(ctx, tenantPool, gen, t.ID); err != nil {
			tenantPool.Close()
			return fmt.Errorf("seed %s: %w", dbName, err)
		}

		// Reset sequences
		if err := resetSequencesTenant(ctx, tenantPool); err != nil {
			tenantPool.Close()
			return fmt.Errorf("reset seqs %s: %w", dbName, err)
		}

		tenantPool.Close()
		log.Printf("[multi]   ✓ %s seeded", dbName)
	}

	return nil
}

// seedTenantDB inserts all rows belonging to a single tenant into its own DB.
// No tenant_id column in multi-DB mode.
func seedTenantDB(ctx context.Context, pool *pgxpool.Pool, gen *Generator, tenantID int) error {
	ts := now()

	// ── Warehouses ────────────────────────────────────────────────────────────
	var whs []WarehouseRow
	for _, w := range gen.Warehouses {
		if w.TenantID == tenantID {
			whs = append(whs, w)
		}
	}
	whRows := make([][]any, len(whs))
	// We reset IDs to 1-based per tenant in multi-DB mode
	whIDMap := make(map[int]int, len(whs)) // global ID → local ID
	for i, w := range whs {
		localID := i + 1
		whIDMap[w.ID] = localID
		whRows[i] = []any{localID, w.Name, ts}
	}
	if _, err := pool.CopyFrom(ctx, pgx.Identifier{"warehouses"},
		[]string{"id", "name", "created_at"}, pgx.CopyFromRows(whRows)); err != nil {
		return fmt.Errorf("warehouses: %w", err)
	}

	// ── Branches ──────────────────────────────────────────────────────────────
	var brs []BranchRow
	for _, b := range gen.Branches {
		if b.TenantID == tenantID {
			brs = append(brs, b)
		}
	}
	brRows := make([][]any, len(brs))
	brIDMap := make(map[int]int, len(brs))
	for i, b := range brs {
		localID := i + 1
		brIDMap[b.ID] = localID
		brRows[i] = []any{localID, b.Phone, b.Name, b.Address, fmt.Sprintf("%d.00", b.OpeningBalance), ts}
	}
	if _, err := pool.CopyFrom(ctx, pgx.Identifier{"branches"},
		[]string{"id", "phone", "name", "address", "opening_balance", "created_at"}, pgx.CopyFromRows(brRows)); err != nil {
		return fmt.Errorf("branches: %w", err)
	}

	// ── Suppliers ─────────────────────────────────────────────────────────────
	var sups []SupplierRow
	for _, s := range gen.Suppliers {
		if s.TenantID == tenantID {
			sups = append(sups, s)
		}
	}
	supRows := make([][]any, len(sups))
	for i, s := range sups {
		supRows[i] = []any{i + 1, s.Name, s.Phone, s.Address, ts}
	}
	if _, err := pool.CopyFrom(ctx, pgx.Identifier{"suppliers"},
		[]string{"id", "name", "phone", "address", "created_at"}, pgx.CopyFromRows(supRows)); err != nil {
		return fmt.Errorf("suppliers: %w", err)
	}

	// ── Items ─────────────────────────────────────────────────────────────────
	var items []ItemRow
	for _, it := range gen.Items {
		if it.TenantID == tenantID {
			items = append(items, it)
		}
	}
	itemIDMap := make(map[int]int, len(items))
	itemRows := make([][]any, len(items))
	for i, it := range items {
		localID := i + 1
		itemIDMap[it.ID] = localID
		itemRows[i] = []any{localID, it.Name, it.SKU, fmt.Sprintf("%d.00", it.Cost), fmt.Sprintf("%d.00", it.Price), it.MarginThreshold, it.Description, ts, ts}
	}
	if _, err := pool.CopyFrom(ctx, pgx.Identifier{"items"},
		[]string{"id", "name", "sku", "cost", "price", "margin_threshold", "description", "created_at", "updated_at"}, pgx.CopyFromRows(itemRows)); err != nil {
		return fmt.Errorf("items: %w", err)
	}

	// ── Users ─────────────────────────────────────────────────────────────────
	var users []UserRow
	for _, u := range gen.Users {
		if u.TenantID == tenantID {
			users = append(users, u)
		}
	}
	userRows := make([][]any, len(users))
	for i, u := range users {
		userRows[i] = []any{i + 1, u.Name, u.Email, u.Password, u.Role, ts}
	}
	if _, err := pool.CopyFrom(ctx, pgx.Identifier{"users"},
		[]string{"id", "name", "email", "password", "role", "created_at"}, pgx.CopyFromRows(userRows)); err != nil {
		return fmt.Errorf("users: %w", err)
	}

	// ── Warehouse Items ────────────────────────────────────────────────────────
	var whItems []WarehouseItemRow
	for _, wi := range gen.WarehouseItems {
		if wi.TenantID == tenantID {
			whItems = append(whItems, wi)
		}
	}
	whItemRows := make([][]any, len(whItems))
	for i, wi := range whItems {
		whItemRows[i] = []any{i + 1, whIDMap[wi.WarehouseID], itemIDMap[wi.ItemID], wi.Stock, ts}
	}
	if _, err := pool.CopyFrom(ctx, pgx.Identifier{"warehouse_items"},
		[]string{"id", "warehouse_id", "item_id", "stock", "updated_at"}, pgx.CopyFromRows(whItemRows)); err != nil {
		return fmt.Errorf("warehouse_items: %w", err)
	}

	// ── Branch Items ───────────────────────────────────────────────────────────
	var brItems []BranchItemRow
	for _, bi := range gen.BranchItems {
		if bi.TenantID == tenantID {
			brItems = append(brItems, bi)
		}
	}
	brItemRows := make([][]any, len(brItems))
	for i, bi := range brItems {
		brItemRows[i] = []any{i + 1, brIDMap[bi.BranchID], itemIDMap[bi.ItemID], bi.Stock, bi.Price, bi.MarginThreshold, ts}
	}
	if _, err := pool.CopyFrom(ctx, pgx.Identifier{"branch_items"},
		[]string{"id", "branch_id", "item_id", "stock", "price", "margin_threshold", "updated_at"}, pgx.CopyFromRows(brItemRows)); err != nil {
		return fmt.Errorf("branch_items: %w", err)
	}

	// ── Customers ─────────────────────────────────────────────────────────────
	var custs []CustomerRow
	for _, c := range gen.Customers {
		if c.TenantID == tenantID {
			custs = append(custs, c)
		}
	}
	custRows := make([][]any, len(custs))
	for i, c := range custs {
		custRows[i] = []any{i + 1, brIDMap[c.BranchID], c.Name, c.Phone, c.Email, ts}
	}
	if _, err := pool.CopyFrom(ctx, pgx.Identifier{"customers"},
		[]string{"id", "branch_id", "name", "phone", "email", "created_at"}, pgx.CopyFromRows(custRows)); err != nil {
		return fmt.Errorf("customers: %w", err)
	}

	// ── Branch & Tenant Cashflow (opening balances) ────────────────────────────
	brCFRows := make([][]any, len(brs))
	var totalBalance int64
	for i, b := range brs {
		totalBalance += b.OpeningBalance
		brCFRows[i] = []any{brIDMap[b.ID], "ADJUSTMENT", "IN", fmt.Sprintf("%d.00", b.OpeningBalance), ts}
	}
	if _, err := pool.CopyFrom(ctx, pgx.Identifier{"branch_cashflow"},
		[]string{"branch_id", "flow_type", "direction", "amount", "created_at"}, pgx.CopyFromRows(brCFRows)); err != nil {
		return fmt.Errorf("branch_cashflow: %w", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO tenant_cashflow (flow_type, direction, amount) VALUES 
		('ADJUSTMENT', 'IN', $1), 
		('ADJUSTMENT', 'IN', '50000000000.00')`,
		fmt.Sprintf("%d.00", totalBalance),
	); err != nil {
		return fmt.Errorf("tenant_cashflow: %w", err)
	}

	return nil
}

// resetSequencesTenant resets all sequences in a tenant DB after seeding.
func resetSequencesTenant(ctx context.Context, pool *pgxpool.Pool) error {
	tables := []string{
		"warehouses", "branches", "suppliers", "items", "users", "customers",
		"warehouse_items", "branch_items", "transactions", "transaction_detail",
		"audit_stock", "branch_cashflow", "tenant_cashflow",
	}
	for _, tbl := range tables {
		q := fmt.Sprintf(`SELECT setval(pg_get_serial_sequence('%s','id'), COALESCE((SELECT MAX(id) FROM %s),0)+1, false)`, tbl, tbl)
		if _, err := pool.Exec(ctx, q); err != nil {
			return fmt.Errorf("reset seq %s: %w", tbl, err)
		}
	}
	return nil
}

// recreateDB drops (if exists) and creates the given database.
// Uses a separate one-shot connection because DROP/CREATE DATABASE cannot run
// inside a transaction.
func recreateDB(ctx context.Context, adminDSN, dbName string) error {
	conn, err := pgx.Connect(ctx, adminDSN)
	if err != nil {
		return fmt.Errorf("connect admin: %w", err)
	}
	defer conn.Close(ctx)

	// Terminate existing connections first
	if _, err := conn.Exec(ctx, fmt.Sprintf(
		`SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '%s' AND pid <> pg_backend_pid()`,
		dbName,
	)); err != nil {
		return err
	}

	if _, err := conn.Exec(ctx, fmt.Sprintf(`DROP DATABASE IF EXISTS "%s"`, dbName)); err != nil {
		return err
	}
	if _, err := conn.Exec(ctx, fmt.Sprintf(`CREATE DATABASE "%s"`, dbName)); err != nil {
		return err
	}
	return nil
}

// stripDBFromDSN removes the database name from a DSN, returning just
// the connection prefix. Handles both URL and key=value formats.
// e.g. "postgres://user:pass@host:5432/dbname?sslmode=disable"
//
//	→ "postgres://user:pass@host:5432"
func stripDBFromDSN(dsn string) string {
	// URL format
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		// Find last '/' after the host part
		afterScheme := strings.Index(dsn, "://") + 3
		slashIdx := strings.Index(dsn[afterScheme:], "/")
		if slashIdx >= 0 {
			base := dsn[:afterScheme+slashIdx]
			// strip query params from base if any leaked through
			return base
		}
		return dsn
	}
	// key=value format: strip dbname= field
	parts := strings.Fields(dsn)
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if !strings.HasPrefix(p, "dbname=") {
			result = append(result, p)
		}
	}
	return strings.Join(result, " ")
}
