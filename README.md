# POS API Multi-Tenant System

Sistem Point of Sale (POS) backend dengan arsitektur multi-tenant yang fleksibel, mendukung dua mode deployment: **Single-DB (Shared Schema)** dan **Multi-DB (Database-per-Tenant)**. Dibangun menggunakan Go (Gin) dan PostgreSQL untuk performa dan skalabilitas tinggi.

---

## 📂 Struktur Proyek

```bash
.
├── api/          # Backend service (Go, Gin, pgxpool)
├── DB/           # Schema database, Docker Compose, dan Go Seeder
├── postman/      # Koleksi API Postman dan Environment
├── Makefile      # Shortcut perintah operasional
└── monitoring/   # (Optional) Konfigurasi untuk monitoring/benchmark
```

---

## 🚀 Persiapan Cepat

### Prasyarat
- Docker & Docker Compose
- Go 1.22+ (untuk running lokal)
- Make (opsional, tapi sangat disarankan)

### Menjalankan Database
Pilih salah satu mode database. Mode **Single-DB** paling ringan untuk pengembangan awal.

```bash
# Menjalankan Single-DB (Small scale by default)
make db-single-up

# Jika ingin skala data lebih besar (medium/large):
make db-single-up SCALE=medium
```

### Menjalankan API (Lokal)
Setelah database up dan seeding selesai, jalankan API dengan mode yang sesuai:

```bash
# Running API mode Single-DB
make api-single-run

# Atau mode Multi-DB
make api-multi-run
```

Akses health check di: `http://localhost:8080/health`

---

## 📊 Skala Data (Seeding)
Proyek ini mendukung seeding otomatis dengan parameter skala berikut:

| Skala | Tenants | Items/Tenant | Estimasi Total Rows |
|---|---|---|---|
| **Small** | 5 | 1.000 | ~37rb |
| **Medium**| 10 | 3.000 | ~345rb |
| **Large** | 50 | 5.000 | ~4,7jt |

Gunakan `make db-single-reseed SCALE=large` untuk menghapus data lama dan mengisi ulang dengan skala baru.

---

## 🛠️ Perintah Utama (Makefile)

| Perintah | Deskripsi |
|---|---|
| `make db-single-up` | DB Single: Jalankan kontainer Postgres & Seeder. |
| `make db-single-logs-seeder` | Lihat progress pengisian data ke database. |
| `make api-single-run` | Jalankan API lokal dengan koneksi ke Single-DB. |
| `make api-build` | Kompilasi Go backend ke file biner. |
| `make db-clean` | Hapus semua data dan kontainer database. |

---

## 📧 Kontak & Pengembangan
Proyek ini dikembangkan oleh **Sajidannn** sebagai bagian dari tugas akhir (skripsi) mengenai performa database multi-tenant.
