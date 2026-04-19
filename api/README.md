# API Service — Golang Backend

Komponen `api/` ini adalah denyut nadi dari sisi server Point of Sale (POS) yang dirancang secara khusus (**Memory-safe & Concurrent-safe**) untuk mengeksekusi beban skenario pengujian TPC-C (hingga lebih dari ribuan Request dalam waktu 5 Menit).

Backend ini ditulis murni menggunakan **Golang 1.25**, dipisahkan dengan *Clean Architecture (Handler-Service-Repository Pattern)* demi mempermudah *switch-over* perbandingan dua mode arsitektur: **Single DB** (Logical isolation) dan **Multi DB** (Physical/Schema isolation).

---

## 🛠️ Stack Teknologi Indeks
- **Engine Router**: `Gin Gonic` (Efisien dalam penanganan alur router berdaya tinggi).
- **Driver Database**: `jackc/pgx/v5` dan `pgxpool` (Konektor PostgreSQL asli berbasis native pool yang amat menekan overhead I/O Disk).
- **Sistem Autentikasi / RBAC**: Bearer *JSON Web Token* (JWT).

---

## 🧩 Modus Arsitektur (Multi-Tenancy)

Logika konektivitas sangat krusial agar gesekan pemrosesan (deadlock dan latensi RAM) tidak terjadi. API mengadopsi 2 *Repository Pattern*:

1. **`singledb/` Repository Layer**
   Semua beban koneksi dipusatkan pada SATU kolam koneksi (Connection Pooler). Setiap fungsi (misal Update Stok atau Cek HPP Barang) wajib menarik Parameter `tenant_id` ke dalam query `WHERE`.
   
2. **`multidb/` Repository Layer**
   API akan secara dinamis membuka *Connection Pooler* mikro sejumlah target yang ada (`tenant_1`, `tenant_2`, ...). Driver pgx akan memanggil `Search Path` yang sesuai pada saat memvalidasi JWT middleware. Query SQL sangat ramping tanpa validasi `WHERE tenant_id`.

---

## ⚙️ Variabel Lingkungan (`.env`)

Untuk berjalan secara lokal maupun di container (Node VM 1), salin berkas `.env.example` menjadi `.env` kemudian sesuaikan nilai konfigurasi utamanya:

| Variabel | Penjelasan Fokus Skripsi |
|---|---|
| `DB_MODE` | Diatur menggunakan `single` atau `multi`. Otomatis merubah jalur *Dependency Injection* ke layer *Service*. |
| `SINGLE_DSN` | Postgres Connect String bila `DB_MODE` adalah `single`. |
| `TENANT_DB_HOST` | Host Address PostgreSQL bila `DB_MODE` adalah `multi` (serta setting Password & Port tambahannya). |
| `JWT_SECRET` | Secret cipher key untuk signing otentikasi login kasir & pengelola toko. |

---

## 📦 Rentang Endpoint TPC-C Equivalent

Semua alur `api/v1/` akan dipukul terus menerus oleh Workload Generator Locust, menyimulasikan transaksi harian ritel POS yang mirip dengan *standard of measure* badan TPC-C internasional. Di dalam file skripsi, diatur sesuai pembagian berikut (pastikan mencocokkannya di postman):

*   **`POST /transactions/sale`** -> Beban Terbesar (*New-Order*: 65%). Mencabut stok pesanan ganda dan mencatat nilai tukar pendapatan.
*   **`POST /transactions/purchase` \ `POST /transactions/transfer`** -> Beban Menengah (*Payment/Restock*: 23%). Validasi otorisasi lintas cabang toko / restock *Warehouse*.
*   **`GET /inventory/branch/{id}`** -> Beban Analisis Cepat (*Stock-Level*: 4%). Pemanggilan nilai hitung persediaan kotor real-time.
*   **`GET /transactions?trans_type=SALE`** -> Beban Pengecekan Riwayat (*Order-Status/Billing*: 4%).
