-- ─── Single-DB seed data ─────────────────────────────────────────────────────
-- A few sample rows to verify the schema and API work end-to-end.
-- Passwords below are bcrypt hashes of "password123".

-- Tenants
INSERT INTO tenants (name) VALUES
    ('Warung Maju'),
    ('Toko Berkah');

-- Warehouses (1 per tenant)
INSERT INTO warehouses (tenant_id, name) VALUES
    (1, 'Gudang Utama Warung Maju'),
    (2, 'Gudang Utama Toko Berkah');

-- Branches (2 per tenant)
INSERT INTO branches (tenant_id, phone, name, address, opening_balance) VALUES
    (1, '081111111111', 'Cabang Jakarta',  'Jl. Sudirman No.1, Jakarta',  5000000),
    (1, '081111111112', 'Cabang Bandung',  'Jl. Dago No.5, Bandung',      3000000),
    (2, '082222222221', 'Cabang Surabaya', 'Jl. Basuki Rahmat No.3, SBY', 4000000),
    (2, '082222222222', 'Cabang Malang',   'Jl. Ijen No.7, Malang',       2500000);

-- Items (3 per tenant)
INSERT INTO items (tenant_id, name, sku, cost, price, description) VALUES
    (1, 'Kopi Robusta',  'KOP-001', 25000, 30000, 'Kopi robusta premium 200g'),
    (1, 'Teh Hijau',     'TEH-001', 15000, 20000, 'Teh hijau organik 100g'),
    (1, 'Gula Pasir',    'GUL-001',  8000, 10000, 'Gula pasir 500g'),
    (2, 'Beras Premium', 'BRS-001', 65000, 75000, 'Beras putih pulen 5kg'),
    (2, 'Minyak Goreng', 'MYK-001', 20000, 25000, 'Minyak goreng 1L'),
    (2, 'Tepung Terigu', 'TPG-001', 12000, 15000, 'Tepung terigu serbaguna 1kg');

-- Warehouse stock
INSERT INTO warehouse_items (tenant_id, warehouse_id, item_id, stock) VALUES
    (1, 1, 1, 100),
    (1, 1, 2,  80),
    (1, 1, 3, 200),
    (2, 2, 4,  50),
    (2, 2, 5, 120),
    (2, 2, 6,  90);

-- Branch stock
INSERT INTO branch_items (tenant_id, branch_id, item_id, stock) VALUES
    (1, 1, 1, 20),
    (1, 1, 2, 15),
    (1, 1, 3, 40),
    (1, 2, 1, 10),
    (1, 2, 2,  8),
    (2, 3, 4, 12),
    (2, 3, 5, 25),
    (2, 4, 4,  8),
    (2, 4, 6, 18);

-- Users (1 owner per tenant, password = "password123")
--       (cashier users,       password = "cashier123")
INSERT INTO users (tenant_id, name, email, password, role) VALUES
    (1, 'Admin Warung Maju',  'admin@warungmaju.id',   '$2b$10$bVBy3KVe0q7E1U6W3IxQeO00msqqrfmljC9Z06NEssWAfM6oN/.sa', 'owner'),
    (1, 'Kasir A',            'kasir.a@warungmaju.id', '$2b$10$RLSAm.64GEao3zZB8o.l..HcCECsELWrLvo6sdY3oe1m.pY6zRH7i', 'cashier'),
    (2, 'Admin Toko Berkah',  'admin@tokoberkah.id',   '$2b$10$bVBy3KVe0q7E1U6W3IxQeO00msqqrfmljC9Z06NEssWAfM6oN/.sa', 'owner'),
    (2, 'Kasir B',            'kasir.b@tokoberkah.id', '$2b$10$RLSAm.64GEao3zZB8o.l..HcCECsELWrLvo6sdY3oe1m.pY6zRH7i', 'cashier');

-- Customers (1 per branch)
INSERT INTO customers (tenant_id, branch_id, name, phone) VALUES
    (1, 1, 'Budi Santoso',  '08131234567'),
    (1, 2, 'Siti Rahayu',   '08139876543'),
    (2, 3, 'Ahmad Fauzi',   '08235551234'),
    (2, 4, 'Dewi Lestari',  '08239876543');

-- Opening balance cashflow entries for tenants
INSERT INTO tenant_cashflow (tenant_id, flow_type, direction, amount) VALUES
    (1, 'ADJUSTMENT', 'IN', 5000000),
    (2, 'ADJUSTMENT', 'IN', 4000000);
