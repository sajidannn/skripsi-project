"""
Login Generator / JWT Pre-Caching
==================================
Script ini dipanggil oleh run_test.sh sebelum Locust dijalankan.
Tugasnya: Login SEMUA user (1 owner + 9 kasir) untuk setiap tenant dan
menyimpan token JWT ke workload/tokens.json.

Semua scale kini menggunakan 3 warehouses × 3 branches = 9 branch per tenant.
Sehingga: 1 owner + 9 kasir = 10 user per tenant (TETAP untuk semua scale).
  small  → 5  tenant × 10 user = 50  token
  medium → 10 tenant × 10 user = 100 token
  large  → 50 tenant × 10 user = 500 token

Cara manggil: python3 workload/login_generator.py <jumlah_tenant>
Contoh:
  python3 workload/login_generator.py 5    # small
  python3 workload/login_generator.py 10   # medium
  python3 workload/login_generator.py 50   # large
"""
import requests
import json
import sys
import os

# Configuration
API_URL = os.getenv("API_URL", "http://192.168.10.183:8080")
OUTPUT_FILE = "workload/tokens.json"

# Jumlah kasir per tenant = WarehousesPerT × BranchesPerWH = 3 × 3 = 9
# HARUS SINKRON dengan DB/seeder/config.go!
CASHIERS_PER_TENANT = 9


def login_user(email: str, password: str, tenant_id: int):
    """Login ke API dan kembalikan token JWT, atau None jika gagal."""
    url = f"{API_URL}/api/v1/auth/login"
    try:
        resp = requests.post(
            url,
            json={"email": email, "password": password, "tenant_id": tenant_id},
            timeout=10,
        )
        if resp.status_code == 200:
            return resp.json().get("data", {}).get("token")
        else:
            print(f"FAILED to login {email}: {resp.status_code} - {resp.text[:120]}")
    except Exception as e:
        print(f"ERROR logging in {email}: {e}")
    return None


def main():
    if len(sys.argv) < 2:
        print("Usage: python login_generator.py <num_tenants>")
        print("  num_tenants: 5 (small) | 10 (medium) | 50 (large)")
        return

    try:
        num_tenants = int(sys.argv[1])
    except ValueError:
        print("num_tenants must be an integer (5, 10, 50)")
        sys.exit(1)

    total_users_expected = num_tenants * (1 + CASHIERS_PER_TENANT)
    print(f"Generating tokens for {num_tenants} tenants...")
    print(f"  Kasir per tenant    : {CASHIERS_PER_TENANT} (branch 001–{CASHIERS_PER_TENANT:03d})")
    print(f"  Total user (expect) : {total_users_expected} (1 owner + {CASHIERS_PER_TENANT} kasir per tenant)")
    print()

    tokens = []

    for tenant_id in range(1, num_tenants + 1):
        # ── Owner ──────────────────────────────────────────────────────────────
        admin_email = f"admin@tenant-{tenant_id:03}.test"
        token = login_user(admin_email, "password123", tenant_id)
        if token:
            tokens.append({
                "tenant_id": tenant_id,
                "email":     admin_email,
                "role":      "owner",
                "token":     token,
            })
            print(f"Authenticated: {admin_email}")

        # ── Kasir (1 per branch, 9 branch per tenant) ──────────────────────────
        for branch_idx in range(1, CASHIERS_PER_TENANT + 1):
            cashier_email = f"kasir.{tenant_id:03}.{branch_idx:03}@tenant-{tenant_id:03}.test"
            token = login_user(cashier_email, "cashier123", tenant_id)
            if token:
                tokens.append({
                    "tenant_id":  tenant_id,
                    "email":      cashier_email,
                    "role":       "cashier",
                    "token":      token,
                })
                print(f"Authenticated: {cashier_email}")

    # Simpan ke file
    os.makedirs(os.path.dirname(OUTPUT_FILE), exist_ok=True)
    with open(OUTPUT_FILE, "w") as f:
        json.dump(tokens, f, indent=2)

    success = len(tokens)
    failed  = total_users_expected - success
    print()
    print(f"SUCCESS: {success}/{total_users_expected} tokens saved to {OUTPUT_FILE}")
    if failed > 0:
        print(f"WARNING: {failed} user(s) gagal login.")
        print(f"  → Pastikan DB sudah di-reseed: make db-multi-reseed SCALE=large")


if __name__ == "__main__":
    main()
