-- migrations/multidb/001_master.sql
-- Run this once against the MASTER DB.
-- Stores tenant routing information (db credentials per tenant).

CREATE TABLE IF NOT EXISTS tenants (
    id          SERIAL PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    db_name     TEXT NOT NULL,
    db_user     TEXT NOT NULL,
    db_password TEXT NOT NULL,
    created_at  TIMESTAMP DEFAULT NOW()
);
