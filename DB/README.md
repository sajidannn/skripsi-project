# 🗄️ Database & Skema Inventaris POS

Pondasi utama dari analisis komparatif performa ini terletak pada **Container Database (`/db`)** dan program automasi *Data Seeder*. Sistem ini bertumpu pada **PostgreSQL v15.0**, memberikan landasan yang kokoh untuk stabilitas isolasi tingkat *RDBMS* saat ribuan transaksi saling mengunci (*ACID guarantees*).

---

## 🏗️ Topologi Isolasi Skema Multi-Tenant

Terdapat perbedaan mendasar dalam struktur data kedua mode arsitektur yang Anda jalankan di skripsi ini:

### 1. Mode `Single DB` (Tabel Flat / Shared Schema)
Modus basis data konvensional. Seluruh data dari *Puluhan Perusahaan Tenant* di tampung di dalam **satu kontainer PostgreSQL tunggal** dengan *schema* bernama `pos_single`.
*   **Risiko Isolasi:** Jika Tenant A membaca tabel, kecepatan bacanya sangat bergantung pada tumpukan jutaan baris milik Tenant B hingga Z di tabel yang sama.
*   **Level Pertahanan:** Membutuhkan kolom `tenant_id` secara keras (Hard-coded) pada tiap struktur perancangan kueri Backend API. (Logika `WHERE tenant_id = X`).

### 2. Mode `Multi DB` (Physical Schema / DB-per-Tenant)
Modus terisolasi dalam satu gugus / *cluster*. Saat sistem dinyalakan, Seeder akan menciptakan **puluhan schema mandiri** dalam instance server (contoh `tenant_001`, `tenant_002`, ... `tenant_050`). 
*   **Keuntungan (Sesuai Konklusi Tesis):** Memperpendek *Index B-Tree* per tabel karena batas ruang pencarian dibatasi di skema si penyewa. Ini menekan sangat jauh penggunaan memori komputasi saat pencatatan I/O di disk.
*   **Level Pertahanan:** Diikat dari parameter *search path* driver API ke nama schema spesifik. Query SQL bisa murni (*plain*) tanpa klausa `tenant_id`.

---

## 🔗 Skema ERD Pokok (*Relational Logic*)

Pada tiap batas otoritas sebuah Tenant, ini merupakan aliran struktur *Node* Data Inventaris.

*   `users`: Manajer, Pemilik dan Kasir yang terasosiasi ke cabang/tenant terkait.
*   `warehouses` & `branches`: Logika titik letak fisik stok fisik (Penyimpanan Utama vs Tempat Penjualan). 
*   `items` / `master_items`: Kamus universal tentang apa saja yang dijual Tenant.
*   `warehouse_items` & `branch_items`: *Bridge table* memecah stok *master item* menjadi entitas parsial lokasi di mana qty baris dipotong dan harga marjin final ditetapkan.
*   `transaction_headers` & `transaction_items`: Log pencatatan transaksi mutasi TPC-C Sale/Purchase/Transfer. Sangat rentan *Table Contention/Locks* manakala ratusan beban user menulis stok ganda pada waktu yang sama.

---

## 🌱 Go Seeder (Penanam Beban Skala Data)

Bagian `/db/seeder` adalah mesin *script golang* *single-responsibility* yang ditugaskan khusus menciptakan jutaan *Dummy Record* awal secara instan (Mengkondisikan agar database "tampak sudah penuh" sebelum `Workload Locust` dimulai). 

Dijalankan secara terpusat dari argumen docker-compose / Makefile root `SCALE=x`:

| Nilai Argument `SCALE` | Arti | Parameter Bebas Ditanam | 
|---|---|---|
| `small` (Tabel 3.6 - Skenario S1/S2)| Baseline (5 Tenant) | 2 Cabang/Tenant,  1.000 Jenis *Items* Master |
| `medium` (Tabel 3.6 - Skenario S3/S5) | Ekspansi (10 Tenant) | 5 Cabang/Tenant, 3.000 Jenis *Items* Master |
| `large` (Tabel 3.6 - Skenario S4/S6)| Super-Stress (50 Tenant) | 10 Cabang/Tenant, 5.000 Jenis *Items* Master |
