# Deskripsi API

API untuk pemenuhan tugas akhir berjudul PERBANDINGAN PERFORMA DAN SKALABILITAS ARSITEKTUR MULTI-TENANT DATABASE PADA SISTEM BACKEND POINT OF SALE. API ini akan digunakan untuk membandingkan bagaimana performa API point of sale yang menggunakan 2 arsitektur database yang berbeda, yaitu single database (semua data tenant / pengguna dijadikan satu dalam satu database) sehingga query menggunakan tenant_id → WHERE tenant_id = ? dan multi database (setiap tenant memiliki database logisnya sendiri, di penelitian ini akan menggunakan satu service postgres berisi beberapa database) sehingga Request  
→ parse JWT  
→ ambil tenant_id  
→ lookup master DB  
→ ambil db credentials  
→ connect / reuse pool  
→ execute

API ini menggunakan layered architecture dan memiliki bentuk payload dan pola service atau logic yang sama persis, sehingga rencananya API ini akan menghandle dua pola skema database tadi, akan ada bagian configurasi yang menentukan API akan berjalan dengan skema multi atau single DB berdasarkan config tersebut (bagian router, controller, dan service mungkin akan sama persis hanya bagian repo dan model saja yang mungkin akan berbeda). jika memang hal tersebut tidak bisa its okay untuk bergeser ke pola lain, mungkin akan dipisahkan menjadi 2 project API yang berbeda.

untuk skema koneksi multi tenant akan menggunakan  PGBouncer untuk menghandle pool connection ke DB

# List Endpoint
## POST   /transactions/sale

Untuk menambah  penjualan dari branch
```
{
  "branch_id": 1,
  "customer_id": 12,
  "items": [
    { "branch_item_id": 5, "qty": 2 },
    { "branch_item_id": 9, "qty": 1 }
  ],
  "note": "walk-in"
}
```

## POST   /transactions/purchase

Untuk menambah pembelian dari supplier


## POST   /transactions/transfer

Untuk melakukan transfer stock


## GET    /inventory/branch/{id}

```
GET /inventory/branch/{id}?low_stock=true
```
Untuk mengcek item tiap branch


## GET    /inventory/warehouse/{id}

Untuk mengecek item tiap warehouse


## GET    /transactions

```
GET /transactions?branch_id=1
GET /transactions?warehouse_id=2
GET /transactions?type=SALE
GET /transactions?from=2026-01-01&to=2026-01-31
```
Ambil semua trasaction by filter

## GET    /transactions/{id}

Ambil detail transaction


## GET    /cashflow/branch/{id}

```
GET /cashflow/branch/{id}?from=...&to=...
```
Ambil keuangan masing" branch


## GET    /cashflow/tenant

```
GET /cashflow/tenant?from=...&to=...
```
Ambil keuangan masing" tenant

## Mungkin ada juga endpoint untuk reporting atau cash management


## POST   /inventory/adjust

Adjust inventory branch/warehouse

## CRUD Item, Branch, Warehouse, User dan Customer

# Struktur Folder
Kurang lebih project akan menggunakan layered arsitektur, sehingga mungkin akan memiliki struktur seperti ini:
```
cmd/
  server/
    main.go

internal/
  api/
    router.go
  handler/
    transaction_handler.go
  service/
    transaction_service.go
  repository/
    transaction_repository.go
  model/
    transaction.go
  middleware/
    auth.go
  tenant/
    resolver.go
  db/
    postgres.go
```
tapi mungkin bisa ada struktur yang lebih baik gapapa

# skema database
-  single db:
```
CREATE TABLE tenants (

id SERIAL PRIMARY KEY,

name TEXT NOT NULL,

created_at TIMESTAMP DEFAULT NOW()

);

  

CREATE TABLE warehouses (

id SERIAL PRIMARY KEY,

tenant_id INT NOT NULL REFERENCES tenants(id),

name TEXT NOT NULL,

created_at TIMESTAMP DEFAULT NOW()

);

CREATE INDEX idx_warehouses_tenant ON warehouses(tenant_id);

  

CREATE TABLE branches (

id SERIAL PRIMARY KEY,

tenant_id INT NOT NULL REFERENCES tenants(id),

phone TEXT NOT NULL,

name TEXT NOT NULL,

address TEXT NOT NULL,

opening_balance NUMERIC(14,2) NOT NULL DEFAULT 0,

created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()

);

CREATE INDEX idx_branches_tenant ON branches(tenant_id);

  

CREATE TABLE items (

id SERIAL PRIMARY KEY,

tenant_id INT NOT NULL REFERENCES tenants(id),

name TEXT NOT NULL,

sku TEXT,

price NUMERIC(12,2) NOT NULL,

description TEXT,

created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

UNIQUE (tenant_id, sku)

);

CREATE INDEX idx_items_tenant ON items(tenant_id);

  

CREATE TABLE branch_items (

id SERIAL PRIMARY KEY,

tenant_id INT NOT NULL REFERENCES tenants(id),

branch_id INT NOT NULL REFERENCES branches(id),

item_id INT NOT NULL REFERENCES items(id),

stock INT NOT NULL DEFAULT 0,

updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

UNIQUE (tenant_id, branch_id, item_id)

);

  

CREATE TABLE warehouse_items (

id SERIAL PRIMARY KEY,

tenant_id INT NOT NULL REFERENCES tenants(id),

warehouse_id INT NOT NULL REFERENCES warehouses(id),

item_id INT NOT NULL REFERENCES items(id),

stock INT NOT NULL DEFAULT 0,

updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

UNIQUE (tenant_id, warehouse_id, item_id)

);

  

CREATE TABLE users (

id SERIAL PRIMARY KEY,

tenant_id INT NOT NULL REFERENCES tenants(id),

name TEXT NOT NULL,

email TEXT NOT NULL,

password TEXT NOT NULL,

role TEXT NOT NULL DEFAULT 'cashier',

created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

UNIQUE (tenant_id, email)

);

  

CREATE TABLE customers (

id SERIAL PRIMARY KEY,

tenant_id INT NOT NULL REFERENCES tenants(id),

branch_id INT NOT NULL REFERENCES branches(id),

name TEXT NOT NULL,

phone TEXT,

email TEXT,

created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()

);

  

CREATE TYPE transaction_type AS ENUM ('SALE', 'PURC', 'TRANSFER');

CREATE TABLE transactions (

id SERIAL PRIMARY KEY,

tenant_id INT NOT NULL REFERENCES tenants(id),

trxno VARCHAR(100) NOT NULL,

branch_id INT REFERENCES branches(id),

warehouse_id INT REFERENCES warehouses(id),

customer_id INT REFERENCES customers(id),

user_id INT REFERENCES users(id),

trans_type transaction_type NOT NULL,

total_amount NUMERIC(12,2) NOT NULL DEFAULT 0,

note TEXT,

created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

UNIQUE (tenant_id, trxno),

CHECK (

(branch_id IS NOT NULL OR warehouse_id IS NOT NULL)

)

);

  

CREATE TABLE transaction_detail (

id SERIAL PRIMARY KEY,

transaction_id INT NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,

branch_item_id INT REFERENCES branch_items(id),

warehouse_item_id INT REFERENCES warehouse_items(id),

quantity INT NOT NULL CHECK (quantity > 0),

price NUMERIC(12,2) NOT NULL,

subtotal NUMERIC(12,2) NOT NULL,

CHECK (

(branch_item_id IS NOT NULL AND warehouse_item_id IS NULL)

OR

(branch_item_id IS NULL AND warehouse_item_id IS NOT NULL)

)

);

  

CREATE TABLE audit_stock (

id SERIAL PRIMARY KEY,

tenant_id INT NOT NULL REFERENCES tenants(id),

warehouse_item_id INT REFERENCES warehouse_items(id),

branch_item_id INT REFERENCES branch_items(id),

change_unit INT NOT NULL,

reason TEXT,

created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()

);

  

CREATE TYPE cashflow_type AS ENUM ('SALE', 'TRANSFER', 'ADJUSTMENT');

CREATE TABLE branch_cashflow (

id SERIAL PRIMARY KEY,

tenant_id INT NOT NULL REFERENCES tenants(id),

branch_id INT NOT NULL REFERENCES branches(id),

transaction_id INT REFERENCES transactions(id),

flow_type cashflow_type NOT NULL,

direction CHAR(3) CHECK (direction IN ('IN','OUT')),

amount NUMERIC(14,2) NOT NULL CHECK (amount > 0),

created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()

);

  

CREATE TYPE tenant_flow_type AS ENUM ('SALE', 'PURC', 'WITHDRAW', 'ADJUSTMENT');

CREATE TABLE tenant_cashflow (

id SERIAL PRIMARY KEY,

tenant_id INT NOT NULL REFERENCES tenants(id),

transaction_id INT REFERENCES transactions(id),

flow_type tenant_flow_type NOT NULL,

direction CHAR(3) CHECK (direction IN ('IN','OUT')),

amount NUMERIC(14,2) NOT NULL CHECK (amount > 0),

created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()

);
```
- multi db
```
-- for master db soon

CREATE TABLE tenants (

id SERIAL PRIMARY KEY,

name TEXT NOT NULL UNIQUE,

db_name TEXT NOT NULL,

db_user TEXT NOT NULL,

db_password TEXT NOT NULL,

created_at TIMESTAMP DEFAULT NOW()

);

  

CREATE TABLE warehouses (

id SERIAL PRIMARY KEY,

name TEXT NOT NULL,

created_at TIMESTAMP DEFAULT NOW()

);

  

CREATE TABLE branches (

id SERIAL PRIMARY KEY,

phone TEXT NOT NULL,

name TEXT NOT NULL,

address TEXT NOT NULL,

opening_balance NUMERIC(14,2) NOT NULL DEFAULT 0,

created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()

);

  

CREATE TABLE items (

id SERIAL PRIMARY KEY,

name TEXT NOT NULL,

sku TEXT UNIQUE,

price NUMERIC(12,2) NOT NULL,

description TEXT,

created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()

);

  

CREATE TABLE branch_items (

id SERIAL PRIMARY KEY,

branch_id INT NOT NULL REFERENCES branches(id),

item_id INT NOT NULL REFERENCES items(id),

stock INT NOT NULL DEFAULT 0,

updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()

);

CREATE UNIQUE INDEX idx_branch_items_unique

ON branch_items(branch_id, item_id);

  

CREATE TABLE warehouse_items (

id SERIAL PRIMARY KEY,

warehouse_id INT NOT NULL REFERENCES warehouses(id),

item_id INT NOT NULL REFERENCES items(id),

stock INT NOT NULL DEFAULT 0,

updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()

);

CREATE UNIQUE INDEX idx_warehouse_items_unique

ON warehouse_items(warehouse_id, item_id);

  

CREATE TABLE users (

id SERIAL PRIMARY KEY,

name TEXT NOT NULL,

email TEXT UNIQUE NOT NULL,

password TEXT NOT NULL,

role TEXT NOT NULL DEFAULT 'cashier',

created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()

);

  

CREATE TABLE customers (

id SERIAL PRIMARY KEY,

branch_id INT NOT NULL REFERENCES branches(id),

name TEXT NOT NULL,

phone TEXT,

email TEXT,

created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()

);

CREATE INDEX idx_customers_branch ON customers(branch_id);

  

CREATE TYPE transaction_type AS ENUM ('SALE', 'PURC', 'TRANSFER');

CREATE TABLE transactions (

id SERIAL PRIMARY KEY,

trxno VARCHAR(100) NOT NULL UNIQUE,

branch_id INT REFERENCES branches(id),

warehouse_id INT REFERENCES warehouses(id),

customer_id INT REFERENCES customers(id),

user_id INT REFERENCES users(id),

trans_type transaction_type NOT NULL,

total_amount NUMERIC(12,2) NOT NULL DEFAULT 0,

note TEXT,

created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()

CHECK (

(branch_id IS NOT NULL OR warehouse_id IS NOT NULL)

)

);

CREATE INDEX idx_transactions_branch ON transactions(branch_id);

CREATE INDEX idx_transactions_warehouse ON transactions(warehouse_id);

CREATE INDEX idx_transactions_type ON transactions(trans_type);

  

CREATE TABLE transaction_detail (

id SERIAL PRIMARY KEY,

transaction_id INT NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,

branch_item_id INT REFERENCES branch_items(id),

warehouse_item_id INT REFERENCES warehouse_items(id),

quantity INT NOT NULL CHECK (quantity > 0),

price NUMERIC(12,2) NOT NULL,

subtotal NUMERIC(12,2) NOT NULL,

CHECK (

(branch_item_id IS NOT NULL AND warehouse_item_id IS NULL)

OR

(branch_item_id IS NULL AND warehouse_item_id IS NOT NULL)

)

);

CREATE INDEX idx_trx_detail ON transaction_detail(transaction_id);

  

CREATE TABLE audit_stock (

id SERIAL PRIMARY KEY,

warehouse_item_id INT REFERENCES warehouse_items(id),

branch_item_id INT REFERENCES branch_items(id),

change_unit INT NOT NULL,

reason TEXT,

created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()

);

  

CREATE TYPE cashflow_type AS ENUM ('SALE', 'TRANSFER', 'ADJUSTMENT');

CREATE TABLE branch_cashflow (

id SERIAL PRIMARY KEY,

branch_id INT NOT NULL REFERENCES branches(id),

transaction_id INT REFERENCES transactions(id),

flow_type cashflow_type NOT NULL,

direction CHAR(3) CHECK (direction IN ('IN','OUT')),

amount NUMERIC(14,2) NOT NULL CHECK (amount > 0),

created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()

);

  

CREATE TYPE tenant_flow_type AS ENUM ('SALE', 'PURC', 'WITHDRAW', 'ADJUSTMENT');

CREATE TABLE tenant_cashflow (

id SERIAL PRIMARY KEY,

transaction_id INT REFERENCES transactions(id),

flow_type tenant_flow_type NOT NULL,

direction CHAR(3) CHECK (direction IN ('IN','OUT')),

amount NUMERIC(14,2) NOT NULL CHECK (amount > 0),

created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()

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

WHEN bc.direction = 'IN' THEN bc.amount

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

WHEN tc.direction = 'IN' THEN tc.amount

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
```