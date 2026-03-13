#!/bin/bash
# ─── Single-DB init script ────────────────────────────────────────────────────
# Runs automatically by the Postgres Docker entrypoint on first start.
# Applies the full schema and seeds a few sample rows into pos_single.
set -e

echo "[init-single] Applying schema to ${POSTGRES_DB}..."
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" \
    -f /docker-entrypoint-initdb.d/pos-single-db.sql

echo "[init-single] Seeding sample data..."
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" \
    -f /docker-entrypoint-initdb.d/seed-single.sql

echo "[init-single] Done."
