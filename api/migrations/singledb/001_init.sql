-- migrations/singledb/001_init.sql
-- Run this once against your shared database.

CREATE TABLE IF NOT EXISTS tenants (
    id         SERIAL PRIMARY KEY,
    name       TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS warehouses (
    id         SERIAL PRIMARY KEY,
    tenant_id  INT NOT NULL REFERENCES tenants(id),
    name       TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_warehouses_tenant ON warehouses(tenant_id);

CREATE TABLE IF NOT EXISTS branches (
    id              SERIAL PRIMARY KEY,
    tenant_id       INT NOT NULL REFERENCES tenants(id),
    phone           TEXT NOT NULL,
    name            TEXT NOT NULL,
    address         TEXT NOT NULL,
    opening_balance NUMERIC(14,2) NOT NULL DEFAULT 0,
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_branches_tenant ON branches(tenant_id);
