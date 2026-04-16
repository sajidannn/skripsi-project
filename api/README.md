# POS API — Backend Service

Backend Point of Sale (POS) yang dibangun dengan **Go 1.22** dan framework **Gin Gonic**. API ini dirancang untuk menangani beban transaksi tinggi dengan pattern yang konsisten untuk kedua mode multi-tenancy.

---

## 🛠️ Arsitektur

### Repository Pattern
Kode dipisahkan berdasarkan tanggung jawabnya:
- **Handler**: Menangani request HTTP, validasi input (DTO), dan mapping error.
- **Service**: Berisi business logic utama (cabang/item/transaksi).
- **Repository**: Abstraksi data akses.
  - `singledb/`: Implementasi SQL dengan filter `tenant_id`.
  - `multidb/`: Implementasi SQL dengan dynamic connection pool per tenant.

### Teknologi
- **Database Driver**: `jackc/pgx/v5` (High performance Postgres driver).
- **Web Framework**: `Gin` (Router & Middleware).
- **Auth**: JWT (JSON Web Token) dengan Role-Based Access Control (RBAC).

---

## ⚙️ Konfigurasi (Environment Variables)

Salin `.env.example` menjadi `.env` dan sesuaikan nilainya:

| Variable | Deskripsi | Nilai Default |
|---|---|---|
| `DB_MODE` | Mode multi-tenancy: `single` atau `multi`. | `single` |
| `SINGLE_DSN` | URL koneksi Postgres untuk mode single. | `postgres://...` |
| `TENANT_DB_HOST` | Host PostgreSQL untuk tenant DB (multi mode). | `localhost` |
| `JWT_SECRET` | Secret key untuk signing token JWT. | - |
| `DEBUG` | Jika `true`, error internal akan muncul di response API. | `false` |

---

## 🚀 Cara Menjalankan (Lokal)

1. Pastikan sudah menjalankan database (lihat README root).
2. Install dependensi:
   ```bash
   go mod tidy
   ```
3. Jalankan server:
   ```bash
   # Mode Single-DB
   DB_MODE=single go run ./cmd/server/...
   ```

---

## 📦 Endpoint Utama

- **Auth**: `POST /api/v1/auth/login`
- **Master Data**: `/api/v1/items`, `/api/v1/branches`, `/api/v1/warehouses`, `/api/v1/suppliers`
- **Transactions**:
  - `POST /api/v1/transactions/sale`: Penjualan kasir.
  - `POST /api/v1/transactions/purchase`: Pembelian stok dari supplier.
  - `POST /api/v1/transactions/transfer`: Perpindahan stok antar lokasi.
  - `POST /api/v1/transactions/return`: Retur penjualan/pembelian.
- **Inventory**: `GET /api/v1/inventory/branch/:id`

Semua request wajib menyertakan Header `Authorization: Bearer <token>` kecuali endpoint Login.

---

## 🧪 Deployment (Docker)

API ini dapat di-deploy menggunakan Docker:
```bash
docker build -t pos-api .
docker run -p 8080:8080 --env-file .env pos-api
```
