import requests
import json
import sys
import os

# Configuration
API_URL = os.getenv("API_URL", "http://192.168.10.183:8080")
OUTPUT_FILE = "workload/tokens.json"

def login_user(email, password, tenant_id):
    url = f"{API_URL}/api/v1/auth/login"
    try:
        payload = {
            "email": email, 
            "password": password,
            "tenant_id": tenant_id
        }
        resp = requests.post(url, json=payload, timeout=5)
        if resp.status_code == 200:
            data = resp.json()
            return data.get("data", {}).get("token")
        else:
            print(f"FAILED to login {email}: {resp.status_code} - {resp.text}")
    except Exception as e:
        print(f"ERROR logging in {email}: {e}")
    return None

def main():
    if len(sys.argv) < 2:
        print("Usage: python login_generator.py <scale>")
        print("Scales: 1, 10, 50 (number of tenants)")
        return

    try:
        scale = int(sys.argv[1])
    except ValueError:
        print("Scale must be an integer (1, 10, 50)")
        return

    print(f"Generating tokens for {scale} tenants...")
    tokens = []

    for tenant_id in range(1, scale + 1):
        # 1. Login Admin (Owner)
        admin_email = f"admin@tenant-{tenant_id:03}.test"
        token = login_user(admin_email, "password123", tenant_id)
        if token:
            tokens.append({
                "tenant_id": tenant_id,
                "email": admin_email,
                "role": "owner",
                "token": token
            })
            print(f"Authenticated: {admin_email}")

        # 2. Login Cashiers (assuming 2 branches per tenant per seeder)
        for branch_idx in range(1, 3):
            cashier_email = f"kasir.{tenant_id:03}.{branch_idx:03}@tenant-{tenant_id:03}.test"
            token = login_user(cashier_email, "cashier123", tenant_id)
            if token:
                tokens.append({
                    "tenant_id": tenant_id,
                    "email": cashier_email,
                    "role": "cashier",
                    "token": token
                })
                print(f"Authenticated: {cashier_email}")

    with open(OUTPUT_FILE, "w") as f:
        json.dump(tokens, f, indent=2)
    
    print(f"\nSUCCESS: Saved {len(tokens)} tokens to {OUTPUT_FILE}")

if __name__ == "__main__":
    main()
