#!/bin/bash
# ============================================================================
#  POS Progressive Scale Test Runner
#  Menjalankan 3 skenario pengujian (small → medium → large) secara otomatis
#  tanpa reseed penuh. Setiap skenario menambah data di atas skenario sebelumnya.
#
#  Alur:
#    [1] Fresh seed small (5 tenant) di VM2 via SSH
#    [2] Login tenant 1-5    → tokens_small.json + tokens.json
#    [3] Locust test SMALL   (10 menit)
#    [4] Additive seed +5 tenant (total 10) di VM2 via SSH
#    [5] Login tenant 6-10   → tokens_medium_new.json → merge → tokens.json
#    [6] Locust test MEDIUM  (10 menit)
#    [7] Additive seed +40 tenant (total 50) di VM2 via SSH
#    [8] Login tenant 11-50  → tokens_large_new.json → merge → tokens.json
#    [9] Locust test LARGE   (10 menit)
#
#  Penggunaan:
#    DB_MODE=multi make workload-progressive-multi
#    DB_MODE=single make workload-progressive-single
# ============================================================================

set -e

# ── Konfigurasi ───────────────────────────────────────────────────────────────
API_URL=${API_URL:-"http://localhost:8080"}
DB_MODE=${DB_MODE:-"multi"}
RUN_TIME=${RUN_TIME:-"10m"}
PROMETHEUS_URL=${PROMETHEUS_URL:-"http://localhost:9090"}
SPAWN_RATE=10

# SSH ke VM1 (API)
VM1_USER=${VM1_USER:-"sajidan"}
VM1_IP=${VM1_IP:-"192.168.10.183"}
VM1_PROJECT_DIR=${VM1_PROJECT_DIR:-"/home/sajidan/skripsi-project"}

# SSH ke VM2 (DB & Seeder)
VM2_USER=${VM2_USER:-"sajidan"}
VM2_IP=${VM2_IP:-"192.168.10.243"}
VM2_PROJECT_DIR=${VM2_PROJECT_DIR:-"/home/sajidan/skripsi-project"}
SSH_KEY=${SSH_KEY:-""}  # Optional: path ke SSH key, kosong = pakai default

# Tentukan file docker-compose seeder berdasarkan mode
if [ "$DB_MODE" = "multi" ]; then
    DC_FILE="DB/docker-compose.multi.yml"
    CONTAINER_NAME="pos-seeder-multi"
else
    DC_FILE="DB/docker-compose.single.yml"
    CONTAINER_NAME="pos-seeder-single"
fi

# Direktori output token (per skema, disimpan terpisah)
TOKEN_DIR="workload"
TOKEN_SMALL="${TOKEN_DIR}/tokens_small.json"
TOKEN_MEDIUM_NEW="${TOKEN_DIR}/tokens_medium_new.json"
TOKEN_LARGE_NEW="${TOKEN_DIR}/tokens_large_new.json"
TOKEN_ACTIVE="${TOKEN_DIR}/tokens.json"

# ── Fungsi Helper ─────────────────────────────────────────────────────────────
ssh_cmd() {
    local target_host=$1
    shift
    # Jalankan perintah di host target via SSH
    if [ -n "$SSH_KEY" ]; then
        ssh -i "$SSH_KEY" -o StrictHostKeyChecking=no "$target_host" "$@"
    else
        ssh -o StrictHostKeyChecking=no "$target_host" "$@"
    fi
}

start_api_on_vm1() {
    echo ""
    echo ">>> [VM1] Memastikan API menyala dalam mode: ${DB_MODE}..."
    ssh_cmd "${VM1_USER}@${VM1_IP}" "cd ${VM1_PROJECT_DIR} && make vm1-${DB_MODE}-up"
    echo ">>> [VM1] API berhasil dinyalakan."
}

seed_on_vm2() {
    local scale=$1
    local additive=${2:-"false"}
    local from_tenant=${3:-0}

    # Terjemahkan additive (true/false) ke string kosong atau "true"
    local seed_add_val=""
    if [ "$additive" = "true" ]; then
        seed_add_val="true"
    fi

    echo ""
    echo ">>> [VM2] Menjalankan 'make vm2-${DB_MODE}-up' (scale=${scale}, additive=${additive})..."

    # Jalankan via make agar exporters ikut nyala
    ssh_cmd "${VM2_USER}@${VM2_IP}" "set -e && cd ${VM2_PROJECT_DIR} && \
        make vm2-${DB_MODE}-up \
        SCALE=${scale} \
        SEED_ADDITIVE=${seed_add_val} \
        SEED_FROM_TENANT=${from_tenant}"

    echo ">>> [VM2] Seeder & Exporters berhasil diproses."
}

# ── Verifikasi jumlah tenant di DB sesuai ekspektasi ──────────────────────────
# Pastikan data benar-benar tersimpan sebelum tes dimulai.
verify_tenant_count() {
    local expected=$1
    echo ">>> [VERIFY] Memeriksa jumlah tenant di DB: expected=${expected}..."

    local count
    if [ "$DB_MODE" = "multi" ]; then
        count=$(ssh_cmd "${VM2_USER}@${VM2_IP}" \
            "docker exec pg-pos-multi psql -U postgres -d pos_master -t \
            -c 'SELECT COUNT(*) FROM tenants;' 2>/dev/null | tr -d ' \n'")
    else
        count=$(ssh_cmd "${VM2_USER}@${VM2_IP}" \
            "docker exec pg-pos-single psql -U postgres -d pos_single -t \
            -c 'SELECT COUNT(*) FROM tenants;' 2>/dev/null | tr -d ' \n'")
    fi

    if [ "$count" = "$expected" ]; then
        echo "  ✓ Verifikasi berhasil: ${count}/${expected} tenant tersimpan di DB."
    else
        echo ""
        echo "  ╔══════════════════════════════════════════════════════════════╗"
        echo "  ║  ERROR: Verifikasi DB GAGAL! Hentikan eksekusi.             ║"
        echo "  ║  Expected : ${expected} tenant                              ║"
        echo "  ║  Actual   : ${count:-'<tidak bisa koneksi ke DB>'}          ║"
        echo "  ║  Cek log seeder: docker logs ${CONTAINER_NAME}              ║"
        echo "  ╚══════════════════════════════════════════════════════════════╝"
        echo ""
        exit 1
    fi
}

# ── VACUUM ANALYZE setelah bulk INSERT ────────────────────────────────────────
# Tujuan: update statistik query planner dan paksa vacuum selesai SEKARANG
# agar autovacuum tidak menyala saat tes Locust berlangsung (bisa bikin spike I/O).
run_vacuum_analyze() {
    echo ">>> [VACUUM] Menjalankan VACUUM ANALYZE untuk menstabilkan DB stats..."

    if [ "$DB_MODE" = "multi" ]; then
        # Vacuum master DB (tabel routing tenants)
        ssh_cmd "${VM2_USER}@${VM2_IP}" \
            "docker exec pg-pos-multi psql -U postgres -d pos_master \
            -c 'VACUUM ANALYZE;' -q && echo '  ✓ VACUUM ANALYZE: pos_master'"

        # Vacuum semua tenant DB yang tercatat di master
        ssh_cmd "${VM2_USER}@${VM2_IP}" \
            "docker exec pg-pos-multi psql -U postgres -d pos_master -t \
            -c 'SELECT db_name FROM tenants;' 2>/dev/null | \
            tr -d ' ' | grep '^pos_' | while read db; do
                docker exec pg-pos-multi psql -U postgres -d \"\$db\" \
                    -c 'VACUUM ANALYZE;' -q 2>/dev/null &&
                    echo \"  ✓ VACUUM ANALYZE: \$db\";
            done"
    else
        ssh_cmd "${VM2_USER}@${VM2_IP}" \
            "docker exec pg-pos-single psql -U postgres -d pos_single \
            -c 'VACUUM ANALYZE;' -q && echo '  ✓ VACUUM ANALYZE: pos_single'"
    fi

    echo "  ✓ VACUUM ANALYZE selesai. Statistik DB sudah diperbarui."
}

# ── Cooldown: tunggu sistem stabil sebelum timestamp tes dimulai ──────────────
# Mencegah spike CPU/IO dari proses seeding "bocor" ke data Prometheus saat tes.
# Meskipun timestamp Locust baru dimulai setelah ini, tetap penting agar kondisi
# baseline sistem benar-benar bersih sebelum tes pertama.
cooldown_and_stabilize() {
    local seconds=${COOLDOWN_SECONDS:-60}
    echo ""
    echo "┌──────────────────────────────────────────────────────────────┐"
    echo "│  COOLDOWN ${seconds}s — Menunggu sistem stabil               │"
    echo "│  Seeding selesai, tapi CPU/IO mungkin masih tinggi.          │"
    echo "│  Timestamp Locust baru dimulai SETELAH cooldown ini.         │"
    echo "└──────────────────────────────────────────────────────────────┘"
    for ((i=seconds; i>0; i--)); do
        printf "\r  Sisa: %3ds " "$i"
        sleep 1
    done
    printf "\r  ✓ Cooldown selesai! Sistem siap, memulai pengujian...        \n"
    echo ""
}

run_locust_test() {
    local tag=$1
    local users=$2
    local scale_label=$3

    echo ""
    echo "============================================================"
    echo "  Menjalankan Locust Test: ${tag} | ${users} users | ${RUN_TIME}"
    echo "============================================================"

    TIMESTAMP=$(date +%Y%m%d_%H%M%S)
    PREFIX="${tag}_${scale_label}t_${users}u_${TIMESTAMP}"
    RESULT_DIR="result/locust/${DB_MODE}/${PREFIX}"
    mkdir -p "${RESULT_DIR}"

    TEST_START_TS=$(date +%s)

    locust \
        -f workload/locustfile.py \
        --host="${API_URL}" \
        --users="${users}" \
        --spawn-rate="${SPAWN_RATE}" \
        --run-time="${RUN_TIME}" \
        --headless \
        --reset-stats \
        --only-summary \
        --csv="${RESULT_DIR}/locust" \
        --html="${RESULT_DIR}/report.html" || true

    TEST_END_TS=$(date +%s)

    echo ""
    echo ">>> Locust selesai. Mengekspor metrik Prometheus..."
    python3 workload/export_metrics.py \
        --from  "${TEST_START_TS}" \
        --to    "${TEST_END_TS}" \
        --tag   "${PREFIX}" \
        --db-mode "${DB_MODE}" \
        --prometheus "${PROMETHEUS_URL}" \
        || echo "  WARN: export_metrics gagal."

    echo ""
    echo "  ✓ Hasil tersimpan:"
    echo "    - Locust CSV/HTML : ${RESULT_DIR}/"
    echo "    - Prometheus CSV  : result/prometheus/${DB_MODE}/${PREFIX}/"
}

merge_tokens() {
    # Gabungkan beberapa file token JSON menjadi satu tokens.json aktif
    # Usage: merge_tokens file1.json file2.json ...
    local files=("$@")
    echo ">>> Menggabungkan token → ${TOKEN_ACTIVE}"
    
    # Kirim path file sebagai argumen ke python (lebih aman daripada interpolasi string)
    python3 - "${files[@]}" <<'PYEOF'
import json, sys

output_file = "workload/tokens.json"
input_files = sys.argv[1:]

all_tokens = []
for fpath in input_files:
    if not fpath: continue
    try:
        with open(fpath) as fp:
            data = json.load(fp)
            all_tokens.extend(data)
            print(f"  + {fpath}: {len(data)} tokens")
    except Exception as e:
        print(f"  WARN: Gagal baca {fpath}: {e}")

with open(output_file, "w") as fp:
    json.dump(all_tokens, fp, indent=2)
print(f"  ✓ Total: {len(all_tokens)} tokens → {output_file}")
PYEOF
}

# ── Verifikasi Prasyarat ──────────────────────────────────────────────────────
echo "============================================================"
echo "  POS Progressive Scale Test Runner"
echo "  Mode DB      : ${DB_MODE}"
echo "  API URL      : ${API_URL}"
echo "  Durasi tes   : ${RUN_TIME} per skenario"
echo "  VM2          : ${VM2_USER}@${VM2_IP} (dir: ${VM2_PROJECT_DIR})"
echo "============================================================"
echo ""

# Test koneksi SSH ke VM2
echo ">>> Memeriksa koneksi SSH ke VM1 & VM2..."
if ! ssh_cmd "${VM1_USER}@${VM1_IP}" "echo 'VM1 SSH OK'" > /dev/null 2>&1; then
    echo "ERROR: Tidak bisa SSH ke VM1 (${VM1_USER}@${VM1_IP})"
    exit 1
fi
if ! ssh_cmd "${VM2_USER}@${VM2_IP}" "echo 'VM2 SSH OK'" > /dev/null 2>&1; then
    echo "ERROR: Tidak bisa SSH ke VM2 (${VM2_USER}@${VM2_IP})"
    exit 1
fi
echo "  ✓ SSH ke VM1 & VM2 berhasil."

# Step 0: Nyalain API di VM1 dulu
start_api_on_vm1

# ╔══════════════════════════════════════════════════════════════╗
# ║  FASE 1: SMALL — 5 tenant, 50 user                         ║
# ╚══════════════════════════════════════════════════════════════╝
echo ""
echo "╔══════════════════════════════════════════════════════════════╗"
echo "║  FASE 1: SMALL (5 tenant, 50 user)                          ║"
echo "╚══════════════════════════════════════════════════════════════╝"

# Step 1a: Fresh seed small
seed_on_vm2 "small" "false"

# Step 1b: Verifikasi data di DB
verify_tenant_count 5

# Step 1c: VACUUM ANALYZE agar autovacuum tidak ganggu saat tes
run_vacuum_analyze

# Step 1d: Cooldown — beri waktu sistem stabil (CPU/IO turun ke baseline)
#          Timestamp Locust dimulai SETELAH cooldown ini selesai.
COOLDOWN_SECONDS=${COOLDOWN_SECONDS:-30} cooldown_and_stabilize

# Step 1e: Login semua tenant 1-5, simpan ke tokens_small.json
echo ">>> [LOGIN] Tenant 1-5 → ${TOKEN_SMALL}"
API_URL=${API_URL} python3 workload/login_generator.py 5 50 \
    --output "${TOKEN_SMALL}"

# Salin sebagai token aktif untuk tes small
cp "${TOKEN_SMALL}" "${TOKEN_ACTIVE}"
echo "  ✓ tokens.json = tokens_small.json (50 tokens)"

# Step 1f: Jalankan tes small
run_locust_test "progressive-small" 50 5

# ╔══════════════════════════════════════════════════════════════╗
# ║  FASE 2: MEDIUM — 10 tenant total, 100 user                ║
# ╚══════════════════════════════════════════════════════════════╝
echo ""
echo "╔══════════════════════════════════════════════════════════════╗"
echo "║  FASE 2: MEDIUM (10 tenant total, 100 user)                 ║"
echo "╚══════════════════════════════════════════════════════════════╝"

# Step 2a: Additive seed +5 tenant (dari tenant 6 s/d 10)
seed_on_vm2 "medium" "true" 5

# Step 2b: Verifikasi total tenant sekarang = 10
verify_tenant_count 10

# Step 2c: VACUUM ANALYZE pada DB yang baru ditambahi data
run_vacuum_analyze

# Step 2d: Cooldown sebelum tes medium
COOLDOWN_SECONDS=${COOLDOWN_SECONDS:-60} cooldown_and_stabilize

# Step 2e: Login hanya tenant baru (6-10), simpan ke tokens_medium_new.json
echo ">>> [LOGIN] Tenant baru (6-10) → ${TOKEN_MEDIUM_NEW}"
API_URL=${API_URL} python3 workload/login_generator.py 10 50 \
    --from-tenant 5 \
    --output "${TOKEN_MEDIUM_NEW}"

# Gabungkan tokens_small + tokens_medium_new → tokens.json aktif
merge_tokens "${TOKEN_SMALL}" "${TOKEN_MEDIUM_NEW}"

# Step 2f: Jalankan tes medium
run_locust_test "progressive-medium" 100 10

# ╔══════════════════════════════════════════════════════════════╗
# ║  FASE 3: LARGE — 50 tenant total, 200 user                 ║
# ╚══════════════════════════════════════════════════════════════╝
echo ""
echo "╔══════════════════════════════════════════════════════════════╗"
echo "║  FASE 3: LARGE (50 tenant total, 200 user)                  ║"
echo "╚══════════════════════════════════════════════════════════════╝"

# Step 3a: Additive seed +40 tenant (dari tenant 11 s/d 50)
seed_on_vm2 "large" "true" 10

# Step 3b: Verifikasi total tenant sekarang = 50
verify_tenant_count 50

# Step 3c: VACUUM ANALYZE — ini fase terbesar, paling penting di-vacuum
run_vacuum_analyze

# Step 3d: Cooldown — fase large butuh waktu lebih lama karena 40 DB baru
COOLDOWN_SECONDS=${COOLDOWN_SECONDS:-30} cooldown_and_stabilize

# Step 3e: Login hanya tenant baru (11-50), simpan ke tokens_large_new.json
echo ">>> [LOGIN] Tenant baru (11-50) → ${TOKEN_LARGE_NEW}"
API_URL=${API_URL} python3 workload/login_generator.py 50 160 \
    --from-tenant 10 \
    --output "${TOKEN_LARGE_NEW}"

# Gabungkan semua token → tokens.json aktif
merge_tokens "${TOKEN_SMALL}" "${TOKEN_MEDIUM_NEW}" "${TOKEN_LARGE_NEW}"

# Step 3f: Jalankan tes large
run_locust_test "progressive-large" 200 50

# ── Ringkasan Akhir ───────────────────────────────────────────────────────────
echo ""
echo "╔══════════════════════════════════════════════════════════════╗"
echo "║  ✅ PROGRESSIVE TEST SELESAI!                               ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""
echo "  Token files yang disimpan:"
echo "    - ${TOKEN_SMALL}        (tenant 1-5)"
echo "    - ${TOKEN_MEDIUM_NEW}   (tenant 6-10, hanya baru)"
echo "    - ${TOKEN_LARGE_NEW}    (tenant 11-50, hanya baru)"
echo ""
echo "  Hasil test tersimpan di:"
echo "    - result/locust/${DB_MODE}/progressive-small_*"
echo "    - result/locust/${DB_MODE}/progressive-medium_*"
echo "    - result/locust/${DB_MODE}/progressive-large_*"
echo "    - result/prometheus/${DB_MODE}/progressive-*"
echo ""
echo "  Untuk generate dashboard Grafana:"
echo "    python3 workload/generate_inline_dashboard.py ${DB_MODE} progressive-small_*"
echo "    python3 workload/generate_inline_dashboard.py ${DB_MODE} progressive-medium_*"
echo "    python3 workload/generate_inline_dashboard.py ${DB_MODE} progressive-large_*"
echo ""
