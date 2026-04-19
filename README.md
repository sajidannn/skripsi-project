# 🛒 Point of Sale (POS) TPC-C Benchmark System

[![Go Version](https://img.shields.io/badge/Go-1.25-blue.svg)](https://golang.org)
[![PostgreSQL](https://img.shields.io/badge/Postgres-16.0-blue.svg)](https://postgresql.org/)
[![Multi-Tenant](https://img.shields.io/badge/Architecture-Multi--Tenant-green.svg)]()

Repositori ini adalah implementasi dari skripsi berfokus pada **Analisis Komparatif Performa Database Multi-Tenant**. Sistem POS ini bertindak sebagai API yang diuji (SUT - *System Under Test*) melalui beban simulasi dari pola model **TPC-C**, untuk menemukan perbedaan latensi, skalabilitas, dan efisiensi resource.

Sistem dirancang untuk mendukung **dua isolasi arsitektur multi-tenant**:
1. **Single-DB (Shared Schema):** Semua data tenant bersatu dalam satu *schema* database dengan isolasi melalui validasi kolom `tenant_id`.
2. **Multi-DB (Database-per-Tenant):** Setiap tenant memiliki ruang database / skema eksklusif (contoh: `tenant_1`, `tenant_2`), terisolasi penuh secara fisik/logis untuk mereduksi *data contention*.

---

## 📂 Struktur Repositori

Setiap bagian subsistem dikelompokkan dengan tanggung jawab / *concern* masing-masing agar perbandingannya jelas:

```bash
.
├── api/          # ⚙️ Layanan Backend (Golang, Gin, pgxpool). Fokus pada logika bisnis & konektivitas DB.
├── db/           # 🗄️ Infrastruktur Database. Berisi File SQL schema, migrasi, Docker Compose, & alat Seeding.
├── workload/     # 🔫 TPC-C Load Generator (Python/Locust). Membombardir API secara realistis layaknya kasir ritel.
├── monitoring/   # 📊 Stack Observabilitas Terdistribusi (Prometheus, Grafana, Exporters).
├── postman/      # 🧪 Postman Collection & Environtment variables untuk testing.
└── Makefile      # 🧰 Kumpulan pintasan operasional Makefile untuk orkestrasi skenario.
```

Dipersilakan merujuk ke setiap direktori untuk membaca **instruksi mendalam** dari masing-masing subsistem (contoh: buka `api/README.md` untuk menjalankan backend).

---

## 📈 Parameter Skala Data & Skenario Uji

Proyek ini telah dikonfigurasi agar sesuai dengan **Tabel Parameter TPC-C** yang diusulkan. Pengujian (*Benchmark*) dilakukan secara otomatis menggunakan Seeder Golang dan pemanggilan skrip pembobot di `workload`.

### 1. Skala Pengisian Database (Seeder)
Digunakan pada saat menyiapkan infrastruktur database (misal `make db-single-up SCALE=medium`):

| Skala Pengujian | Total Tenants | Total Kasir | Jumlah Cabang per Tenant | Stok Baris / Barang Awal |
|---|---|---|---|---|
| **Small**  | 5 | 50 (Kasir) | 2 | 1.000 Jenis x QTY Besar |
| **Medium** | 10 | 100 (Kasir)| 5 | 3.000 Jenis x QTY Besar |
| **Large**  | 50 | 200 (Kasir)| 10 | 5.000 Jenis x QTY Besar |

### 2. Skenario Pengujian Mutlak
Pengujian dijalankan setelah API Server dan Database menyala sempurna. Alat Generator Beban (*workload*) secara otomatis menyimulasikan transaksi selama batas runtime **5 Menit** per skenario:

*   **Skala Small**: `make workload-small` (Baseline 5 tenant, 50 user, 5 menit)
*   **Skala Medium**: `make workload-medium` (Skalabilitas 10 tenant, 100 user, 5 menit)
*   **Skala Large**: `make workload-large` (Maksimum stres 50 tenant, 200 user, 5 menit)

---

*Proyek Riset Tugas Akhir: **Perbandingan Performa dan Skalabilitas Arsitektur Multi-Tenant Database pada Sistem Backend Point of Sale** — oleh **Ahmad Nur Sajidan**.*
