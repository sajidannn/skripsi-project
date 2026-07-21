# API Service — Golang Backend

Komponen `api/` ini adalah denyut nadi dari sisi server Point of Sale (POS) yang dirancang secara khusus (**Memory-safe & Concurrent-safe**) untuk mengeksekusi beban skenario pengujian TPC-C (hingga lebih dari ribuan Request dalam waktu 10 Menit).

Backend ini ditulis murni menggunakan **Golang 1.25**, dipisahkan dengan *Layered Architecture (Handler-Service-Repository Pattern)* demi mempermudah *switch-over* perbandingan dua mode arsitektur: **Single DB** (Logical isolation) dan **Multi DB** (Physical/Schema isolation).

---

## Stack Teknologi Indeks
- **Engine Router**: `Gin Gonic` (Efisien dalam penanganan alur router berdaya tinggi).
- **Driver Database**: `jackc/pgx/v5` dan `pgxpool` (Konektor PostgreSQL asli berbasis native pool yang amat menekan overhead I/O Disk).
- **Sistem Autentikasi / RBAC**: Bearer *JSON Web Token* (JWT).

---

## Modus Arsitektur (Multi-Tenancy)

Logika konektivitas sangat krusial agar gesekan pemrosesan (deadlock dan latensi RAM) tidak terjadi. API mengadopsi 2 *Repository Pattern*:

1. **`singledb/` Repository Layer**
   Semua beban koneksi dipusatkan pada SATU kolam koneksi (Connection Pooler). Setiap fungsi (misal Update Stok atau Cek HPP Barang) wajib menarik Parameter `tenant_id` ke dalam query `WHERE`.
   
2. **`multidb/` Repository Layer**
   API akan secara dinamis membuka *Connection Pooler* mikro secara *lazy loading* khusus untuk basis data fisik milik tenant. Pengelolaan koneksi per tenant dipisah secara independen dengan batas maksimal koneksi kecil (`MaxConns=4`) untuk menghindari habisnya `max_connections` PostgreSQL dan menghindari konsumsi RAM yang terlalu membengkak.

---

## Variabel Lingkungan (`.env`)

Untuk berjalan secara lokal maupun di container, salin berkas `.env.example` menjadi `.env` kemudian sesuaikan nilai konfigurasi utamanya:

| Variabel | Penjelasan Fokus Skripsi |
|---|---|
| `DB_MODE` | Diatur menggunakan `single` atau `multi`. Otomatis merubah jalur *Dependency Injection* ke layer *Service*. |
| `SINGLE_DSN` | Postgres Connect String bila `DB_MODE` adalah `single`. |
| `TENANT_DB_HOST` | Host Address PostgreSQL bila `DB_MODE` adalah `multi` (serta setting Password & Port tambahannya). |
| `MASTER_DSN` | Postgres Connect String ke database `pos_master` yang bertugas sebagai *directory service* metadata tenant pada mode `multi`. |
| `JWT_SECRET` | Secret cipher key untuk signing otentikasi login kasir & pengelola toko. |

---

## Rentang Endpoint TPC-C Equivalent

Semua alur akan dipanggil terus menerus oleh Workload Generator Locust, menyimulasikan transaksi harian ritel POS yang diadaptasi dari beban campuran (mixed workload) TPC-C dengan proporsi berikut:

*   **Transaksi Penjualan (New Order - 43%)**: `POST /transactions/sale` — Simulasi kasir melayani pelanggan dan melakukan perubahan stock serta saldo.
*   **Manajemen Stock (Stock Distribution - 22%)**: `POST /transactions/transfer` — Distribusi barang antar cabang atau gudang.
*   **Restock (Payment/Supply - 18%)**: `POST /transactions/purchase` — Pengadaan stock barang dari supplier oleh masing-masing tenant.
*   **Pengecekan Stock (Stock Level Check - 4%)**: `GET /inventory/branch/:id` — Monitoring ketersediaan barang tenant di tiap warehouse atau branch.
*   **Analisis & Laporan (13%)**: Kombinasi `GET /reports/summary`, `GET /reports/balance/tenant`, dan `GET /transactions` untuk memicu beban baca/tulis analitis.
