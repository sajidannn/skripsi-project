#!/usr/bin/env python3
"""
POS TPC-C Test Results Viewer
==============================
Membaca hasil export CSV Prometheus dan menghasilkan satu file HTML
dengan grafik interaktif (Plotly.js). Tidak perlu Grafana, tidak perlu Docker.

Usage:
  python3 workload/view_results.py <db_mode> <tag>
  python3 workload/view_results.py multi large_50t_200u_20260426_223603

  # Tanpa argumen: tampilkan semua folder yang tersedia
  python3 workload/view_results.py

Output:
  result/prometheus/<db_mode>/<tag>/report.html
  → Buka langsung di browser (double-click atau xdg-open)
"""

import csv
import json
import os
import sys
from collections import defaultdict
from datetime import datetime, timezone

# ── Konfigurasi Panel ─────────────────────────────────────────────────────────
# (filename_tanpa_csv, judul, unit_label, group, aggregate)
# aggregate: "sum" | "mean" | "max" | None (tampilkan semua series)
PANELS = [
    # System - API Server
    ("cpu_api",            "CPU Usage — API Server",          "%",      "System (API Server)", "mean"),
    ("memory_api",         "Memory Usage — API Server",       "bytes",  "System (API Server)", "mean"),
    ("network_in_api",     "Network In — API Server",         "Bps",    "System (API Server)", "mean"),
    ("network_out_api",    "Network Out — API Server",        "Bps",    "System (API Server)", "mean"),
    ("container_cpu_api",  "Container CPU — API Server",      "%",      "System (API Server)", "mean"),
    ("container_memory_api","Container Memory — API Server",  "bytes",  "System (API Server)", "mean"),
    # System - DB Server
    ("cpu_db",             "CPU Usage — DB Server",           "%",      "System (DB Server)",  "mean"),
    ("memory_db",          "Memory Usage — DB Server",        "bytes",  "System (DB Server)",  "mean"),
    ("network_in_db",      "Network In — DB Server",          "Bps",    "System (DB Server)",  "mean"),
    # PostgreSQL
    ("pg_connections",     "Active Connections (per DB)",     "conn",   "PostgreSQL",          None),
    ("pg_connections_total","Total Connections",              "conn",   "PostgreSQL",          "sum"),
    ("pg_transactions",    "Transactions Committed (TPS)",    "txn/s",  "PostgreSQL",          "sum"),
    ("pg_rollbacks",       "Rollbacks",                       "txn/s",  "PostgreSQL",          "sum"),
    ("pg_deadlocks",       "Deadlocks",                       "count",  "PostgreSQL",          "sum"),
    ("pg_locks",           "Locks",                           "count",  "PostgreSQL",          "sum"),
    ("pg_tup_inserted",    "Tuples Inserted/s",               "row/s",  "PostgreSQL Tuples",   "sum"),
    ("pg_tup_updated",     "Tuples Updated/s",                "row/s",  "PostgreSQL Tuples",   "sum"),
    ("pg_tup_fetched",     "Tuples Fetched/s",                "row/s",  "PostgreSQL Tuples",   "sum"),
    ("pg_blk_hit_ratio",   "Cache Hit Ratio",                 "%",      "PostgreSQL Cache",    "mean"),
]


def fmt_bytes(v):
    """Format bytes menjadi human-readable."""
    for unit in ["B", "KB", "MB", "GB"]:
        if abs(v) < 1024:
            return f"{v:.1f} {unit}"
        v /= 1024
    return f"{v:.1f} TB"


def read_csv(path):
    """Baca CSV, return dict: label -> (timestamps[], values[])"""
    series = defaultdict(lambda: ([], []))
    try:
        with open(path, newline="", encoding="utf-8") as f:
            reader = csv.DictReader(f)
            for row in reader:
                try:
                    ts = int(float(row["timestamp"])) * 1000  # ms untuk Plotly
                    val = float(row["value"])
                    label = row.get("labels", "").strip() or "total"
                    series[label][0].append(ts)
                    series[label][1].append(val)
                except (ValueError, KeyError):
                    continue
    except FileNotFoundError:
        pass
    return dict(series)


def aggregate_series(series_dict, method):
    """Gabungkan semua series menjadi satu dengan method sum/mean."""
    if not series_dict:
        return {}
    all_ts = sorted(set(ts for tss, _ in series_dict.values() for ts in tss))
    # Buat lookup ts -> value per series
    lookup = {}
    for label, (tss, vals) in series_dict.items():
        lookup[label] = dict(zip(tss, vals))

    combined_vals = []
    for ts in all_ts:
        vals = [lookup[l].get(ts, 0) for l in series_dict]
        if method == "sum":
            combined_vals.append(sum(vals))
        else:  # mean
            combined_vals.append(sum(vals) / len(vals) if vals else 0)

    return {"aggregated": (all_ts, combined_vals)}


def make_traces_json(key, unit, aggregate, folder_path):
    """Hasilkan list Plotly trace sebagai JSON string."""
    csv_path = os.path.join(folder_path, f"{key}.csv")
    raw = read_csv(csv_path)
    if not raw:
        return "[]", False

    if aggregate and len(raw) > 1:
        data = aggregate_series(raw, aggregate)
        show_legend = False
    elif aggregate and len(raw) == 1:
        data = raw
        show_legend = False
    else:
        data = raw
        show_legend = len(data) > 1

    traces = []
    for i, (label, (tss, vals)) in enumerate(data.items()):
        # Format values untuk tooltip
        if unit == "bytes":
            text = [fmt_bytes(v) for v in vals]
            display_vals = [v / (1024 * 1024) for v in vals]  # → MB
            yaxis_unit = "MB"
        else:
            text = [f"{v:.2f} {unit}" for v in vals]
            display_vals = vals
            yaxis_unit = unit

        # Buat nama yang lebih singkat untuk legend
        short_label = label
        if "datname=" in label:
            for part in label.split(","):
                if part.startswith("datname="):
                    short_label = part.split("=")[1]
                    break
        elif "instance=" in label:
            for part in label.split(","):
                if part.startswith("instance="):
                    short_label = part.split("=")[1]
                    break

        trace = {
            "x": tss,
            "y": display_vals,
            "text": text,
            "hovertemplate": "%{text}<br>%{x}<extra>" + short_label + "</extra>",
            "mode": "lines",
            "name": short_label if show_legend else key,
            "showlegend": show_legend,
            "line": {"width": 2},
            "type": "scattergl",
        }
        traces.append(trace)

    return json.dumps(traces), True


def generate_html(folder_path, summary):
    tag     = summary.get("tag", os.path.basename(folder_path))
    db_mode = summary.get("db_mode", "?")
    start_h = summary.get("start_human", "?")
    end_h   = summary.get("end_human", "?")
    dur_s   = summary.get("duration_s", 0)

    # Kelompokkan panels per group
    groups = {}
    for cfg in PANELS:
        key, title, unit, group, agg = cfg
        groups.setdefault(group, []).append(cfg)

    # Build panel HTML
    panels_js = []  # list of JS blocs
    panels_html = []

    panel_idx = 0
    for group, cfgs in groups.items():
        panels_html.append(f'<div class="section-header"><span class="section-icon">◈</span>{group}</div>')
        panels_html.append('<div class="row">')
        count_in_row = 0

        for cfg in cfgs:
            key, title, unit, _, agg = cfg
            div_id = f"panel_{panel_idx}"

            traces_json, has_data = make_traces_json(key, unit, agg, folder_path)

            display_unit = "MB" if unit == "bytes" else unit

            if has_data:
                panels_js.append(f"""
Plotly.newPlot('{div_id}',
  {traces_json},
  {{
    paper_bgcolor: '#1a1a2e',
    plot_bgcolor:  '#16213e',
    font: {{ color: '#e0e0e0', family: 'Inter, sans-serif', size: 11 }},
    margin: {{ t: 36, r: 16, b: 48, l: 56 }},
    xaxis: {{
      type: 'date',
      gridcolor: '#2d3561',
      linecolor: '#2d3561',
      tickformat: '%H:%M:%S',
      title: {{ text: 'Waktu (UTC)', font: {{ size: 10 }} }}
    }},
    yaxis: {{
      gridcolor: '#2d3561',
      linecolor: '#2d3561',
      title: {{ text: '{display_unit}', font: {{ size: 10 }} }},
      rangemode: 'tozero'
    }},
    title: {{
      text: '{title}',
      font: {{ size: 13, color: '#a0c4ff' }},
      x: 0.02,
      xanchor: 'left'
    }},
    legend: {{ bgcolor: 'rgba(0,0,0,0.3)', bordercolor: '#2d3561', borderwidth: 1, font: {{ size: 9 }} }},
    hovermode: 'x unified',
    hoverlabel: {{ bgcolor: '#0f3460', bordercolor: '#4a90e2', font: {{ color: '#fff', size: 11 }} }}
  }},
  {{ responsive: true, displayModeBar: true, displaylogo: false,
    modeBarButtonsToRemove: ['select2d', 'lasso2d', 'autoScale2d'] }}
);
""")
                panel_html = f'<div class="panel-card" id="{div_id}"></div>'
            else:
                panel_html = f'<div class="panel-card no-data"><div class="no-data-label">📂 {title}<br><span>Tidak ada data</span></div></div>'

            panels_html.append(panel_html)
            panel_idx += 1
            count_in_row += 1

            if count_in_row == 2:
                panels_html.append('</div><div class="row">')
                count_in_row = 0

        panels_html.append('</div>')

    panels_html_str = "\n".join(panels_html)
    panels_js_str   = "\n".join(panels_js)

    dur_min = dur_s // 60
    dur_sec = dur_s % 60

    html = f"""<!DOCTYPE html>
<html lang="id">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>POS TPC-C Result — {tag}</title>
  <script src="https://cdn.plot.ly/plotly-2.32.0.min.js" charset="utf-8"></script>
  <link rel="preconnect" href="https://fonts.googleapis.com" />
  <link href="https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700&display=swap" rel="stylesheet" />
  <style>
    *, *::before, *::after {{ box-sizing: border-box; margin: 0; padding: 0; }}
    :root {{
      --bg:       #0d0d1a;
      --surface:  #1a1a2e;
      --surface2: #16213e;
      --accent:   #4a90e2;
      --accent2:  #a0c4ff;
      --text:     #e0e0e0;
      --muted:    #8899bb;
      --border:   #2d3561;
      --success:  #4caf7d;
      --warn:     #f5a623;
      --radius:   10px;
    }}
    body {{
      background: var(--bg);
      color: var(--text);
      font-family: 'Inter', sans-serif;
      font-size: 14px;
      min-height: 100vh;
    }}

    /* Header */
    .header {{
      background: linear-gradient(135deg, #0f3460 0%, #16213e 50%, #0d0d1a 100%);
      border-bottom: 1px solid var(--border);
      padding: 28px 32px 24px;
    }}
    .header-top {{
      display: flex;
      align-items: center;
      gap: 14px;
      margin-bottom: 20px;
    }}
    .header-icon {{
      width: 44px; height: 44px;
      background: var(--accent);
      border-radius: 10px;
      display: flex; align-items: center; justify-content: center;
      font-size: 22px;
    }}
    .header h1 {{
      font-size: 22px; font-weight: 700;
      color: var(--accent2);
      letter-spacing: -0.3px;
    }}
    .header h1 small {{
      display: block;
      font-size: 13px;
      font-weight: 400;
      color: var(--muted);
      margin-top: 2px;
    }}
    .meta-cards {{
      display: flex;
      flex-wrap: wrap;
      gap: 12px;
    }}
    .meta-card {{
      background: rgba(255,255,255,0.05);
      border: 1px solid var(--border);
      border-radius: 8px;
      padding: 10px 18px;
      display: flex;
      flex-direction: column;
      gap: 2px;
    }}
    .meta-card .label {{
      font-size: 10px;
      text-transform: uppercase;
      letter-spacing: 0.8px;
      color: var(--muted);
    }}
    .meta-card .value {{
      font-size: 15px;
      font-weight: 600;
      color: var(--accent2);
    }}
    .badge {{
      display: inline-block;
      padding: 2px 10px;
      border-radius: 20px;
      font-size: 11px;
      font-weight: 600;
      background: var(--accent);
      color: white;
      vertical-align: middle;
    }}

    /* Content */
    .content {{ padding: 24px 32px 48px; max-width: 1400px; margin: 0 auto; }}

    /* Section header */
    .section-header {{
      font-size: 13px;
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: 1.2px;
      color: var(--accent);
      padding: 20px 0 10px;
      border-bottom: 1px solid var(--border);
      margin-bottom: 16px;
      display: flex;
      align-items: center;
      gap: 8px;
    }}
    .section-icon {{ font-size: 16px; opacity: 0.7; }}

    /* Panel grid */
    .row {{
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: 16px;
      margin-bottom: 16px;
    }}
    @media (max-width: 900px) {{ .row {{ grid-template-columns: 1fr; }} }}

    .panel-card {{
      background: var(--surface);
      border: 1px solid var(--border);
      border-radius: var(--radius);
      height: 300px;
      overflow: hidden;
      transition: border-color 0.2s;
    }}
    .panel-card:hover {{ border-color: var(--accent); }}

    .no-data {{
      display: flex;
      align-items: center;
      justify-content: center;
    }}
    .no-data-label {{
      text-align: center;
      color: var(--muted);
      font-size: 13px;
      line-height: 1.7;
    }}
    .no-data-label span {{ font-size: 11px; opacity: 0.6; }}

    /* Footer */
    .footer {{
      text-align: center;
      padding: 24px;
      color: var(--muted);
      font-size: 11px;
      border-top: 1px solid var(--border);
      margin-top: 32px;
    }}
  </style>
</head>
<body>

<header class="header">
  <div class="header-top">
    <div class="header-icon">📊</div>
    <div class="header">
      <h1>
        POS TPC-C Load Test — Result Report
        <small>Generated: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}</small>
      </h1>
    </div>
  </div>
  <div class="meta-cards">
    <div class="meta-card">
      <span class="label">Tag / Run</span>
      <span class="value">{tag}</span>
    </div>
    <div class="meta-card">
      <span class="label">DB Mode</span>
      <span class="value"><span class="badge">{db_mode.upper()}</span></span>
    </div>
    <div class="meta-card">
      <span class="label">Mulai (UTC)</span>
      <span class="value">{start_h}</span>
    </div>
    <div class="meta-card">
      <span class="label">Selesai (UTC)</span>
      <span class="value">{end_h}</span>
    </div>
    <div class="meta-card">
      <span class="label">Durasi</span>
      <span class="value">{dur_min}m {dur_sec}s</span>
    </div>
  </div>
</header>

<div class="content">
{panels_html_str}
</div>

<footer class="footer">
  POS TPC-C Load Test Result Viewer &mdash; dibuat otomatis oleh <code>workload/view_results.py</code>
</footer>

<script>
{panels_js_str}
</script>
</body>
</html>
"""
    return html


def list_available():
    script_dir   = os.path.dirname(os.path.abspath(__file__))
    project_root = os.path.dirname(script_dir)
    base         = os.path.join(project_root, "result", "prometheus")
    print("Folder hasil tes yang tersedia:\n")
    print(f"  {'DB Mode':<12} {'Tag'}")
    print(f"  {'-'*12} {'-'*50}")
    if os.path.isdir(base):
        for db in sorted(os.listdir(base)):
            db_path = os.path.join(base, db)
            if os.path.isdir(db_path) and not db_path.endswith("current"):
                for tag in sorted(os.listdir(db_path)):
                    if os.path.isdir(os.path.join(db_path, tag)):
                        print(f"  {db:<12} {tag}")
    print()
    print("Usage:")
    print("  python3 workload/view_results.py <db_mode> <tag>")
    print()
    print("Contoh:")
    print("  python3 workload/view_results.py multi large_50t_200u_20260426_223603")


def main():
    if len(sys.argv) < 3:
        list_available()
        sys.exit(0)

    db_mode = sys.argv[1]
    tag     = sys.argv[2]

    script_dir   = os.path.dirname(os.path.abspath(__file__))
    project_root = os.path.dirname(script_dir)
    folder_path  = os.path.join(project_root, "result", "prometheus", db_mode, tag)

    if not os.path.isdir(folder_path):
        print(f"ERROR: Folder tidak ditemukan: {folder_path}")
        list_available()
        sys.exit(1)

    # Baca summary
    summary = {}
    summary_path = os.path.join(folder_path, "summary.json")
    if os.path.exists(summary_path):
        with open(summary_path) as f:
            summary = json.load(f)
    summary.setdefault("tag", tag)
    summary.setdefault("db_mode", db_mode)

    print(f"Generating report untuk: [{db_mode}] {tag}")
    print(f"  Periode: {summary.get('start_human','?')} → {summary.get('end_human','?')}")
    print(f"  Durasi : {summary.get('duration_s', 0)} detik")
    print()

    html = generate_html(folder_path, summary)

    out_path = os.path.join(folder_path, "report.html")
    with open(out_path, "w", encoding="utf-8") as f:
        f.write(html)

    print(f"✅ Report berhasil dibuat:")
    print(f"   {out_path}")
    print()
    print("Buka di browser:")
    print(f"   xdg-open '{out_path}'")
    print(f"   # atau double-click file-nya di file manager")


if __name__ == "__main__":
    main()
