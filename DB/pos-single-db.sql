CREATE TABLE tenants (
    id          SERIAL PRIMARY KEY,
    name        TEXT NOT NULL,
    created_at  TIMESTAMP DEFAULT NOW()
);

CREATE TABLE warehouses (
    id          SERIAL PRIMARY KEY,
    tenant_id   INT NOT NULL REFERENCES tenants(id),
    name        TEXT NOT NULL,
    created_at  TIMESTAMP DEFAULT NOW()
);
CREATE INDEX idx_warehouses_tenant ON warehouses(tenant_id);

CREATE TABLE branches (
    id            SERIAL PRIMARY KEY,
    tenant_id     INT NOT NULL REFERENCES tenants(id),
    phone         TEXT NOT NULL,
    name          TEXT NOT NULL,
    address       TEXT NOT NULL,
    opening_balance NUMERIC(14,2) NOT NULL DEFAULT 0,
    created_at    TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
CREATE INDEX idx_branches_tenant ON branches(tenant_id);

CREATE TABLE items (
    id          SERIAL PRIMARY KEY,
    tenant_id   INT NOT NULL REFERENCES tenants(id),
    name        TEXT NOT NULL,
    sku         TEXT,
    price       NUMERIC(12,2) NOT NULL,
    description TEXT,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE (tenant_id, sku)
);
CREATE INDEX idx_items_tenant ON items(tenant_id);

CREATE TABLE branch_items (
    id          SERIAL PRIMARY KEY,
    tenant_id   INT NOT NULL REFERENCES tenants(id),
    branch_id   INT NOT NULL REFERENCES branches(id),
    item_id     INT NOT NULL REFERENCES items(id),
    stock       INT NOT NULL DEFAULT 0,
    updated_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE (tenant_id, branch_id, item_id)
);

CREATE TABLE warehouse_items (
    id            SERIAL PRIMARY KEY,
    tenant_id     INT NOT NULL REFERENCES tenants(id),
    warehouse_id  INT NOT NULL REFERENCES warehouses(id),
    item_id       INT NOT NULL REFERENCES items(id),
    stock         INT NOT NULL DEFAULT 0,
    updated_at    TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE (tenant_id, warehouse_id, item_id)
);

CREATE TABLE users (
    id          SERIAL PRIMARY KEY,
    tenant_id   INT NOT NULL REFERENCES tenants(id),
    name        TEXT NOT NULL,
    email       TEXT NOT NULL,
    password    TEXT NOT NULL,
    role        TEXT NOT NULL DEFAULT 'cashier',
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE (tenant_id, email)
);

CREATE TABLE customers (
    id          SERIAL PRIMARY KEY,
    tenant_id   INT NOT NULL REFERENCES tenants(id),
    branch_id   INT NOT NULL REFERENCES branches(id),
    name        TEXT NOT NULL,
    phone       TEXT,
    email       TEXT,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TYPE transaction_type AS ENUM ('SALE', 'PURC', 'TRANSFER');
CREATE TABLE transactions (
    id              SERIAL PRIMARY KEY,
    tenant_id       INT NOT NULL REFERENCES tenants(id),
    trxno           VARCHAR(100) NOT NULL,
    branch_id       INT REFERENCES branches(id),
    warehouse_id    INT REFERENCES warehouses(id),
    customer_id     INT REFERENCES customers(id),
    user_id         INT REFERENCES users(id),
    trans_type      transaction_type NOT NULL,
    total_amount    NUMERIC(12,2) NOT NULL DEFAULT 0,
    note            TEXT,
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE (tenant_id, trxno),
    CHECK (
      (branch_id IS NOT NULL OR warehouse_id IS NOT NULL)
    )
);

CREATE TABLE transaction_detail (
    id                  SERIAL PRIMARY KEY,
    transaction_id      INT NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
    branch_item_id      INT REFERENCES branch_items(id),
    warehouse_item_id   INT REFERENCES warehouse_items(id),
    quantity            INT NOT NULL CHECK (quantity > 0),
    price               NUMERIC(12,2) NOT NULL,
    subtotal            NUMERIC(12,2) NOT NULL,
    CHECK (
      (branch_item_id IS NOT NULL AND warehouse_item_id IS NULL)
      OR
      (branch_item_id IS NULL AND warehouse_item_id IS NOT NULL)
    )
);

CREATE TABLE audit_stock (
    id                 SERIAL PRIMARY KEY,
    tenant_id           INT NOT NULL REFERENCES tenants(id),
    warehouse_item_id  INT REFERENCES warehouse_items(id),
    branch_item_id     INT REFERENCES branch_items(id),
    change_unit        INT NOT NULL,
    reason             TEXT,
    created_at         TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TYPE cashflow_type AS ENUM ('SALE', 'TRANSFER', 'ADJUSTMENT');
CREATE TABLE branch_cashflow (
    id              SERIAL PRIMARY KEY,
    tenant_id       INT NOT NULL REFERENCES tenants(id),
    branch_id       INT NOT NULL REFERENCES branches(id),
    transaction_id INT REFERENCES transactions(id),
    flow_type       cashflow_type NOT NULL,
    direction       CHAR(3) CHECK (direction IN ('IN','OUT')),
    amount          NUMERIC(14,2) NOT NULL CHECK (amount > 0),
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TYPE tenant_flow_type AS ENUM ('SALE', 'PURC', 'WITHDRAW', 'ADJUSTMENT');
CREATE TABLE tenant_cashflow (
    id              SERIAL PRIMARY KEY,
    tenant_id       INT NOT NULL REFERENCES tenants(id),
    transaction_id INT REFERENCES transactions(id),
    flow_type       tenant_flow_type NOT NULL,
    direction       CHAR(3) CHECK (direction IN ('IN','OUT')),
    amount          NUMERIC(14,2) NOT NULL CHECK (amount > 0),
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

