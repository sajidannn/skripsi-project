#!/bin/bash
# ─── Multi-DB init script ─────────────────────────────────────────────────────
# Runs automatically by the Postgres Docker entrypoint on first start.
# 1. Applies master schema to pos_master (already created by POSTGRES_DB env).
# 2. Seeds master DB with tenant routing entries.
# 3. Creates pos_tenant1..3, applies tenant schema + seed to each.
set -e

TENANT_DBS=("pos_tenant1" "pos_tenant2" "pos_tenant3")

echo "[init-multi] Applying master schema to ${POSTGRES_DB}..."
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" \
    -f /docker-entrypoint-initdb.d/pos-multi-master.sql

echo "[init-multi] Seeding master DB..."
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" \
    -f /docker-entrypoint-initdb.d/seed-multi-master.sql

for tenant_db in "${TENANT_DBS[@]}"; do
    echo "[init-multi] Creating tenant database: ${tenant_db}..."
    psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" \
        -c "CREATE DATABASE ${tenant_db};"

    echo "[init-multi] Applying tenant schema to ${tenant_db}..."
    psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "${tenant_db}" \
        -f /docker-entrypoint-initdb.d/pos-multi-tenant.sql

    echo "[init-multi] Seeding ${tenant_db}..."
    psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "${tenant_db}" \
        -f /docker-entrypoint-initdb.d/seed-multi-tenant.sql
done

echo "[init-multi] Done. Tenant databases: ${TENANT_DBS[*]}"
