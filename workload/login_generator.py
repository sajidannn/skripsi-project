"""
Login Generator / JWT Pre-Caching
==================================
Script ini dipanggil oleh run_test.sh sebelum Locust dijalankan.
Tugasnya: Login SEMUA user (1 owner + 9 kasir) untuk setiap tenant dan
menyimpan token JWT ke workload/tokens.json.

UNIVERSAL (bekerja untuk singledb DAN multidb):
  Setelah owner berhasil login, script LANGSUNG menanyakan ke API:
  "Branch apa saja yang dimiliki tenant ini?" via GET /api/v1/branches.
  Branch ID yang ASLI dari API inilah yang kemudian di-assign ke kasir,
  bukan tebakan angka. Ini memastikan kasir selalu punya branch_id yang valid
  terlepas dari bagaimana database disusun (global ID vs per-tenant ID).

  small  → 5  tenant × 10 user = 50  token
  medium → 10 tenant × 10 user = 100 token
  large  → 50 tenant × 10 user = 500 token

Format token yang disimpan:
  {
    "tenant_id": 1,
    "email": "admin@tenant-001.test",
    "role": "owner",
    "branch_id": null,    # owner: null (akses semua branch)
    "token": "..."
  }

  {
    "tenant_id": 1,
    "email": "kasir.001.003@tenant-001.test",
    "role": "cashier",
    "branch_id": 42,      # ID ASLI dari API (bisa berbeda di singledb vs multidb!)
    "token": "..."
  }

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
API_URL = os.getenv("API_URL", "http://localhost:8080")
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
            print(f"  FAILED to login {email}: {resp.status_code} - {resp.text[:120]}")
    except Exception as e:
        print(f"  ERROR logging in {email}: {e}")
    return None


def fetch_tenant_branches(token: str, tenant_id: int) -> list:
    """
    Ambil daftar branch ASLI milik tenant ini dari API.
    Menggunakan token owner yang baru saja di-generate.
    Ini bekerja baik di singledb (ID global) maupun multidb (ID per-tenant).

    Returns:
        list of branch dicts: [{"id": 42, "name": "Cabang-001-01"}, ...]
        list kosong jika gagal.
    """
    url = f"{API_URL}/api/v1/branches"
    headers = {
        "Authorization": f"Bearer {token}",
        "Content-Type":  "application/json",
    }
    try:
        # Ambil semua branch (limit besar agar pasti dapat semua 9 branch)
        resp = requests.get(url, params={"page": 1, "limit": 50}, headers=headers, timeout=10)
        if resp.status_code == 200:
            branches = resp.json().get("data", [])
            return branches
        else:
            print(f"  WARN: Gagal fetch branches untuk tenant {tenant_id}: "
                  f"HTTP {resp.status_code} - {resp.text[:100]}")
    except Exception as e:
        print(f"  WARN: Exception saat fetch branches tenant {tenant_id}: {e}")
    return []


def main():
    try:
        num_tenants = int(sys.argv[1])
        total_users_target = int(sys.argv[2])
    except (ValueError, IndexError):
        print("Usage: python login_generator.py <num_tenants> <total_users>")
        print("  Example: python login_generator.py 10 100")
        sys.exit(1)

    # Strategi: Setiap tenant wajib 1 owner, sisanya adalah kasir
    if total_users_target < num_tenants:
        print(f"ERROR: total_users ({total_users_target}) minimal harus sama dengan num_tenants ({num_tenants})")
        sys.exit(1)

    owners_needed = num_tenants
    cashiers_needed = total_users_target - owners_needed
    
    base_cashiers_per_t = cashiers_needed // num_tenants
    extra_cashiers = cashiers_needed % num_tenants

    print(f"Generating tokens for {num_tenants} tenants...")
    print(f"  Target Total Users  : {total_users_target}")
    print(f"  Distribusi per Tenant:")
    print(f"    - Owner           : 1 per tenant (Total: {owners_needed})")
    print(f"    - Kasir (base)    : {base_cashiers_per_t} per tenant")
    if extra_cashiers > 0:
        print(f"    - Kasir (extra)   : +1 untuk {extra_cashiers} tenant pertama")
    print()

    tokens = []
    branch_verification_failures = 0

    for tenant_id in range(1, num_tenants + 1):
        print(f"[Tenant {tenant_id:03d}]")

        # ── STEP 1: Login Owner ────────────────────────────────────────────────
        admin_email = f"admin@tenant-{tenant_id:03}.test"
        owner_token = login_user(admin_email, "password123", tenant_id)

        if not owner_token:
            print(f"  SKIP: Owner gagal login, skip seluruh tenant {tenant_id}.")
            branch_verification_failures += 1
            continue

        tokens.append({
            "tenant_id": tenant_id,
            "email":     admin_email,
            "role":      "owner",
            "branch_id": None,   # owner tidak terikat ke satu branch
            "token":     owner_token,
        })
        print(f"  ✓ Owner  : {admin_email}")

        # ── STEP 2: Verifikasi Branch ASLI dari API ────────────────────────────
        # Inilah kunci universalnya: kita tidak menebak ID branch.
        # Kita tanya langsung ke API menggunakan token owner.
        real_branches = fetch_tenant_branches(owner_token, tenant_id)

        if not real_branches:
            print(f"  WARN: Tidak ada branch ditemukan untuk tenant {tenant_id}. "
                  f"Kasir akan di-skip, tapi owner tetap disimpan.")
            branch_verification_failures += 1
            # Masih lanjut untuk login kasir, tapi tanpa branch_id
            # (kasir akan bertindak seperti owner: pilih branch saat runtime)
            for branch_idx in range(1, CASHIERS_PER_TENANT + 1):
                cashier_email = f"kasir.{tenant_id:03}.{branch_idx:03}@tenant-{tenant_id:03}.test"
                token = login_user(cashier_email, "cashier123", tenant_id)
                if token:
                    tokens.append({
                        "tenant_id": tenant_id,
                        "email":     cashier_email,
                        "role":      "cashier",
                        "branch_id": None,   # tidak bisa di-pin, branch tidak terverifikasi
                        "token":     token,
                    })
                    print(f"  ✓ Kasir  : {cashier_email} → branch_id=UNVERIFIED")
            continue

        # Tampilkan branch yang ditemukan untuk verifikasi manual
        branch_ids = [b["id"] for b in real_branches]
        print(f"  ✓ Branch : {len(real_branches)} branch ditemukan → IDs: {branch_ids}")

        # ── STEP 3: Login Kasir & Pin ke Branch ASLI ──────────────────────────
        # Tentukan berapa kasir untuk tenant ini
        my_cashiers_count = base_cashiers_per_t
        if tenant_id <= extra_cashiers:
            my_cashiers_count += 1

        for branch_idx in range(1, my_cashiers_count + 1):
            cashier_email = f"kasir.{tenant_id:03}.{branch_idx:03}@tenant-{tenant_id:03}.test"
            token = login_user(cashier_email, "cashier123", tenant_id)

            if token:
                # Ambil branch ID asli (0-indexed karena list Python)
                assigned_branch = real_branches[(branch_idx - 1) % len(real_branches)]
                assigned_branch_id = assigned_branch["id"]
                assigned_branch_name = assigned_branch.get("name", f"branch-{assigned_branch_id}")

                tokens.append({
                    "tenant_id": tenant_id,
                    "email":     cashier_email,
                    "role":      "cashier",
                    "branch_id": assigned_branch_id,   # ← ID ASLI dari API, bukan tebakan!
                    "token":     token,
                })
                print(f"  ✓ Kasir  : {cashier_email} → branch_id={assigned_branch_id} ({assigned_branch_name})")

        print()

    # ── Simpan ke file ─────────────────────────────────────────────────────────
    os.makedirs(os.path.dirname(OUTPUT_FILE), exist_ok=True)
    with open(OUTPUT_FILE, "w") as f:
        json.dump(tokens, f, indent=2)

    success = len(tokens)
    failed  = total_users_target - success
    print("=" * 50)
    print(f"SUCCESS : {success}/{total_users_target} tokens saved → {OUTPUT_FILE}")
    if branch_verification_failures > 0:
        print(f"WARN    : {branch_verification_failures} tenant(s) gagal verifikasi branch.")
    if failed > 0:
        print(f"MISSING : {failed} user(s) gagal login atau di-skip.")
        print(f"  → Cek: make db-reseed SCALE=<scale>")
    print("=" * 50)


if __name__ == "__main__":
    main()
