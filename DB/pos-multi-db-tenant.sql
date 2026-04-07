-- ─── Multi-DB per-tenant schema ──────────────────────────────────────────────
-- Run this inside EACH TENANT's own database.
-- No tenant_id column — isolation is achieved at the database level.

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

CREATE TABLE IF NOT EXISTS items (
    id          SERIAL PRIMARY KEY,
    name        TEXT NOT NULL,
    sku         TEXT NOT NULL UNIQUE,
    price       NUMERIC(12,2) NOT NULL,
    description TEXT,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_items_sku ON items(sku);

CREATE TABLE IF NOT EXISTS branch_items (
    id        SERIAL PRIMARY KEY,
    branch_id INT NOT NULL REFERENCES branches(id),
    item_id   INT NOT NULL REFERENCES items(id),
    stock     INT NOT NULL DEFAULT 0,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE (branch_id, item_id)
);
CREATE INDEX IF NOT EXISTS idx_branch_items_branch ON branch_items(branch_id);

CREATE TABLE IF NOT EXISTS warehouse_items (
    id           SERIAL PRIMARY KEY,
    warehouse_id INT NOT NULL REFERENCES warehouses(id),
    item_id      INT NOT NULL REFERENCES items(id),
    stock        INT NOT NULL DEFAULT 0,
    updated_at   TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE (warehouse_id, item_id)
);
CREATE INDEX IF NOT EXISTS idx_warehouse_items_warehouse ON warehouse_items(warehouse_id);

CREATE TABLE IF NOT EXISTS users (
    id         SERIAL PRIMARY KEY,
    name       TEXT NOT NULL,
    email      TEXT UNIQUE NOT NULL,
    password   TEXT NOT NULL,
    role       TEXT NOT NULL DEFAULT 'cashier',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS customers (
    id         SERIAL PRIMARY KEY,
    branch_id  INT NOT NULL REFERENCES branches(id),
    name       TEXT NOT NULL,
    phone      TEXT,
    email      TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_customers_branch ON customers(branch_id);

DO $$ BEGIN
    CREATE TYPE transaction_type AS ENUM ('SALE', 'PURC', 'TRANSFER');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE TABLE IF NOT EXISTS transactions (
    id           SERIAL PRIMARY KEY,
    trxno        VARCHAR(100) NOT NULL UNIQUE,
    branch_id    INT REFERENCES branches(id),
    warehouse_id INT REFERENCES warehouses(id),
    customer_id  INT REFERENCES customers(id),
    user_id      INT REFERENCES users(id),
    trans_type   transaction_type NOT NULL,
    total_amount NUMERIC(12,2) NOT NULL DEFAULT 0,
    note         TEXT,
    created_at   TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CHECK (branch_id IS NOT NULL OR warehouse_id IS NOT NULL)
);
CREATE INDEX IF NOT EXISTS idx_transactions_branch ON transactions(branch_id);
CREATE INDEX IF NOT EXISTS idx_transactions_warehouse ON transactions(warehouse_id);
CREATE INDEX IF NOT EXISTS idx_transactions_type ON transactions(trans_type);

CREATE TABLE IF NOT EXISTS transaction_detail (
    id                SERIAL PRIMARY KEY,
    transaction_id    INT NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
    branch_item_id    INT REFERENCES branch_items(id),
    warehouse_item_id INT REFERENCES warehouse_items(id),
    quantity          INT NOT NULL CHECK (quantity > 0),
    price             NUMERIC(12,2) NOT NULL,
    subtotal          NUMERIC(12,2) NOT NULL,
    CHECK (
        (branch_item_id IS NOT NULL AND warehouse_item_id IS NULL)
        OR
        (branch_item_id IS NULL  AND warehouse_item_id IS NOT NULL)
    )
);
CREATE INDEX IF NOT EXISTS idx_trx_detail ON transaction_detail(transaction_id);

CREATE TABLE IF NOT EXISTS audit_stock (
    id                SERIAL PRIMARY KEY,
    warehouse_item_id INT REFERENCES warehouse_items(id),
    branch_item_id    INT REFERENCES branch_items(id),
    change_unit       INT NOT NULL,
    reason            TEXT,
    created_at        TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

DO $$ BEGIN
    CREATE TYPE cashflow_type AS ENUM ('SALE', 'TRANSFER', 'ADJUSTMENT');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE TABLE IF NOT EXISTS branch_cashflow (
    id             SERIAL PRIMARY KEY,
    branch_id      INT NOT NULL REFERENCES branches(id),
    transaction_id INT REFERENCES transactions(id),
    flow_type      cashflow_type NOT NULL,
    direction      CHAR(3) CHECK (direction IN ('IN','OUT')),
    amount         NUMERIC(14,2) NOT NULL CHECK (amount > 0),
    created_at     TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_branch_cashflow_branch ON branch_cashflow(branch_id);

DO $$ BEGIN
    CREATE TYPE tenant_flow_type AS ENUM ('SALE', 'PURC', 'WITHDRAW', 'ADJUSTMENT');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE TABLE IF NOT EXISTS tenant_cashflow (
    id             SERIAL PRIMARY KEY,
    transaction_id INT REFERENCES transactions(id),
    flow_type      tenant_flow_type NOT NULL,
    direction      CHAR(3) CHECK (direction IN ('IN','OUT')),
    amount         NUMERIC(14,2) NOT NULL CHECK (amount > 0),
    created_at     TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
