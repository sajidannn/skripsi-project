# 📊 Observability & Monitoring Stack 

Terdapat puluhan metrik sistem operasi dan engine container yang berjejak mili-detik dalam skripsi benchmarking Performa Back-End ini. Sistem Ekstraksi *Observability* menggunakan pola **Remote Exporters Pattern (Push-Pull Strategy)** untuk menangkap data efisiensi komputasi dari Server Target tanpa membebani performa server target itu sendiri saat di load test.

---

## 📡 Arsitektur Monitoring Terdistribusi

Sesuai metodologi pada skripsi, agen pemantul (Exporters) ditempel pada virtual mesin dan dipanen oleh satu Hub Komando tersentral (Prometheus):

```text
┌────────────────────────┐      ┌────────────────────────┐      ┌────────────────────────┐
│     Laptop Lokal       │      │   Cloud VM 1           │      │   Cloud VM 2           │
│   (Monitoring Hub)     │      │   (API Server)         │      │   (DB Server)          │
│                        │      │                        │      │                        │
│ ┌────────────────┐     │      │ ┌────────────────┐     │      │ ┌────────────────┐     │
│ │ Prometheus     │─────┼──────▶│ Node Exporter  │     │      │ │ Node Exporter  │     │
│ └────────────────┘     │      │ ├────────────────┤     │      │ ├────────────────┤     │
│ ┌────────────────┐     │      │ │ cAdvisor       │     │      │ │ Postgres Exp   │     │
│ │ Grafana        │     │      │ └────────────────┘     │      │ └────────────────┘     │
│ └────────────────┘     │      │                        │      │                        │
└────────────────────────┘      └────────────────────────┘      └────────────────────────┘
```

1. **Laptop Lokal (Monitoring Hub):** Server utama tempat peneliti menarik kesimpulan metrik Visual. 
   - `Prometheus` menarik data target `/metrics` setiap detik.
   - `Grafana` mensintesis laporan CSV dan menampilkan GUI Dashboard pada Browser.

2. **Mesin Node Uji (Target Scraping Virtual Machines):**
   - **`Node Exporter`**: Terpasang di kedua VM untuk menyadap pemakaian mentah Mesin OS (Network Bandwidth, Total CPU Instance, Memory Keseluruhan sistem operasi Ubuntu).
   - **`cAdvisor (Google)`**: Terpasang esklusif pada VM 1-API untuk mengekstraksi kesehatan Kontainer Isolasi Docker, mengungkap isolasi performa Working Set Size (WSS) App Golangnya saja (`<~16 MB Memory footprint`). Menggunakan v0.55+ demi kestabilan sinkronasi API Client Engine.
   - **`Postgres Exporter`**: Terpasang di VM 2-PostgreSQL untuk mengungkap *Tuple Insert/s, Idle vs Active Queries Connection, dan Exclusive Locks Wait* layaknya DBA Profesional.

---

## 🚀 Instalasi Skenario Deployment

Ikuti urutan sinkronisasi ini untuk memulai awal evaluasi data:

### 1. Bangun Ekstraktor di `VM 1 (API)`
Buka terminal SSH di Mesin VM 1, lalu injek Eksportir:
```bash
# Menghidupkan Node Exporter & cAdvisor v0.55
make exporters-api-up
```

### 2. Bangun Ekstraktor di `VM 2 (Database)`
Buka terminal SSH di Mesin VM 2, lalu hubungkan Database Postgres dengan Eksportir:
*(Ingat: Pastikan Mesin container Postgres Node Utama sudah menyala sebelumnya).*
```bash
# Untuk skenario Single DB
make exporters-db-single-up

# Untuk skenario Multi DB
make exporters-db-multi-up
```

### 3. Bangun Hub Pengambil Data (Laptop Anda)
Kembali ke mesin Lokal milik Anda, operasikan Command Center ini:
```bash
make monitoring-up
```
Akses Portal Metrik pada URL Browser:
*   🔑 **Grafana:** `http://localhost:3000` *(Login Default: `admin` / Password: `admin`)*
*   📡 **Prometheus Data Store Target State:** `http://localhost:9090`

---

## 🛠️ Modifikasi Target Scraping (Custom IP Target)

Jika Server Riset VM atau EC2 Cloud Anda dimatikan dan direstart sehingga *IP Publik* terganti. Harap sinkronkan IP barunya pada `monitoring/prometheus/prometheus.yml`:

```yaml
# Contoh Target Scrape di prometheus.yml
  - job_name: 'node_exporter'
    static_configs:
      - targets: ['192.168.10.183:9100', '192.168.10.243:9100'] # Ganti 2 IP VM Ini
```

Jika terubah, Muat ulang tanpa mematikan layanan Hub Grafana dengan cara merelay *reload API Call Signal*:
```bash
make monitoring-reload
```

## 📉 Evaluasi Metrik Target

Bila melakukan Uji Locust, Pastikan merekam nilai-nilai **Kritikal Evaluasi Tabel Riset Skripsi Anda**, yakni merujuk pada:
*   `CPU/Memory Mode` API Isolation dan Database Isolation
*   `Contention/Lock/Transaction Limit Error 400 Wait Rate` pada PostgeSQL Panel
*   `Disk Throughput & Latency Average` (Kepekaan kecepatan baca-tulis IOPS Solid State Drive VM)
