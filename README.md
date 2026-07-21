# Point of Sale (POS) TPC-C Benchmark System

[![Go Version](https://img.shields.io/badge/Go-1.25-blue.svg)](https://golang.org)
[![PostgreSQL](https://img.shields.io/badge/Postgres-16.0-blue.svg)](https://postgresql.org/)
[![Multi-Tenant](https://img.shields.io/badge/Architecture-Multi--Tenant-green.svg)]()

Repositori ini adalah implementasi dari skripsi berfokus pada **Analisis Komparatif Performa Database Multi-Tenant**. Sistem POS ini bertindak sebagai API yang diuji (SUT - *System Under Test*) melalui beban simulasi dari pola model **TPC-C**, untuk menemukan perbedaan latensi, skalabilitas, dan efisiensi resource.

Sistem dirancang untuk mendukung **dua isolasi arsitektur multi-tenant**:
1. **Single-DB (Shared Schema):** Semua data tenant bersatu dalam satu *schema* database dengan isolasi melalui validasi kolom `tenant_id`.
2. **Multi-DB (Database-per-Tenant):** Setiap tenant memiliki ruang database / skema eksklusif (contoh: `tenant_1`, `tenant_2`), terisolasi penuh secara fisik/logis untuk mereduksi *data contention*.

---

## Struktur Repositori

Setiap bagian subsistem dikelompokkan dengan tanggung jawab / *concern* masing-masing agar perbandingannya jelas:

```bash
.
├── api/          # Layanan Backend (Golang, Gin, pgxpool). Fokus pada logika bisnis & konektivitas DB.
├── db/           # Infrastruktur Database. Berisi File SQL schema, migrasi, Docker Compose, & alat Seeding.
├── workload/     # TPC-C Load Generator (Python/Locust). Membombardir API secara realistis layaknya kasir ritel.
├── monitoring/   # Stack Observabilitas Terdistribusi (Prometheus, Grafana, Exporters).
├── postman/      # Postman Collection & Environtment variables untuk testing.
└── Makefile      # Kumpulan pintasan operasional Makefile untuk orkestrasi skenario.
```


---

## Parameter Skala Data & Skenario Uji

Proyek ini telah dikonfigurasi agar sesuai dengan **Tabel Parameter TPC-C** yang diadaptasi untuk sistem Point of Sale. Pengujian (*Benchmark*) dilakukan secara otomatis menggunakan Seeder Golang dan pemanggilan skrip beban kerja melalui Locust.

### 1. Skala Pengisian Database (Seeder)
Digunakan pada saat menyiapkan infrastruktur database (misal `make db-single-up SCALE=medium`):

| Skala Pengujian | Total Tenants | Virtual Users | Warehouse / Tenant | Branch / Warehouse | Item / Tenant |
|---|---|---|---|---|---|
| **Small** | 5 | 50 | 3 | 3 | 1.000 |
| **Medium** | 10 | 100 | 3 | 3 | 3.000 |
| **Large** | 50 | 500 | 3 | 3 | 5.000 |
| **Extreme** | 150 | 1.500 | 3 | 3 | 2.000 |

### 2. Skenario Pengujian Mutlak
Pengujian dijalankan setelah API Server dan Database menyala sempurna. Alat Generator Beban secara otomatis menyimulasikan transaksi dengan kepadatan **10 Virtual User per tenant** selama durasi **10 Menit** per skenario:

*   **Skala Small**: `make workload-small` (Baseline 5 tenant, 50 user, 10 menit)
*   **Skala Medium**: `make workload-medium` (Skalabilitas 10 tenant, 100 user, 10 menit)
*   **Skala Large**: `make workload-large` (Maksimum stres 50 tenant, 500 user, 10 menit)
*   **Skala Extreme**: `make workload-extreme` (Skala ekstrem 150 tenant, 1.500 user, 10 menit)

---

## Ringkasan Hasil Pengujian

Berdasarkan analisis perbandingan performa serta skalabilitas antara arsitektur *Single-DB* dan *Multi-DB*, diperoleh kesimpulan utama sebagai berikut:

*   **Kesetaraan Performa Aplikasi**: Kedua arsitektur menunjukkan throughput yang identik (rata-rata ~170 RPS pada skala ekstrim) dengan tingkat kegagalan (*error rate*) 0,00%, membuktikan kemampuan skalabilitas beban yang sangat baik.
*   **Keunggulan Latensi Multi-DB**: Arsitektur Multi-DB secara konsisten memberikan latensi respons ~10% lebih rendah berkat ukuran indeks (*B-Tree traversal*) yang lebih kecil di setiap database tenant.
*   **Keunggulan Mutlak Efisiensi Single-DB**: Arsitektur Single-DB jauh lebih efisien dalam penggunaan sumber daya (CPU dan RAM). Pada skala ekstrim, Single-DB menghemat memori database hingga 2,65x lipat, disk I/O 16% lebih rendah, serta mencapai *Cache Hit Ratio* 8% lebih tinggi karena konsolidasi memori penyangga (*shared_buffers*).
*   **Trade-Off**: Multi-DB menawarkan isolasi fisik yang kuat untuk keamanan data dan latensi kueri yang cepat, namun dengan biaya konsumsi infrastruktur yang jauh lebih besar (alokasi *connection pool* yang masif). Sebaliknya, Single-DB memaksimalkan efisiensi server dengan kompromi pada isolasi logikal (*tenant_id*).

---

*Proyek Riset Tugas Akhir: **Perbandingan Performa dan Skalabilitas Arsitektur Multi-Tenant Database pada Sistem Backend Point of Sale** — oleh **Ahmad Nur Sajidan**.*
