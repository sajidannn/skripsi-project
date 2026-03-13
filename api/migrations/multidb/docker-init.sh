#!/usr/bin/env bash
# Docker init script for multi-DB mode.
# Runs inside the postgres container on first start.
#
# Creates:
#   pos_master    — master DB (tenants routing table)
#   pos_tenant_1  — tenant "alpha" database
#   pos_tenant_2  — tenant "beta"  database
#
# Postgres already creates the default database ($POSTGRES_DB = pos_master).
# We only need to create the additional tenant databases here.

set -e

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL

-- ── Master DB schema ────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS tenants (
    id          SERIAL PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    db_name     TEXT NOT NULL,
    db_user     TEXT NOT NULL,
    db_password TEXT NOT NULL,
    created_at  TIMESTAMP DEFAULT NOW()
);

-- Register tenant DBs.
-- db_host / db_port are intentionally omitted — the API falls back to
-- TENANT_DB_HOST / TENANT_DB_PORT env vars (set to this same Postgres container).
INSERT INTO tenants (name, db_name, db_user, db_password)
VALUES
    ('tenant-alpha', 'pos_tenant_1', '$POSTGRES_USER', '$POSTGRES_PASSWORD'),
    ('tenant-beta',  'pos_tenant_2', '$POSTGRES_USER', '$POSTGRES_PASSWORD')
ON CONFLICT (name) DO NOTHING;

EOSQL

# ── Create tenant databases ───────────────────────────────────────────────────
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" \
    -c "SELECT 'CREATE DATABASE pos_tenant_1' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'pos_tenant_1')\gexec"

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" \
    -c "SELECT 'CREATE DATABASE pos_tenant_2' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'pos_tenant_2')\gexec"

# ── Apply tenant schema to each tenant DB ────────────────────────────────────
for DB in pos_tenant_1 pos_tenant_2; do
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$DB" <<-EOSQL

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

EOSQL
done

echo "✓ multi-DB init complete: pos_master, pos_tenant_1, pos_tenant_2"
