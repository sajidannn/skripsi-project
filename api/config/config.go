package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

// DBMode defines which multi-tenancy strategy is active.
type DBMode string

const (
	DBModeSingle DBMode = "single" // single shared DB, tenant_id on every table
	DBModeMulti  DBMode = "multi"  // one DB per tenant, resolved from master DB
)

type Config struct {
	// Server
	Port string

	// DB mode: "single" | "multi"
	DBMode DBMode

	// Single-DB: one connection string
	SingleDSN string

	// Multi-DB: master DB connection string, tenant credentials fetched at runtime
	MasterDSN string

	// JWT
	JWTSecret string
}

// Load reads .env (if present) then falls back to OS environment variables.
func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, reading from OS env")
	}

	mode := DBMode(getEnv("DB_MODE", "single"))
	if mode != DBModeSingle && mode != DBModeMulti {
		log.Fatalf("DB_MODE must be 'single' or 'multi', got: %s", mode)
	}

	return &Config{
		Port:      getEnv("PORT", "8080"),
		DBMode:    mode,
		SingleDSN: getEnv("SINGLE_DSN", ""),
		MasterDSN: getEnv("MASTER_DSN", ""),
		JWTSecret: getEnv("JWT_SECRET", "change-me-in-production"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
