# DB — Database Server (VM 2)

Direktori ini berisi schema database dan Go seeder untuk deployment PostgreSQL pada server DB terpisah (VM 2). Dirancang untuk pengujian API POS berbasis workload TPC-C dengan **3 skala data** yang dapat dipilih secara dinamis.

## Struktur

```
DB/
├── schema/                       # SQL schema files (single source of truth)
│   ├── single-db.sql             # Schema shared-schema multi-tenant
│   ├── multi-db-master.sql       # Schema master DB (tenant routing)
│   └── multi-db-tenant.sql       # Schema per-tenant DB
├── seeder/                       # Go seeder program
│   ├── main.go                   # CLI entry point
│   ├── config.go                 # Definisi 3 skala data
│   ├── generator.go              # Deterministic data generation
│   ├── seed_single.go            # Single-DB seeding logic
│   ├── seed_multi.go             # Multi-DB seeding logic
│   ├── go.mod / go.sum
│   └── Dockerfile                # Multi-stage build
├── docker-compose.single.yml     # Deploy single-DB mode
├── docker-compose.multi.yml      # Deploy multi-DB mode
└── README.md
```

---

## Skala Data

Berdasarkan **Proposal Tabel 3.6 — Parameter Skala Data**:

| Parameter | Small | Medium | Large |
|---|---|---|---|
| Tenants | 5 | 10 | 50 |
| Warehouse/tenant | 5 | 7 | 10 |
| Branch/warehouse | 1 | 3 | 5 |
| Items/tenant | 1,000 | 3,000 | 5,000 |
| Customer/branch | 100 | 200 | 300 |
| Supplier/tenant | 10 | 30 | 50 |

**Estimasi total rows (single-DB, tanpa transaksi):**

| Skala | Estimasi Rows |
|---|---|
| Small | ~37,635 |
| Medium | ~345,810 |
| Large | ~4,758,100 |

---

## Cara Penggunaan

### Via Makefile (dari root project)

```bash
# Default (small scale)
make db-single-up

# Medium scale
make db-single-up SCALE=medium

# Large scale
make db-single-up SCALE=large

# Multi-DB mode
make db-multi-up SCALE=medium

# Lihat log seeder (progress seeding)
make db-single-logs-seeder
make db-multi-logs-seeder

# Clean + rebuild dengan skala baru
make db-single-reseed SCALE=large

# Hapus semua data (volume dihapus)
make db-single-clean
make db-multi-clean
make db-clean        # keduanya sekaligus
```

### Via Docker Compose langsung

```bash
cd DB

# Single-DB, small scale
SCALE=small docker compose -f docker-compose.single.yml up --build -d

# Multi-DB, medium scale
SCALE=medium docker compose -f docker-compose.multi.yml up --build -d

# Lihat progress
docker compose -f docker-compose.single.yml logs -f seeder

# Clean rebuild dengan skala berbeda
docker compose -f docker-compose.single.yml down -v
SCALE=large docker compose -f docker-compose.single.yml up --build -d
```

---

## Alur Kerja

```
docker compose up
  ↓
Postgres starts
  ↓ mounts schema SQL → /docker-entrypoint-initdb.d/
Postgres runs schema init (CREATE TABLE, CREATE INDEX, ...)
  ↓
Postgres healthy (pg_isready OK)
  ↓
Seeder container starts
  ↓
Seeder: pre-compute bcrypt hashes (sekali)
Seeder: generate all data in memory (deterministic, seed=42)
Seeder: TRUNCATE CASCADE semua tabel
Seeder: bulk insert via pgx COPY protocol (batch 50k rows)
Seeder: reset sequences → siap untuk API insert
  ↓
Seeder exits (restart: "no")
  ↓
Postgres running, ready to serve API ✓
```

---

## Konsistensi Data

- **Deterministik**: Menggunakan `rand.New(rand.NewSource(42))` — data identik setiap dijalankan pada skala yang sama
- **FK Valid**: Data di-generate dalam urutan dependency (tenants → warehouses → branches → items → ...)
- **Bcrypt**: Hash di-compute sekali, bukan per-row (efisiensi tinggi)
- **Bulk Insert**: Menggunakan PostgreSQL `COPY` protocol via `pgx.CopyFrom()` — jauh lebih cepat dari INSERT biasa

**Password:**
- Owner: `password123`
- Cashier: `cashier123`

---

## Estimasi Waktu Seeding

| Skala | Single-DB | Multi-DB (per tenant) |
|---|---|---|
| Small | ~5 detik | ~10 detik |
| Medium | ~30 detik | ~60 detik |
| Large | ~2–3 menit | ~5–8 menit |

> Estimasi untuk server dengan CPU 2-4 core, RAM 4GB, SSD.

---

## Update Schema

Jika ada perubahan tabel/kolom:

1. Edit file di `schema/` (single source of truth)
2. Update `generator.go` dan `seed_single.go` / `seed_multi.go` jika ada kolom baru yang perlu di-seed
3. Clean rebuild: `make db-single-reseed SCALE=<scale>`

Tidak perlu migration file — setiap rebuild selalu fresh dari schema terbaru.
