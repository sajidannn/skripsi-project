# Monitoring Stack — Remote Exporter Pattern

Prometheus dan Grafana berjalan di **Laptop Lokal (Monitoring Hub)**, menarik metrik dari **VM 1 (API)** dan **VM 2 (DB)** melalui jaringan.

## Arsitektur

```
┌────────────────────────┐      ┌────────────────────────┐      ┌────────────────────────┐
│     Laptop Lokal       │      │         VM 1           │      │         VM 2           │
│   (Monitoring Hub)     │      │     (API Server)       │      │     (DB Server)        │
│                        │      │                        │      │                        │
│ ┌────────────────┐     │      │ ┌────────────────┐     │      │ ┌────────────────┐     │
│ │ Prometheus     │─────┼──────▶│ Node Exporter  │     │      │ │ Node Exporter  │     │
│ └────────────────┘     │      │ ├────────────────┤     │      │ ├────────────────┤     │
│ ┌────────────────┐     │      │ │ cAdvisor       │     │      │ │ Postgres Exp   │     │
│ │ Grafana        │     │      │ └────────────────┘     │      │ └────────────────┘     │
│ └────────────────┘     │      │                        │      │                        │
└────────────────────────┘      └────────────────────────┘      └────────────────────────┘
```

---

## Cara Pakai

Ikuti urutan ini untuk setup awal:

### 1. VM 1 (API Server) — Jalankan Exporters
Buka terminal di VM 1, lalu jalankan:
```bash
make exporters-api-up
```
Ini akan menjalankan **Node Exporter** (host metrics) dan **cAdvisor** (container metrics).

### 2. VM 2 (DB Server) — Jalankan Exporters
Buka terminal di VM 2, lalu jalankan sesuai skenario:
```bash
# Untuk skenario Single DB
make exporters-db-single

# Untuk skenario Multi DB
make exporters-db-multi
```
Ini akan menjalankan **Node Exporter** dan **Postgres Exporter** (dengan fitur *auto-discovery* untuk semua tenant DB).

### 3. Laptop Lokal — Jalankan Monitoring Hub
Di terminal laptop kamu, jalankan:
```bash
make monitoring-up
```
Buka Grafana: **http://localhost:3000** (login: `admin` / `admin`).

---

## Verifikasi di Prometheus (Laptop)

Buka **http://localhost:9090/targets** di browser laptop kamu. Pastikan semua target berikut berstatus **UP**:
- `prometheus` (localhost)
- `node_exporter_api` (192.168.10.183:9100)
- `cadvisor_api` (192.168.10.183:8081)
- `node_exporter_db` (192.168.10.243:9100)
- `postgres_exporter` (192.168.10.243:9187)

---

## Konfigurasi IP
Jika IP VM berubah, edit file `monitoring/prometheus/prometheus.yml` dan sesuaikan bagian `static_configs` targets. Setelah simpan, jalankan:
```bash
make monitoring-reload
```

## Metrik yang Di-capture
Sesuai Proposal Tabel 3.9, semua resource utilization (CPU, Memory, Disk I/O) dari kedua VM akan terkumpul di satu dashboard pusat di laptop kamu.
- **Dashboard API Server**: Fokus ke performa VM 1 dan isolasi container `pos-api`.
- **Dashboard DB Server**: Fokus ke performa disk I/O dan kesehatan PostgreSQL.
