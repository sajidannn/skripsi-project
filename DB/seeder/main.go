package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	// ── CLI flags ────────────────────────────────────────────────────────────
	mode  := flag.String("mode",  "", `DB mode: "single" or "multi" (required)`)
	scale := flag.String("scale", "", `Data scale: "small", "medium", or "large" (required)`)
	dsn   := flag.String("dsn",   "", "PostgreSQL DSN, e.g. postgres://user:pass@host:5432/dbname?sslmode=disable")
	schemaPath := flag.String("tenant-schema", "/app/schema/multi-db-tenant.sql", "Path to tenant schema SQL file (multi mode only)")
	flag.Parse()

	// Read from env vars as fallback (Docker Compose sets these)
	if *mode == "" {
		*mode = os.Getenv("SEED_MODE")
	}
	if *scale == "" {
		*scale = os.Getenv("SEED_SCALE")
	}
	if *dsn == "" {
		*dsn = os.Getenv("DATABASE_URL")
	}
	if s := os.Getenv("TENANT_SCHEMA_PATH"); s != "" {
		*schemaPath = s
	}

	// ── Validate ─────────────────────────────────────────────────────────────
	if *mode != "single" && *mode != "multi" {
		log.Fatalf("ERROR: -mode must be 'single' or 'multi' (got: %q)", *mode)
	}
	cfg, ok := scales[*scale]
	if !ok {
		log.Fatalf("ERROR: -scale must be 'small', 'medium', or 'large' (got: %q)", *scale)
	}
	if *dsn == "" {
		log.Fatal("ERROR: -dsn or DATABASE_URL environment variable is required")
	}

	// ── Pre-compute bcrypt hashes (done ONCE for performance) ─────────────
	log.Println("[seeder] Pre-computing bcrypt hashes...")
	ownerHash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("bcrypt owner: %v", err)
	}
	cashierHash, err := bcrypt.GenerateFromPassword([]byte("cashier123"), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("bcrypt cashier: %v", err)
	}

	// ── Generate data ─────────────────────────────────────────────────────
	log.Printf("[seeder] Generating data for scale=%s mode=%s ...", cfg.Name, *mode)
	startGen := time.Now()
	gen := NewGenerator(cfg, string(ownerHash), string(cashierHash))
	gen.Generate()
	log.Printf("[seeder] Data generation done in %s", time.Since(startGen).Round(time.Millisecond))

	// Print summary
	counts := cfg.derivedCounts()
	log.Println("[seeder] ──── Data summary ────────────────────────────────")
	log.Printf("[seeder]   Scale          : %s", cfg.Name)
	log.Printf("[seeder]   Mode           : %s", *mode)
	log.Printf("[seeder]   tenants        : %d", counts["tenants"])
	log.Printf("[seeder]   warehouses     : %d", counts["warehouses"])
	log.Printf("[seeder]   branches       : %d", counts["branches"])
	log.Printf("[seeder]   items          : %d", counts["items"])
	log.Printf("[seeder]   suppliers      : %d", counts["suppliers"])
	log.Printf("[seeder]   users          : %d", counts["users"])
	log.Printf("[seeder]   warehouse_items: %d", counts["warehouse_items"])
	log.Printf("[seeder]   branch_items   : %d (est)", counts["branch_items"])
	log.Printf("[seeder]   customers      : %d", counts["customers"])
	log.Printf("[seeder]   Total estimated: ~%d rows", estimateTotal(counts))
	log.Println("[seeder] ─────────────────────────────────────────────────")

	// ── Connect ───────────────────────────────────────────────────────────
	ctx := context.Background()
	log.Printf("[seeder] Connecting to database...")
	pool, err := waitForDB(ctx, *dsn)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer pool.Close()
	log.Println("[seeder] Connected!")

	// ── Seed ─────────────────────────────────────────────────────────────
	startSeed := time.Now()
	switch *mode {
	case "single":
		if err := SeedSingle(ctx, pool, gen); err != nil {
			log.Fatalf("[seeder] single seed failed: %v", err)
		}
		log.Println("[seeder] Resetting sequences...")
		if err := resetSequencesSingle(ctx, pool); err != nil {
			log.Fatalf("[seeder] reset sequences failed: %v", err)
		}
	case "multi":
		if err := SeedMulti(ctx, pool, *dsn, *schemaPath, gen); err != nil {
			log.Fatalf("[seeder] multi seed failed: %v", err)
		}
	}

	elapsed := time.Since(startSeed).Round(time.Millisecond)
	log.Printf("[seeder] ✓ Seeding complete in %s!", elapsed)
	fmt.Printf("\n=== SEEDER DONE ===\nScale: %s | Mode: %s | Duration: %s\n\n", cfg.Name, *mode, elapsed)
}

func estimateTotal(counts map[string]int) int {
	total := 0
	for _, v := range counts {
		total += v
	}
	return total
}
