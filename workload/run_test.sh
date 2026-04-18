#!/bin/bash

# Configuration
API_URL=${API_URL:-"http://192.168.10.183:8080"}

# Get parameters from environment variables or defaults
SCALE=${SCALE:-1}
USERS=${USERS:-5}

echo "=========================================================="
echo "    POS TPC-C Workload Runner                             "
echo "=========================================================="
echo "Target: $API_URL"
echo "Scale:  $SCALE Tenant(s)"
echo "Users:  $USERS Concurrent Users"
echo "=========================================================="

echo ""
echo ">>> STEP 1: Login dan Caching Token JWT..."
export API_URL=$API_URL
python3 workload/login_generator.py $SCALE

if [ ! -f "workload/tokens.json" ]; then
    echo "ERROR: Gagal menghasilkan tokens.json. Pastikan API menyala di $API_URL"
    exit 1
fi

echo ""
echo ">>> STEP 2: Menjalankan Locust (Master-less)..."
echo "Note: Buka Dashboard Monitoring di Laptop kamu untuk melihat hasil."
echo "Tekan CTRL+C untuk berhenti."
echo ""

locust -f workload/locustfile.py --host=$API_URL --users=$USERS --spawn-rate=10
