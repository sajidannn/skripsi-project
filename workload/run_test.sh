#!/bin/bash
# ============================================================
#  POS TPC-C Workload Runner
#  Sesuai Proposal, Tabel 3.5 Parameter Sistem:
#    - Ramp-up : 10 user/detik
#    - Durasi  : 5 menit (300 detik)
# ============================================================

set -e

API_URL=${API_URL:-"http://192.168.10.183:8080"}
SCALE=${SCALE:-1}
USERS=${USERS:-50}
RUN_TIME=${RUN_TIME:-"5m"}
HEADLESS=${HEADLESS:-"true"}

# Spawn rate sesuai proposal: 10 user/detik
SPAWN_RATE=10

echo "=========================================================="
echo "  POS TPC-C Workload Runner"
echo "=========================================================="
echo "  API URL    : $API_URL"
echo "  Tenants    : $SCALE"
echo "  Users      : $USERS concurrent"
echo "  Spawn rate : $SPAWN_RATE user/detik"
echo "  Duration   : $RUN_TIME"
echo "  Headless   : $HEADLESS"
echo "=========================================================="
echo ""

# ── STEP 1: Login & cache JWT tokens ─────────────────────────
echo ">>> STEP 1: Login dan caching token JWT..."
export API_URL=$API_URL
python3 workload/login_generator.py $SCALE

if [ $? -ne 0 ]; then
    echo "ERROR: login_generator.py gagal. Pastikan API berjalan di $API_URL"
    exit 1
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

if [ "$HEADLESS" = "true" ]; then
    locust \
        -f workload/locustfile.py \
        --host="$API_URL" \
        --users="$USERS" \
        --spawn-rate="$SPAWN_RATE" \
        --run-time="$RUN_TIME" \
        --headless \
        --only-summary
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
        --run-time="$RUN_TIME"
fi
