// Package multidb provides a per-tenant connection pool manager used when the
// API runs in "multi" DB mode.  Each tenant has its own PostgreSQL database;
// credentials are kept in the master DB (tenants table).
//
// Flow per request:
//
//	JWT verified → tenant_id extracted → Manager.Pool(tenantID) →
//	  look up master DB → build DSN → create / reuse pgxpool → execute query
package multidb

import (
	"context"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TenantDB holds connection details fetched from the master tenants table.
type TenantDB struct {
	DBName string
	DBUser string
	DBPass string
	DBHost string // defaults to the master host when empty
	DBPort string // defaults to "5432"
}

// Manager maintains one pgxpool per tenant, lazily created and cached.
type Manager struct {
	master *pgxpool.Pool
	mu     sync.RWMutex
	pools  map[int]*pgxpool.Pool
	// Host / port used to build per-tenant DSNs.  In a PGBouncer setup this
	// typically points to the PGBouncer endpoint.
	dbHost string
	dbPort string
}

// NewManager connects to the master DB and returns a Manager.
//
// masterDSN example: postgres://admin:secret@localhost:5432/pos_master?sslmode=disable
// pgbouncerHost / pgbouncerPort are forwarded to pgxpool DSNs built for each tenant.
// Leave them empty to fall back to the master DSN host/port.
func NewManager(ctx context.Context, masterDSN, tenantHost, tenantPort string) (*Manager, error) {
	if masterDSN == "" {
		return nil, fmt.Errorf("multidb: MASTER_DSN is not set")
	}

	master, err := pgxpool.New(ctx, masterDSN)
	if err != nil {
		return nil, fmt.Errorf("multidb: failed to connect to master DB: %w", err)
	}

	if err := master.Ping(ctx); err != nil {
		master.Close()
		return nil, fmt.Errorf("multidb: failed to ping master DB: %w", err)
	}

	if tenantHost == "" {
		tenantHost = "localhost"
	}
	if tenantPort == "" {
		tenantPort = "5432"
	}

	return &Manager{
		master: master,
		pools:  make(map[int]*pgxpool.Pool),
		dbHost: tenantHost,
		dbPort: tenantPort,
	}, nil
}

// Pool returns (and lazily creates) the pgxpool for the given tenantID.
// On first call for a tenant it queries the master DB for credentials.
func (m *Manager) Pool(ctx context.Context, tenantID int) (*pgxpool.Pool, error) {
	// fast-path: pool already exists
	m.mu.RLock()
	if p, ok := m.pools[tenantID]; ok {
		m.mu.RUnlock()
		return p, nil
	}
	m.mu.RUnlock()

	// slow-path: fetch credentials & create pool
	m.mu.Lock()
	defer m.mu.Unlock()

	// double-check after acquiring write lock
	if p, ok := m.pools[tenantID]; ok {
		return p, nil
	}

	tenant, err := m.fetchTenantDB(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	host := m.dbHost
	port := m.dbPort
	if tenant.DBHost != "" {
		host = tenant.DBHost
	}
	if tenant.DBPort != "" {
		port = tenant.DBPort
	}

	// Keep per-tenant pool small: for a thesis load-test with ≤500 tenants,
	// 2 min / 4 max connections per tenant is sufficient and avoids exhausting
	// Postgres max_connections without needing PGBouncer.
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable&pool_min_conns=2&pool_max_conns=4",
		tenant.DBUser, tenant.DBPass, host, port, tenant.DBName,
	)

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("multidb: failed to create pool for tenant %d: %w", tenantID, err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("multidb: failed to ping tenant %d DB: %w", tenantID, err)
	}

	m.pools[tenantID] = pool
	return pool, nil
}

// Close closes all managed pools including the master pool.
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range m.pools {
		p.Close()
	}
	m.master.Close()
}

// fetchTenantDB queries the master tenants table for the given tenantID.
func (m *Manager) fetchTenantDB(ctx context.Context, tenantID int) (*TenantDB, error) {
	var t TenantDB
	row := m.master.QueryRow(ctx,
		`SELECT db_name, db_user, db_password FROM tenants WHERE id = $1`,
		tenantID,
	)
	if err := row.Scan(&t.DBName, &t.DBUser, &t.DBPass); err != nil {
		return nil, fmt.Errorf("multidb: tenant %d not found in master DB: %w", tenantID, err)
	}
	return &t, nil
}
