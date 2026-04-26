-- ─── Single-DB schema ────────────────────────────────────────────────────────
-- All tenants share ONE Postgres database.
-- Every table carries a tenant_id foreign key for row-level isolation.
-- Run: mounted to /docker-entrypoint-initdb.d/ → auto-applied on first start.

CREATE TABLE IF NOT EXISTS tenants (
    id          SERIAL PRIMARY KEY,
    name        TEXT NOT NULL,
    created_at  TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS warehouses (
    id          SERIAL PRIMARY KEY,
    tenant_id   INT NOT NULL REFERENCES tenants(id),
    name        TEXT NOT NULL,
    created_at  TIMESTAMP DEFAULT NOW()
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

CREATE TABLE IF NOT EXISTS suppliers (
    id          SERIAL PRIMARY KEY,
    tenant_id   INT NOT NULL REFERENCES tenants(id),
    name        TEXT NOT NULL,
    phone       TEXT,
    address     TEXT,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_suppliers_tenant ON suppliers(tenant_id);

CREATE TABLE IF NOT EXISTS items (
    id          SERIAL PRIMARY KEY,
    tenant_id   INT NOT NULL REFERENCES tenants(id),
    name        TEXT NOT NULL,
    sku         TEXT NOT NULL,
    cost        NUMERIC(12,2) NOT NULL DEFAULT 0,
    price       NUMERIC(12,2) NOT NULL DEFAULT 0,
    margin_threshold NUMERIC(5,2) NOT NULL DEFAULT 10.00,
    description TEXT,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE (tenant_id, sku)
);
CREATE INDEX IF NOT EXISTS idx_items_tenant ON items(tenant_id);

CREATE TABLE IF NOT EXISTS branch_items (
    id          SERIAL PRIMARY KEY,
    tenant_id   INT NOT NULL REFERENCES tenants(id),
    branch_id   INT NOT NULL REFERENCES branches(id),
    item_id     INT NOT NULL REFERENCES items(id),
    stock       INT NOT NULL DEFAULT 0,
    price       NUMERIC(12,2) DEFAULT NULL,
    margin_threshold NUMERIC(5,2) DEFAULT NULL,
    updated_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE (tenant_id, branch_id, item_id)
);
CREATE INDEX IF NOT EXISTS idx_branch_items_tenant_branch ON branch_items(tenant_id, branch_id, item_id);

CREATE TABLE IF NOT EXISTS warehouse_items (
    id           SERIAL PRIMARY KEY,
    tenant_id    INT NOT NULL REFERENCES tenants(id),
    warehouse_id INT NOT NULL REFERENCES warehouses(id),
    item_id      INT NOT NULL REFERENCES items(id),
    stock        INT NOT NULL DEFAULT 0,
    updated_at   TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE (tenant_id, warehouse_id, item_id)
);
CREATE INDEX IF NOT EXISTS idx_warehouse_items_tenant_warehouse ON warehouse_items(tenant_id, warehouse_id, item_id);

CREATE TABLE IF NOT EXISTS users (
    id          SERIAL PRIMARY KEY,
    tenant_id   INT NOT NULL REFERENCES tenants(id),
    name        TEXT NOT NULL,
    email       TEXT NOT NULL,
    password    TEXT NOT NULL,
    role        TEXT NOT NULL DEFAULT 'cashier' CHECK (role IN ('owner', 'manager', 'cashier')),
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE (tenant_id, email)
);
CREATE INDEX IF NOT EXISTS idx_users_tenant ON users(tenant_id);

CREATE TABLE IF NOT EXISTS customers (
    id          SERIAL PRIMARY KEY,
    tenant_id   INT NOT NULL REFERENCES tenants(id),
    branch_id   INT NOT NULL REFERENCES branches(id),
    name        TEXT NOT NULL,
    phone       TEXT,
    email       TEXT,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_customers_tenant_branch ON customers(tenant_id, branch_id);

DO $$ BEGIN
    CREATE TYPE transaction_type AS ENUM ('SALE', 'PURC', 'TRANSFER', 'RETURN', 'RETURN_PURC', 'VOID');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE TABLE IF NOT EXISTS transactions (
    id           SERIAL PRIMARY KEY,
    tenant_id    INT NOT NULL REFERENCES tenants(id),
    trxno        VARCHAR(100) NOT NULL,
    branch_id    INT REFERENCES branches(id),
    warehouse_id INT REFERENCES warehouses(id),
    customer_id  INT REFERENCES customers(id),
    supplier_id  INT REFERENCES suppliers(id),
    user_id      INT REFERENCES users(id),
    trans_type   transaction_type NOT NULL,
    total_amount NUMERIC(12,2) NOT NULL DEFAULT 0,
    tax          NUMERIC(12,2) NOT NULL DEFAULT 0,
    discount     NUMERIC(12,2) NOT NULL DEFAULT 0,
    reference_transaction_id INT REFERENCES transactions(id),
    note         TEXT,
    created_at   TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE (tenant_id, trxno),
    CHECK (branch_id IS NOT NULL OR warehouse_id IS NOT NULL)
);
CREATE INDEX IF NOT EXISTS idx_transactions_tenant_branch_created ON transactions(tenant_id, branch_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_transactions_tenant_warehouse_created ON transactions(tenant_id, warehouse_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_transactions_type ON transactions(trans_type);

CREATE TABLE IF NOT EXISTS transaction_detail (
    id                SERIAL PRIMARY KEY,
    transaction_id    INT NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
    branch_item_id    INT REFERENCES branch_items(id),
    warehouse_item_id INT REFERENCES warehouse_items(id),
    quantity          INT NOT NULL CHECK (quantity > 0),
    cogs              NUMERIC(12,2) NOT NULL DEFAULT 0,
    price             NUMERIC(12,2) NOT NULL DEFAULT 0,
    subtotal          NUMERIC(12,2) NOT NULL DEFAULT 0,
    CHECK (
        (branch_item_id IS NOT NULL AND warehouse_item_id IS NULL)
        OR
        (branch_item_id IS NULL  AND warehouse_item_id IS NOT NULL)
    )
);
CREATE INDEX IF NOT EXISTS idx_trx_detail ON transaction_detail(transaction_id);

CREATE TABLE IF NOT EXISTS audit_stock (
    id                SERIAL PRIMARY KEY,
    tenant_id         INT NOT NULL REFERENCES tenants(id),
    warehouse_item_id INT REFERENCES warehouse_items(id),
    branch_item_id    INT REFERENCES branch_items(id),
    change_unit       INT NOT NULL,
    reason            TEXT,
    user_id           INT REFERENCES users(id),
    created_at        TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_audit_stock_tenant_created ON audit_stock(tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_stock_user ON audit_stock(user_id);

DO $$ BEGIN
    CREATE TYPE cashflow_type AS ENUM ('SALE', 'TRANSFER', 'ADJUSTMENT', 'RETURN', 'VOID', 'WITHDRAW');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE TABLE IF NOT EXISTS branch_cashflow (
    id             SERIAL PRIMARY KEY,
    tenant_id      INT NOT NULL REFERENCES tenants(id),
    branch_id      INT NOT NULL REFERENCES branches(id),
    transaction_id INT REFERENCES transactions(id),
    flow_type      cashflow_type NOT NULL,
    direction      CHAR(3) CHECK (direction IN ('IN','OUT')),
    amount         NUMERIC(14,2) NOT NULL CHECK (amount > 0),
    note           TEXT,
    created_at     TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_branch_cashflow_tenant_branch_created ON branch_cashflow(tenant_id, branch_id, created_at DESC);

DO $$ BEGIN
    CREATE TYPE tenant_flow_type AS ENUM ('SALE', 'PURC', 'WITHDRAW', 'ADJUSTMENT', 'RETURN', 'RETURN_PURC');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE TABLE IF NOT EXISTS tenant_cashflow (
    id             SERIAL PRIMARY KEY,
    tenant_id      INT NOT NULL REFERENCES tenants(id),
    transaction_id INT REFERENCES transactions(id),
    flow_type      tenant_flow_type NOT NULL,
    direction      CHAR(3) CHECK (direction IN ('IN','OUT')),
    amount         NUMERIC(14,2) NOT NULL CHECK (amount > 0),
    note           TEXT,
    created_at     TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_tenant_cashflow_tenant_created ON tenant_cashflow(tenant_id, created_at DESC);
