-- migrations/multidb/002_tenant_schema.sql
-- Run this inside EACH TENANT's own database.
-- No tenant_id column — isolation is at the database level.

CREATE TABLE IF NOT EXISTS warehouses (
    id         SERIAL PRIMARY KEY,
    name       TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS branches (
    id              SERIAL PRIMARY KEY,
    phone           TEXT NOT NULL,
    name            TEXT NOT NULL,
    address         TEXT NOT NULL,
    opening_balance NUMERIC(14,2) NOT NULL DEFAULT 0,
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
