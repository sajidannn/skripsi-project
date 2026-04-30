#!/usr/bin/env python3
"""
Prometheus Metrics Exporter untuk TPC-C Test
=============================================
Mengambil metrik dari Prometheus untuk rentang waktu tes tertentu,
lalu menyimpannya sebagai CSV terorganisir per tes.

Usage:
  python3 workload/export_metrics.py \
    --from 1714000000 --to 1714000300 \
    --tag large_50t_200u_20260424_120000 \
    --db-mode multi \
    --prometheus http://localhost:9090

Output: result/prometheus/<db_mode>/<tag>/
  - cpu_api.csv
  - cpu_db.csv
  - memory_api.csv
  - memory_db.csv
  - network_api.csv
  - pg_connections.csv
  - pg_queries.csv
  - pg_locks.csv
  - summary.json
"""
import argparse
import csv
import json
import os
import sys
from datetime import datetime, timezone

try:
    import requests
except ImportError:
    print("ERROR: 'requests' tidak ditemukan. Jalankan: pip3 install requests")
    sys.exit(1)


# ── Definisi Metrik yang Akan Di-export ───────────────────────────────────────
# Format: (nama_file, judul, query_promql)
METRICS = [
    # ── API Server (VM1) ──────────────────────────────────────────────────────
    (
        "cpu_api",
        "CPU Usage API Server (%)",
        '100 - (avg by (instance) (rate(node_cpu_seconds_total{mode="idle", vm="vm1"}[1m])) * 100)',
    ),
    (
        "memory_api",
        "Memory Used API Server (bytes)",
        'node_memory_MemTotal_bytes{vm="vm1"} - node_memory_MemAvailable_bytes{vm="vm1"}',
    ),
    (
        "network_in_api",
        "Network Receive API (bytes/s)",
        'rate(node_network_receive_bytes_total{vm="vm1", device!~"lo|docker.*|br.*"}[1m])',
    ),
    (
        "network_out_api",
        "Network Transmit API (bytes/s)",
        'rate(node_network_transmit_bytes_total{vm="vm1", device!~"lo|docker.*|br.*"}[1m])',
    ),
    (
        "container_cpu_api",
        "Container CPU API (cores)",
        'rate(container_cpu_usage_seconds_total{vm="vm1", name=~"pos-api.*", name!=""}[1m]) * 100',
    ),
    (
        "container_memory_api",
        "Container Memory API (bytes)",
        'container_memory_working_set_bytes{vm="vm1", name=~"pos-api.*", name!=""}',
    ),

    # ── DB Server (VM2) ───────────────────────────────────────────────────────
    (
        "cpu_db",
        "CPU Usage DB Server (%)",
        '100 - (avg by (instance) (rate(node_cpu_seconds_total{mode="idle", vm="vm2"}[1m])) * 100)',
    ),
    (
        "memory_db",
        "Memory Used DB Server (bytes)",
        'node_memory_MemTotal_bytes{vm="vm2"} - node_memory_MemAvailable_bytes{vm="vm2"}',
    ),
    (
        "memory_db_percent",
        "Memory Usage DB Server (%)",
        '100 - (node_memory_MemAvailable_bytes{vm="vm2"} / node_memory_MemTotal_bytes{vm="vm2"} * 100)',
    ),
    (
        "disk_read_db",
        "Disk Read Throughput DB (bytes/s)",
        'rate(node_disk_read_bytes_total{vm="vm2", device!~"sr.*|loop.*"}[1m])',
    ),
    (
        "disk_write_db",
        "Disk Write Throughput DB (bytes/s)",
        'rate(node_disk_written_bytes_total{vm="vm2", device!~"sr.*|loop.*"}[1m])',
    ),
    (
        "disk_read_latency_db",
        "Disk Read Latency DB (ms)",
        'rate(node_disk_read_time_seconds_total{vm="vm2", device!~"sr.*|loop.*"}[1m]) / rate(node_disk_reads_completed_total{vm="vm2", device!~"sr.*|loop.*"}[1m]) * 1000',
    ),
    (
        "disk_write_latency_db",
        "Disk Write Latency DB (ms)",
        'rate(node_disk_write_time_seconds_total{vm="vm2", device!~"sr.*|loop.*"}[1m]) / rate(node_disk_writes_completed_total{vm="vm2", device!~"sr.*|loop.*"}[1m]) * 1000',
    ),
    (
        "network_in_db",
        "Network Receive DB (bytes/s)",
        'rate(node_network_receive_bytes_total{vm="vm2", device!~"lo|docker.*|br.*"}[1m])',
    ),
    (
        "network_out_db",
        "Network Transmit DB (bytes/s)",
        'rate(node_network_transmit_bytes_total{vm="vm2", device!~"lo|docker.*|br.*"}[1m])',
    ),

    # ── PostgreSQL ────────────────────────────────────────────────────────────
    (
        "pg_connections",
        "PostgreSQL Active Connections",
        'pg_stat_activity_count',
    ),
    (
        "pg_tup_returned",
        "PostgreSQL Rows Returned/Read (rate/s)",
        "rate(pg_stat_database_tup_returned[1m])",
    ),
    (
        "pg_tup_inserted",
        "PostgreSQL Rows Inserted (rate/s)",
        "rate(pg_stat_database_tup_inserted[1m])",
    ),
    (
        "pg_tup_updated",
        "PostgreSQL Rows Updated (rate/s)",
        "rate(pg_stat_database_tup_updated[1m])",
    ),
    (
        "pg_locks",
        "PostgreSQL Locks Count",
        "pg_locks_count",
    ),
    (
        "pg_transactions",
        "PostgreSQL Transactions Committed (rate/s)",
        "rate(pg_stat_database_xact_commit[1m])",
    ),
    (
        "pg_rollbacks",
        "PostgreSQL Rollbacks (rate/s)",
        "rate(pg_stat_database_xact_rollback[1m])",
    ),
    (
        "pg_deadlocks",
        "PostgreSQL Deadlocks (count)",
        "pg_stat_database_deadlocks",
    ),
    (
        "pg_blk_hit_ratio",
        "PostgreSQL Cache Hit Ratio (%)",
        "100 * (sum by (datname) (rate(pg_stat_database_blks_hit[1m])) / (sum by (datname) (rate(pg_stat_database_blks_hit[1m])) + sum by (datname) (rate(pg_stat_database_blks_read[1m])) > 0))",
    ),
]


def query_range(prometheus_url: str, query: str, start: int, end: int, step: int = 15):
    """Query Prometheus range API dan kembalikan list of (timestamp, value, labels)."""
    url = f"{prometheus_url}/api/v1/query_range"
    try:
        resp = requests.get(url, params={
            "query": query,
            "start": start,
            "end":   end,
            "step":  step,
        }, timeout=30)
        resp.raise_for_status()
        data = resp.json()
        if data.get("status") != "success":
            return []
        return data.get("data", {}).get("result", [])
    except Exception as e:
        print(f"  WARN: query failed [{query[:60]}...]: {e}")
        return []


def results_to_rows(results: list) -> list:
    """Ubah hasil Prometheus menjadi list of dict untuk CSV."""
    rows = []
    for series in results:
        labels = series.get("metric", {})
        label_str = ",".join(f"{k}={v}" for k, v in labels.items()
                             if k not in ("__name__",))
        for ts, val in series.get("values", []):
            rows.append({
                "timestamp":  ts,
                "datetime":   datetime.fromtimestamp(ts, tz=timezone.utc)
                              .strftime("%Y-%m-%d %H:%M:%S UTC"),
                "value":      val,
                "labels":     label_str,
            })
    return sorted(rows, key=lambda r: r["timestamp"])


def save_csv(filepath: str, rows: list):
    """Simpan rows ke CSV."""
    if not rows:
        return
    os.makedirs(os.path.dirname(filepath), exist_ok=True)
    with open(filepath, "w", newline="") as f:
        writer = csv.DictWriter(f, fieldnames=["timestamp", "datetime", "value", "labels"])
        writer.writeheader()
        writer.writerows(rows)


def main():
    parser = argparse.ArgumentParser(description="Export Prometheus metrics for a test run.")
    parser.add_argument("--from",   dest="ts_from", type=int, required=True,
                        help="Unix timestamp start of test")
    parser.add_argument("--to",     dest="ts_to",   type=int, required=True,
                        help="Unix timestamp end of test")
    parser.add_argument("--tag",    required=True,
                        help="Prefix nama file (misal: large_50t_200u_20260424_120000)")
    parser.add_argument("--db-mode", dest="db_mode", default="multi",
                        choices=["single", "multi"],
                        help="DB mode untuk organisasi folder (default: multi)")
    parser.add_argument("--prometheus", default="http://localhost:9090",
                        help="URL Prometheus (default: http://localhost:9090)")
    parser.add_argument("--step",   type=int, default=15,
                        help="Step resolusi data dalam detik (default: 15)")
    args = parser.parse_args()

    out_dir = os.path.join("result", "prometheus", args.db_mode, args.tag)
    os.makedirs(out_dir, exist_ok=True)

    duration_s  = args.ts_to - args.ts_from
    start_human = datetime.fromtimestamp(args.ts_from, tz=timezone.utc).strftime("%Y-%m-%d %H:%M:%S UTC")
    end_human   = datetime.fromtimestamp(args.ts_to,   tz=timezone.utc).strftime("%Y-%m-%d %H:%M:%S UTC")

    print(f"\n>>> Exporting Prometheus metrics")
    print(f"    Tag        : {args.tag}")
    print(f"    DB Mode    : {args.db_mode}")
    print(f"    Time range : {start_human} → {end_human} ({duration_s}s)")
    print(f"    Step       : {args.step}s")
    print(f"    Output     : {out_dir}/")
    print(f"    Prometheus : {args.prometheus}")
    print()

    exported = []
    failed   = []

    for filename, title, query in METRICS:
        print(f"  Fetching [{filename}]...", end=" ", flush=True)
        results = query_range(args.prometheus, query, args.ts_from, args.ts_to, args.step)
        rows    = results_to_rows(results)

        if rows:
            filepath = os.path.join(out_dir, f"{filename}.csv")
            save_csv(filepath, rows)
            print(f"✓ {len(rows)} data points")
            exported.append(filename)
        else:
            print("- no data (metric not scraped or label mismatch)")
            failed.append(filename)

    # ── Simpan summary.json ───────────────────────────────────────────────────
    summary = {
        "tag":         args.tag,
        "db_mode":     args.db_mode,
        "start_ts":    args.ts_from,
        "end_ts":      args.ts_to,
        "start_human": start_human,
        "end_human":   end_human,
        "duration_s":  duration_s,
        "prometheus":  args.prometheus,
        "step_s":      args.step,
        "exported":    exported,
        "no_data":     failed,
    }
    summary_path = os.path.join(out_dir, "summary.json")
    with open(summary_path, "w") as f:
        json.dump(summary, f, indent=2)

    print()
    print(f"  ✓ Exported  : {len(exported)} metrics")
    if failed:
        print(f"  - No data   : {len(failed)} metrics ({', '.join(failed)})")
        print(f"    → Cek label vm='vm1'/'vm2' sudah sesuai di prometheus.yml")
    print(f"  ✓ Summary   : {summary_path}")
    print()


if __name__ == "__main__":
    main()
