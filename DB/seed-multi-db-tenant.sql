-- ─── Multi-DB per-tenant seed data ──────────────────────────────────────────
-- Applied to EACH tenant database (pos_tenant1, pos_tenant2, pos_tenant3).
-- Passwords below are bcrypt hashes of "password123".

-- Warehouse
INSERT INTO warehouses (name) VALUES
    ('Gudang Utama');

-- Branches
INSERT INTO branches (phone, name, address, opening_balance) VALUES
    ('081100000001', 'Cabang Pusat',  'Jl. Merdeka No.1', 5000000),
    ('081100000002', 'Cabang Timur',  'Jl. Pahlawan No.5', 3000000);

-- Items
INSERT INTO items (name, sku, price, description) VALUES
    ('Produk A', 'PRD-001', 50000, 'Produk unggulan A'),
    ('Produk B', 'PRD-002', 30000, 'Produk unggulan B'),
    ('Produk C', 'PRD-003', 20000, 'Produk pilihan C');

-- Warehouse stock
INSERT INTO warehouse_items (warehouse_id, item_id, stock) VALUES
    (1, 1, 100),
    (1, 2,  80),
    (1, 3, 150);

-- Branch stock
INSERT INTO branch_items (branch_id, item_id, stock) VALUES
    (1, 1, 20),
    (1, 2, 15),
    (1, 3, 30),
    (2, 1, 10),
    (2, 2,  8);

-- Users (owner password = "password123", cashier password = "cashier123")
INSERT INTO users (name, email, password, role) VALUES
    ('Admin Tenant',  'admin@tenant.id',  '$2b$10$bVBy3KVe0q7E1U6W3IxQeO00msqqrfmljC9Z06NEssWAfM6oN/.sa', 'owner'),
    ('Kasir Tenant',  'kasir@tenant.id',  '$2b$10$RLSAm.64GEao3zZB8o.l..HcCECsELWrLvo6sdY3oe1m.pY6zRH7i', 'cashier');

-- Customers
INSERT INTO customers (branch_id, name, phone) VALUES
    (1, 'Pelanggan A', '08100000001'),
    (2, 'Pelanggan B', '08100000002');

-- Opening balance cashflow for this tenant
INSERT INTO tenant_cashflow (flow_type, direction, amount) VALUES
    ('ADJUSTMENT', 'IN', 5000000);
