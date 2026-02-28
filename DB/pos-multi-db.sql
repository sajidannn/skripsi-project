-- for master db soon
CREATE TABLE tenants (
    id              SERIAL PRIMARY KEY,
    name            TEXT NOT NULL UNIQUE,
    db_name         TEXT NOT NULL,
    db_user         TEXT NOT NULL,
    db_password     TEXT NOT NULL,
    created_at      TIMESTAMP DEFAULT NOW()
);

CREATE TABLE warehouses (
    id          SERIAL PRIMARY KEY,
    name        TEXT NOT NULL,
    created_at  TIMESTAMP DEFAULT NOW()
);

CREATE TABLE branches (
    id            SERIAL PRIMARY KEY,
    phone         TEXT NOT NULL,
    name          TEXT NOT NULL,
    address       TEXT NOT NULL,
    opening_balance NUMERIC(14,2) NOT NULL DEFAULT 0,
    created_at    TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE items (
    id          SERIAL PRIMARY KEY,
    name        TEXT NOT NULL,
    sku         TEXT UNIQUE,
    price       NUMERIC(12,2) NOT NULL,
    description TEXT,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE branch_items (
    id          SERIAL PRIMARY KEY,
    branch_id   INT NOT NULL REFERENCES branches(id),
    item_id     INT NOT NULL REFERENCES items(id),
    stock       INT NOT NULL DEFAULT 0,
    updated_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
CREATE UNIQUE INDEX idx_branch_items_unique
ON branch_items(branch_id, item_id);

CREATE TABLE warehouse_items (
    id            SERIAL PRIMARY KEY,
    warehouse_id  INT NOT NULL REFERENCES warehouses(id),
    item_id       INT NOT NULL REFERENCES items(id),
    stock         INT NOT NULL DEFAULT 0,
    updated_at    TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
CREATE UNIQUE INDEX idx_warehouse_items_unique
ON warehouse_items(warehouse_id, item_id);

CREATE TABLE users (
    id          SERIAL PRIMARY KEY,
    name        TEXT NOT NULL,
    email       TEXT UNIQUE NOT NULL,
    password    TEXT NOT NULL,
    role        TEXT NOT NULL DEFAULT 'cashier',
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE customers (
    id          SERIAL PRIMARY KEY,
    branch_id   INT NOT NULL REFERENCES branches(id),
    name        TEXT NOT NULL,
    phone       TEXT,
    email       TEXT,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
CREATE INDEX idx_customers_branch ON customers(branch_id);

CREATE TYPE transaction_type AS ENUM ('SALE', 'PURC', 'TRANSFER');
CREATE TABLE transactions (
    id              SERIAL PRIMARY KEY,
    trxno           VARCHAR(100) NOT NULL UNIQUE,
    branch_id       INT REFERENCES branches(id),
    warehouse_id    INT REFERENCES warehouses(id),
    customer_id     INT REFERENCES customers(id),
    user_id         INT REFERENCES users(id),
    trans_type      transaction_type NOT NULL,
    total_amount    NUMERIC(12,2) NOT NULL DEFAULT 0,
    note            TEXT,
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW()
    CHECK (
      (branch_id IS NOT NULL OR warehouse_id IS NOT NULL)
    )
);
CREATE INDEX idx_transactions_branch ON transactions(branch_id);
CREATE INDEX idx_transactions_warehouse ON transactions(warehouse_id);
CREATE INDEX idx_transactions_type ON transactions(trans_type);

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
CREATE INDEX idx_trx_detail ON transaction_detail(transaction_id);

CREATE TABLE audit_stock (
    id                 SERIAL PRIMARY KEY,
    warehouse_item_id  INT REFERENCES warehouse_items(id),
    branch_item_id     INT REFERENCES branch_items(id),
    change_unit        INT NOT NULL,
    reason             TEXT,
    created_at         TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TYPE cashflow_type AS ENUM ('SALE', 'TRANSFER', 'ADJUSTMENT');
CREATE TABLE branch_cashflow (
    id          SERIAL PRIMARY KEY,
    branch_id   INT NOT NULL REFERENCES branches(id),
    transaction_id INT REFERENCES transactions(id),
    flow_type   cashflow_type NOT NULL,
    direction   CHAR(3) CHECK (direction IN ('IN','OUT')),
    amount      NUMERIC(14,2) NOT NULL CHECK (amount > 0),
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TYPE tenant_flow_type AS ENUM ('SALE', 'PURC', 'WITHDRAW', 'ADJUSTMENT');
CREATE TABLE tenant_cashflow (
    id          SERIAL PRIMARY KEY,
    transaction_id INT REFERENCES transactions(id),
    flow_type   tenant_flow_type NOT NULL,
    direction   CHAR(3) CHECK (direction IN ('IN','OUT')),
    amount      NUMERIC(14,2) NOT NULL CHECK (amount > 0),
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- input balance awal tenant
DO $$
DECLARE
    opening_balance NUMERIC(14,2) := 12000000;
BEGIN
    INSERT INTO tenant_cashflow
        (flow_type, direction, amount)
    VALUES
        ('ADJUSTMENT', 'IN', opening_balance);
END $$;

-- saldo branch
SELECT
    b.id,
    b.name,
    b.opening_balance
    + COALESCE(SUM(
        CASE
            WHEN bc.direction = 'IN'  THEN bc.amount
            WHEN bc.direction = 'OUT' THEN -bc.amount
        END
      ), 0) AS current_balance
FROM branches b
LEFT JOIN branch_cashflow bc
    ON bc.branch_id = b.id
GROUP BY b.id;

-- omzet tenant
SELECT
    COALESCE(SUM(amount), 0) AS total_omzet
FROM tenant_cashflow
WHERE flow_type = 'SALE'
  AND direction = 'IN';

-- saldo tenant
SELECT
    COALESCE(SUM(
        CASE
            WHEN tc.direction = 'IN'  THEN tc.amount
            WHEN tc.direction = 'OUT' THEN -tc.amount
        END
      ), 0) AS tenant_balance
FROM tenant_cashflow tc;

-- snapshoot lengkap
SELECT
    COALESCE(SUM(CASE WHEN tc.flow_type = 'SALE' AND tc.direction='IN' THEN tc.amount END),0) AS omzet,
    COALESCE(SUM(CASE WHEN tc.direction='IN' THEN tc.amount END),0)
  - COALESCE(SUM(CASE WHEN tc.direction='OUT' THEN tc.amount END),0) AS current_balance
FROM tenant_cashflow tc;



-- Pastikan kamu bisa menjelaskan atomic steps berikut:
--
-- SALE
-- insert transactions
-- insert transaction_detail
-- update branch_items.stock (-)
-- insert audit_stock
-- insert branch_cashflow (IN)
-- insert tenant_cashflow (IN)
-- → 1 DB transaction

-- PURC
-- insert transactions
-- insert transaction_detail
-- update warehouse_items.stock (+)
-- insert audit_stock
-- insert tenant_cashflow (OUT)

-- TRANSFER
-- insert transactions
-- insert 2 transaction_detail
-- update warehouse stock (-)
-- update branch stock (+)
-- insert 2 audit_stock
-- insert branch_cashflow (OUT) (jika memang ada biaya)
-- Kalau kamu bisa tulis ini jelas, desainmu lulus uji akademik.
