# 🔫 Generator Beban TPC-C (Locust Workload)

Direktori `workload/` berisi konfigurasi *stress testing* yang berfungsi memborbardir sistem API/Database seperti di dunia nyata. Generator ini **secara absolut menuruti aturan parameter penelitian pada Bab 3 Skripsi Anda** yang mengadaptasi de-facto benchmarking ritel POS standard dewa, yaitu **TPC-C**.

Sistem diatur menggunakan skrip Python (*Locust*) karena sifatnya yang mudah diekstensi dan mampu membangun ribuan Virtual User seolah-olah bertindak sebagai entitas otonom.

---

## ⚖️ Konfigurasi Proporsional Akses (Adopsi TPC-C)

Tidak seperti HTTP stres tes konvensional (contoh: *JMeter* atau *Apache Bench*) yang menembak asal, modul `locustfile.py` diatur dengan probabilitas cerdas untuk menjaga rasio spesifikasi TPC-C **(Tabel 3.4 Proposal)**:

*   🏎️ `@task(65)` **Task Sale (New-Order)** ── `[65% beban]`  
    Transaksi paling mematikan. Modul Kasir merandom barang inventaris lokal dan memotong stok seraya mencatat mutasi uang se-akurat mungkin.
*   🚚 `@task(23)` **Task Restock/Transfer (Payment eq)** ── `[23% beban]`  
    Akses tingkat Owner/Manajer yang secara masif memasukkan/memindahkan balok inventaris di warehouse ke branch.
*   🔍 `@task(4)` **Task Stock Check (Stock-Level)** ── `[4% beban]`  
*   💳 `@task(4)` **Task Billing Check (Delivery eq)** ── `[4% beban]`  
*   📜 `@task(4)` **Task History (Order-Status)** ── `[4% beban]`  

Antar transaksi tersebut, diinjeksikan sebuah nilai variabel **wait_time (Keying Time)** senilai `antara 1 hingga 10 detik` yang amat esensial memvalidasi skenario TPC-C betapa lamanya rasio berpikir seorang kasir menekan tombol sistem (*Think Time*).

---

## 🔑 Login Pre-Caching (Bypass Limit Pendaftaran)

Salah satu skenario Multi-Tenant TPC-C termasif membutuhkan **200 Tenant User** untuk login sebelum serangan Workload dimulai. Mem-ping endpoint otentikasi login secara masif dan sinkron akan membuat *stress testing* tidak akurat (di dunia ritel orang sudah login pagi hari, dan baru sibuk di siang hari).

**Solusi:** Skrip pendukung, `login_generator.py` dipanggil sesaat sebelum *Locust* mengambil alih. Skrip ini menarik seluruh data karyawan yang dijanjikan Seeder Golang dari API, merangkumnya menjadi susunan Bearer JWT (*Authorization Tokens*) dalam berkas statis sementara `tokens.json`.

---

## 🔥 Eksekusi Skala Skenario Pengujian (Run Test)

Segala beban operasional memakan waktu keras tepat **5 Menit (`RUN_TIME=5m`)**. Skenario dieksekusi terpusat melalui peluncur cangkang (Shell) `run_test.sh` via root Makefile sesuai **Tabel Parameter Evaluasi 3.7 & 3.8.**

Gunakan Makefile dari Root Repo untuk langsung mengeksekusi sesuai target yang didata:

```bash
# S1 / S2 : Mode Baseline
make workload-small      # (5 Tenant / 50 Virtual Users)

# S3 / S5 : Mode Skalabilitas Medium
make workload-medium     # (10 Tenant / 100 Virtual Users)

# S4 / S6 : Mode Tekanan Puncak (Rasio Skripsi Maksimun)
make workload-large      # (50 Tenant / 200 Virtual Users)
```

**Tips:** Beban di-trigger diam-diam di background (headless shell mode). Saat selesai, konsol ini akan mencetak agregat *Average Time P50 P90 P99.* Tambahkan post-fix `-ui` (contoh: `make workload-large-ui`) apabila hendak memantau letusannya berbasis *Web Interface* (*Realtime RPS/Failures Flow*) dari laman `http://localhost:8089`.
