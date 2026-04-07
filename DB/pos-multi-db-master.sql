-- ─── Multi-DB master schema ──────────────────────────────────────────────────
-- Run this ONCE against the MASTER database (pos_master).
-- Stores tenant routing information: which DB / credentials each tenant uses.

CREATE TABLE IF NOT EXISTS tenants (
    id          SERIAL PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    db_name     TEXT NOT NULL,
    db_user     TEXT NOT NULL,
    db_password TEXT NOT NULL,
    created_at  TIMESTAMP DEFAULT NOW()
);
