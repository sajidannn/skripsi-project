#!/usr/bin/env python3
import json
import os
import csv
import sys
import subprocess

def read_csv_inline(base_path, csv_file):
    """Baca file CSV dan return sebagai string, hanya kolom yang diperlukan."""
    path = os.path.join(base_path, csv_file)
    if not os.path.exists(path):
        return None
    seen_ts = {}
    with open(path, newline="", encoding="utf-8") as f:
        reader = csv.DictReader(f)
        for row in reader:
            ts  = row.get("timestamp", "").strip()
            val = row.get("value", "").strip()
            if not ts or not val:
                continue
            if ts not in seen_ts:
                seen_ts[ts] = float(val)
            else:
                seen_ts[ts] += float(val)  # sum per timestamp
    
    if not seen_ts:
        return None
        
    rows = ["timestamp,value"]
    for ts, val in sorted(seen_ts.items(), key=lambda x: int(x[0])):
        rows.append(f"{ts},{val:.4f}")
    return "\n".join(rows)

def make_target(base_path, csv_file, ref_id="A"):
    data_str = read_csv_inline(base_path, csv_file)
    if data_str is None:
        return None
    return {
        "datasource": {"type": "yesoreyeram-infinity-datasource", "uid": "infinity-csv"},
        "refId": ref_id,
        "type": "csv",
        "parser": "frontend",
        "source": "inline",
        "data": data_str,
        "url_options": {"method": "GET", "data": ""},
        "columns": [
            {"selector": "timestamp", "text": "Time",  "type": "timestamp_epoch_s"},
            {"selector": "value",     "text": "Value", "type": "number"}
        ],
        "format": "timeseries"
    }

def make_panel(base_path, pid, title, csv_file, x, y, w=12, h=8, unit="percent", min_=0, max_=None):
    target = make_target(base_path, csv_file)
    if target is None:
        return None
    fc = {"defaults": {"unit": unit, "min": min_}, "overrides": []}
    if max_ is not None:
        fc["defaults"]["max"] = max_
    return {
        "id": pid, "title": title, "type": "timeseries",
        "datasource": {"type": "yesoreyeram-infinity-datasource", "uid": "infinity-csv"},
        "gridPos": {"x": x, "y": y, "w": w, "h": h},
        "fieldConfig": fc,
        "options": {
            "tooltip": {"mode": "multi", "sort": "none"},
            "legend": {"displayMode": "list", "placement": "bottom", "showLegend": True}
        },
        "targets": [target]
    }

def make_row(row_id, title, y):
    return {"id": row_id, "title": title, "type": "row",
            "collapsed": False, "gridPos": {"x": 0, "y": y, "w": 24, "h": 1}}

def list_available_results(project_root):
    base = os.path.join(project_root, "result", "prometheus")
    print("Folder hasil tes yang tersedia:\n")
    print(f"  {'DB Mode':<12} {'Tag'}")
    print(f"  {'-'*12} {'-'*50}")
    if os.path.isdir(base):
        for db in sorted(os.listdir(base)):
            db_path = os.path.join(base, db)
            if os.path.isdir(db_path) and db != "current":
                for tag in sorted(os.listdir(db_path)):
                    if os.path.isdir(os.path.join(db_path, tag)):
                        print(f"  {db:<12} {tag}")
    print("\nUsage:")
    print("  python3 workload/generate_inline_dashboard.py <db_mode> <tag>")

def main():
    script_dir = os.path.dirname(os.path.abspath(__file__))
    project_root = os.path.dirname(script_dir)
    
    if len(sys.argv) < 3:
        list_available_results(project_root)
        sys.exit(0)

    db_mode = sys.argv[1]
    tag = sys.argv[2]
    
    base_path = os.path.join(project_root, "result", "prometheus", db_mode, tag)
    
    if not os.path.exists(base_path):
        print(f"ERROR: Path tidak ditemukan: {base_path}")
        sys.exit(1)
        
    summary_path = os.path.join(base_path, "summary.json")
    start_ts = 0
    end_ts = 0
    if os.path.exists(summary_path):
        with open(summary_path) as f:
            summary = json.load(f)
            start_ts = summary.get("start_ts", 0) * 1000
            end_ts = summary.get("end_ts", 0) * 1000

    panels = []
    pid = 1

    specs = [
        # (title, csv_file, x, y, w, unit, min_, max_)
        # Row System API
        (None,   "🖥  System — API Server (VM1)",            None,                       0,  0, 24, None, None,    None),
        ("CPU Usage — API Server",          "cpu_api.csv",              0,  1, 12, "percent",  0,   100),
        ("Memory Usage — API Server",       "memory_api.csv",          12,  1, 12, "decbytes", 0,   None),
        ("Container CPU — API (Docker)",    "container_cpu_api.csv",    0,  9, 12, "percent",  0,   None),
        ("Container Memory — API (Docker)", "container_memory_api.csv",12,  9, 12, "decbytes", 0,   None),
        ("Network In — API Server",         "network_in_api.csv",       0, 17, 12, "Bps",      0,   None),
        ("Network Out — API Server",        "network_out_api.csv",     12, 17, 12, "Bps",      0,   None),
        # Row System DB
        (None,   "🗄  System — DB Server (VM2)",              None,                       0, 25, 24, None, None,    None),
        ("CPU Usage — DB Server",           "cpu_db.csv",               0, 26, 12, "percent",  0,   100),
        ("Memory Usage — DB Server",        "memory_db.csv",           12, 26, 12, "decbytes", 0,   None),
        ("Network In — DB Server",          "network_in_db.csv",        0, 34, 24, "Bps",      0,   None),
        # Row PG Connections
        (None,   "🐘  PostgreSQL — Connections",              None,                       0, 42, 24, None, None,    None),
        ("Total Active Connections",        "pg_connections_total.csv", 0, 43, 12, "short",    0,   None),
        ("Connections (Sum All DB)",        "pg_connections.csv",      12, 43, 12, "short",    0,   None),
        # Row PG Throughput
        (None,   "📊  PostgreSQL — Throughput & Locks",       None,                       0, 51, 24, None, None,    None),
        ("Transactions Committed (TPS)",    "pg_transactions.csv",      0, 52, 12, "ops",      0,   None),
        ("Rollbacks / sec",                 "pg_rollbacks.csv",        12, 52, 12, "ops",      0,   None),
        ("Deadlocks",                       "pg_deadlocks.csv",         0, 60, 12, "short",    0,   None),
        ("Locks",                           "pg_locks.csv",            12, 60, 12, "short",    0,   None),
        # Row PG Tuples
        (None,   "🗃  PostgreSQL — Tuples & Cache",           None,                       0, 68, 24, None, None,    None),
        ("Tuples Inserted / sec",           "pg_tup_inserted.csv",      0, 69, 12, "rps",      0,   None),
        ("Tuples Updated / sec",            "pg_tup_updated.csv",      12, 69, 12, "rps",      0,   None),
        ("Tuples Fetched / sec",            "pg_tup_fetched.csv",       0, 77, 12, "rps",      0,   None),
        ("Cache Hit Ratio",                 "pg_blk_hit_ratio.csv",    12, 77, 12, "percent",  0,   100),
    ]

    for spec in specs:
        if spec[0] is None:
            _, row_title, _, _, y, _, _, _, _ = spec
            panels.append(make_row(pid, row_title, y)); pid += 1
        else:
            title, csv_file, x, y, w, unit, min_, max_ = spec
            p = make_panel(base_path, pid, title, csv_file, x, y, w, 8, unit, min_, max_)
            if p:
                panels.append(p)
            pid += 1

    # UID must be <= 40 characters
    dashboard_uid = f"p-{db_mode}-{tag}"[:40]
    dashboard = {
        "annotations": {"list": []},
        "editable": True,
        "fiscalYearStartMonth": 0,
        "graphTooltip": 1,
        "links": [],
        "panels": panels,
        "refresh": "",
        "schemaVersion": 39,
        "tags": ["pos", db_mode, "inline"],
        "time": {"from": str(start_ts), "to": str(end_ts)},
        "timepicker": {},
        "timezone": "utc",
        "title": f"POS Replay — {db_mode.upper()} / {tag}",
        "uid": dashboard_uid,
        "version": 1
    }

    out_file = f"result_{db_mode}_{tag}.json"
    out_path = os.path.join(project_root, "monitoring", "grafana", "dashboards", out_file)
    with open(out_path, "w") as f:
        json.dump(dashboard, f, indent=2)

    size_kb = os.path.getsize(out_path) / 1024
    print(f"✅ Dashboard generated: {out_file}")
    print(f"   UID: {dashboard_uid}")
    print(f"   Size: {size_kb:.1f} KB")
    
    # Reload Grafana dashboards via API if possible
    try:
        print("\nMencoba reload dashboard di Grafana...")
        subprocess.run(
            ["curl", "-s", "-u", "admin:admin", "-X", "POST", "http://localhost:3000/api/admin/provisioning/dashboards/reload"],
            check=True, stdout=subprocess.DEVNULL
        )
        print("✅ Dashboards direload! Silakan cek Grafana.")
    except Exception as e:
        print(f"Gagal reload otomatis: {e}. Grafana akan mereload sendiri dalam ~30 detik.")

if __name__ == "__main__":
    main()
