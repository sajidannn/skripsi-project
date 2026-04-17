# Monitoring Stack — POS Skripsi

Prometheus + Grafana + exporters untuk capture resource utilization selama pengujian TPC-C.

## Arsitektur

```
┌─────────────────────────────────┐   ┌────────────────────────────────┐
│         VM 1 (API Server)       │   │        VM 2 (DB Server)        │
│                                 │   │                                │
│  monitoring/                    │   │  DB/docker-compose.exporters   │
│  ├─ Prometheus  :9090   ────────┼───┤──▶ node_exporter       :9100  │
│  ├─ Grafana     :3000           │   │──▶ postgres_exporter   :9187  │
│  ├─ node-exporter :9100  ◀──┐   │   │                                │
│  └─ cAdvisor     :8081  ◀──┘   │   │  DB containers                 │
│                           │    │   │  ├─ pg-pos-single  :5432       │
│  pos-api container  ──────┘    │   │  └─ pg-pos-multi   :5433       │
└─────────────────────────────────┘   └────────────────────────────────┘
```

---

## Cara Pakai

### VM 2 (DB Server) — jalankan sekali sebelum eksperimen

```bash
# Skenario S1/S3/S4 (single-DB)
make db-exporters-single

# Skenario S2/S5/S6 (multi-DB)
make db-exporters-multi
```

### VM 1 (API Server) — jalankan sekali sebelum eksperimen

```bash
make monitoring-up
```

Buka Grafana: **http://VM1_IP:3000** (login: `admin` / `admin`)

Dua dashboard sudah ter-import otomatis:
- **POS — API Server (VM 1)** — CPU host, Memory, Container CPU/Memory, Network I/O
- **POS — DB Server (VM 2)** — CPU, Memory, Disk I/O, Disk Latency, PostgreSQL stats

### Verifikasi targets di Prometheus

```bash
# Cek semua scrape targets statusnya UP
curl -s http://localhost:9090/api/v1/targets | python3 -c "
import json,sys
d = json.load(sys.stdin)
for t in d['data']['activeTargets']:
    print(t['labels']['job'], '->', t['health'], t['lastError'] or '')
"
```

---

## Metrik yang Di-capture (sesuai Proposal Tabel 3.9)

| Komponen | Metrik | Exporter |
|---|---|---|
| **API Server (VM 1)** | CPU usage (host) | node_exporter |
| | Memory usage (host) | node_exporter |
| | CPU per container (pos-api) | cAdvisor |
| | Memory per container (pos-api) | cAdvisor |
| | Network I/O | node_exporter |
| **DB Server (VM 2)** | CPU usage | node_exporter |
| | Memory usage | node_exporter |
| | **Disk I/O throughput** (read/write bytes/s) | node_exporter |
| | Disk latency | node_exporter |
| | PostgreSQL connections | postgres_exporter |
| | Tuples insert/update/read rate | postgres_exporter |
| | Lock wait count | postgres_exporter |

> Response time, Throughput, Error rate → dari **Locust** (client-side), bukan Prometheus.

---

## Alur Eksperimen (Workflow)

```
1. make db-single-reseed SCALE=small   # VM 2: seed data
2. make db-exporters-single             # VM 2: start exporters (sekali saja)
3. make monitoring-up                   # VM 1: start monitoring (sekali saja)
4. make api-single-up                   # VM 1: start API
5. Jalankan Locust dari perangkat lain  # workload generator
6. Buka Grafana, amati dashboard selama Locust jalan
7. Export data dari Locust (CSV) + screenshot Grafana graphs
8. make db-single-reseed SCALE=medium  # VM 2: ganti skala, ulangi
```

---

## Konfigurasi

### Ganti DB Server IP

Edit `monitoring/prometheus/prometheus.yml`:
```yaml
- job_name: node_exporter_db
  static_configs:
    - targets: ['<IP_VM2>:9100']   # ganti di sini

- job_name: postgres_exporter
  static_configs:
    - targets: ['<IP_VM2>:9187']   # ganti di sini
```

Lalu reload Prometheus (tanpa restart):
```bash
make monitoring-reload
```

### Akses Grafana dari luar VM

Port 3000 sudah di-expose. Buka dari browser di perangkat Locust:
```
http://192.168.10.xxx:3000
```
