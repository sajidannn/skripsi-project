#!/bin/bash
# ============================================================
#  POS TPC-C Workload Runner
#  Sesuai Proposal, Tabel 3.5 Parameter Sistem:
#    - Ramp-up : 10 user/detik
#    - Durasi  : 5 menit (300 detik)
# ============================================================

set -e

API_URL=${API_URL:-"http://localhost:8080"}
SCALE=${SCALE:-1}
USERS=${USERS:-50}
RUN_TIME=${RUN_TIME:-"5m"}
HEADLESS=${HEADLESS:-"true"}
PROMETHEUS_URL=${PROMETHEUS_URL:-"http://localhost:9090"}
SKIP_LOGIN=${SKIP_LOGIN:-"false"}
DB_MODE=${DB_MODE:-"multi"}
TAG=${TAG:-"workload"}

# Spawn rate sesuai proposal: 10 user/detik
SPAWN_RATE=10

echo "=========================================================="
echo "  POS TPC-C Workload Runner"
echo "=========================================================="
echo "  API URL        : $API_URL"
echo "  Tenants        : $SCALE"
echo "  Users          : $USERS concurrent"
echo "  Spawn rate     : $SPAWN_RATE user/detik"
echo "  Duration       : $RUN_TIME"
echo "  Headless       : $HEADLESS"
echo "  DB Mode        : $DB_MODE"
echo "  Prometheus     : $PROMETHEUS_URL"
echo "=========================================================="
echo ""

# ── STEP 1: Login & cache JWT tokens ─────────────────────────
if [ "$SKIP_LOGIN" != "true" ]; then
    echo ">>> STEP 1: Login dan caching token JWT..."
    export API_URL=$API_URL
    python3 workload/login_generator.py $SCALE

    if [ $? -ne 0 ]; then
        echo "ERROR: login_generator.py gagal. Pastikan API berjalan di $API_URL"
        exit 1
    fi
else
    echo ">>> STEP 1: SKIP_LOGIN aktif. Menggunakan tokens.json yang ada..."
fi

if [ ! -f "workload/tokens.json" ]; then
    echo "ERROR: workload/tokens.json tidak ditemukan."
    exit 1
fi

TOKEN_COUNT=$(python3 -c "import json; print(len(json.load(open('workload/tokens.json'))))")
echo "INFO: $TOKEN_COUNT token tersimpan."
echo ""

# ── STEP 2: Jalankan Locust ───────────────────────────────────
echo ">>> STEP 2: Menjalankan Locust workload..."
echo ""

TIMESTAMP=$(date +%Y%m%d_%H%M%S)
PREFIX="${TAG}_${SCALE}t_${USERS}u_${TIMESTAMP}"

RESULT_DIR="result/locust/$DB_MODE/$PREFIX"
mkdir -p "$RESULT_DIR"

# Catat waktu mulai test (Unix timestamp) untuk export Prometheus nanti
TEST_START_TS=$(date +%s)

if [ "$HEADLESS" = "true" ]; then
    locust \
        -f workload/locustfile.py \
        --host="$API_URL" \
        --users="$USERS" \
        --spawn-rate="$SPAWN_RATE" \
        --run-time="$RUN_TIME" \
        --headless \
        --only-summary \
        --csv="$RESULT_DIR/locust" \
        --html="$RESULT_DIR/report.html" || true
else
    echo "Mode UI: Buka http://localhost:8089 di browser kamu."
    echo "  Users target  : $USERS"
    echo "  Spawn rate    : $SPAWN_RATE"
    echo "  Duration      : $RUN_TIME"
    echo "  Tekan CTRL+C untuk berhenti."
    echo ""
    locust \
        -f workload/locustfile.py \
        --host="$API_URL" \
        --users="$USERS" \
        --spawn-rate="$SPAWN_RATE" \
        --run-time="$RUN_TIME" \
        --csv="$RESULT_DIR/locust" \
        --html="$RESULT_DIR/report.html" || true
fi

# Catat waktu selesai
TEST_END_TS=$(date +%s)

echo ""
echo "=========================================================="
echo "  Locust selesai!"
echo "  - CSV : $RESULT_DIR/locust*.csv"
echo "  - HTML: $RESULT_DIR/report.html"
echo "=========================================================="
echo ""

# ── STEP 3: Export metrik Prometheus ─────────────────────────
echo ">>> STEP 3: Mengekspor metrik Prometheus (${TEST_START_TS} → ${TEST_END_TS})..."
python3 workload/export_metrics.py \
    --from  "$TEST_START_TS" \
    --to    "$TEST_END_TS" \
    --tag   "$PREFIX" \
    --db-mode "$DB_MODE" \
    --prometheus "$PROMETHEUS_URL" \
    || echo "  WARN: export_metrics gagal (Prometheus tidak tersedia atau belum jalan?)"

echo ""
echo "=========================================================="
echo "  Semua hasil tersimpan:"
echo "  - Locust CSV/HTML  : $RESULT_DIR/"
echo "  - Prometheus CSV   : result/prometheus/$DB_MODE/$PREFIX/"
echo "=========================================================="
