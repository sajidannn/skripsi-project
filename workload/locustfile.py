"""
TPC-C Adapted Workload Generator untuk POS API
================================================
Sesuai Proposal Skripsi - Tabel 3.4 Rasio Workload:
  - Sale (New-Order equivalent)   : 65%
  - Transfer/Restock (Payment eq) : 23%
  - Stock-level check             : 4%
  - Billing check (cashflow)      : 4%
  - Order History                 : 4%

Keying time: 1-10 detik (antara transaksi), sesuai karakteristik TPC-C.

Request/Response disesuaikan dengan pos-api.postman_collection.json:
  - Sale   : POST /api/v1/transactions/sale   → branch_item_id + qty
  - Restock: POST /api/v1/transactions/purchase → item_id + qty + cost
  - Transfer: POST /api/v1/transactions/transfer → source/dest_type/id + items[item_id+qty]
  - Stock check: GET /api/v1/inventory/branch/:id
  - Billing : GET /api/v1/cashflow/branch (atau /transactions?trans_type=SALE)
  - History : GET /api/v1/transactions?branch_id={id}

RBAC:
  - Sale          : semua role (cashier, owner, manager)
  - Restock/Transfer: owner & manager only
  - Read-only     : semua role
"""
import json
import random
import os
import logging
from locust import HttpUser, task, between, events

# ── Configuration ─────────────────────────────────────────────────────────────
TOKEN_FILE = "workload/tokens.json"

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("locustfile")

# ── Token Loader ──────────────────────────────────────────────────────────────
def load_tokens():
    if not os.path.exists(TOKEN_FILE):
        return []
    with open(TOKEN_FILE, "r") as f:
        try:
            return json.load(f)
        except Exception as e:
            logger.error(f"Failed to parse {TOKEN_FILE}: {e}")
            return []

tokens_pool = load_tokens()

# ── POSUser ───────────────────────────────────────────────────────────────────
class POSUser(HttpUser):
    # TPC-C keying time: 1-10 detik antar transaksi
    wait_time = between(1, 10)

    def on_start(self):
        """Dijalankan saat user di-spawn."""
        self.headers = {}
        self.user_data = {"role": "guest", "token": "", "email": "", "tenant_id": 0}
        self.meta = {
            "branches":     [],   # list of {id, name}
            "warehouses":   [],   # list of {id, name}
            "suppliers":    [],   # list of {id, name}
            "master_items": [],   # list of {id, cost, ...}
            # branch_id → list of {id(=branch_item_id), item_id, stock, cost, final_price, ...}
            "branch_items": {},
        }

        if not tokens_pool:
            logger.error("No tokens found in tokens.json! Run login_generator first.")
            return

        # Pilih token secara acak (cycling pool)
        self.user_data = random.choice(tokens_pool)
        self.headers = {
            "Authorization": f"Bearer {self.user_data['token']}",
            "Content-Type":  "application/json",
        }

        logger.info(
            f"User spawned: {self.user_data['email']} "
            f"(Role: {self.user_data['role']})"
        )

        # Ambil metadata awal
        try:
            self._discover_metadata()
        except Exception as e:
            logger.error(
                f"Metadata discovery failed for {self.user_data['email']}: {e}"
            )

    # ── Metadata Discovery ────────────────────────────────────────────────────

    def _discover_metadata(self):
        """Ambil data master dari API untuk dipakai dalam transaksi."""
        h = self.headers

        # Branches
        with self.client.get(
            "/api/v1/branches", headers=h,
            name="[META] branches", catch_response=True
        ) as r:
            if r.status_code == 200:
                self.meta["branches"] = r.json().get("data", [])
            else:
                r.failure(f"branches: {r.status_code}")

        # Warehouses
        with self.client.get(
            "/api/v1/warehouses", headers=h,
            name="[META] warehouses", catch_response=True
        ) as r:
            if r.status_code == 200:
                self.meta["warehouses"] = r.json().get("data", [])

        # Suppliers
        with self.client.get(
            "/api/v1/suppliers", headers=h,
            name="[META] suppliers", catch_response=True
        ) as r:
            if r.status_code == 200:
                self.meta["suppliers"] = r.json().get("data", [])

        # Master Items (untuk purchase & transfer)
        with self.client.get(
            "/api/v1/items", headers=h,
            name="[META] items", catch_response=True
        ) as r:
            if r.status_code == 200:
                self.meta["master_items"] = r.json().get("data", [])

        # Branch Inventory (untuk sale & transfer — ambil semua branch sekarang)
        self._refresh_branch_inventory()

    def _refresh_branch_inventory(self):
        """
        Refresh stok branch lokal.
        Response BranchItemResponse fields yang penting:
          - id         : branch_item_id  → dipakai di Sale
          - item_id    : master item ID  → dipakai di Transfer
          - stock      : qty tersedia
          - cost       : HPP
          - final_price: harga jual
        """
        for b in self.meta["branches"]:
            bid = b["id"]
            with self.client.get(
                f"/api/v1/inventory/branch/{bid}", headers=self.headers,
                name="[META] inventory/branch", catch_response=True
            ) as r:
                if r.status_code == 200:
                    # Simpan semua item apa adanya (sudah ada field yg kita butuhkan)
                    self.meta["branch_items"][bid] = r.json().get("data", [])

    # ── Helpers ───────────────────────────────────────────────────────────────

    def _log_fail(self, op: str, r, payload=None):
        """Cetak detail error ke terminal untuk debugging."""
        logger.error(
            f"\n{'='*50}\n"
            f"FAIL: {op} | HTTP {r.status_code}\n"
            f"Response : {r.text[:500]}\n"
            f"Payload  : {json.dumps(payload)[:500] if payload else '-'}\n"
            f"{'='*50}"
        )

    def _stocked_branch_items(self):
        """
        Kembalikan dict branch_id → [items_with_stock].
        Hanya branch yang punya setidaknya 1 item berstock.
        """
        result = {}
        for bid, items in self.meta["branch_items"].items():
            stocked = [i for i in items if i.get("stock", 0) > 0]
            if stocked:
                result[bid] = stocked
        return result

    # ── TPC-C Tasks ───────────────────────────────────────────────────────────

    @task(65)  # ≈New-Order: transaksi penjualan (tulis-berat)
    def task_sale(self):
        """
        POST /api/v1/transactions/sale
        Semua role bisa melakukan sale.
        Payload menggunakan branch_item_id (bukan item_id).
        """
        stocked = self._stocked_branch_items()
        if not stocked:
            # Kalau semua habis, refresh dulu lalu skip
            self._refresh_branch_inventory()
            return

        branch_id = random.choice(list(stocked.keys()))
        pool = stocked[branch_id]

        # Pilih 1-3 item secara acak
        k = random.randint(1, min(3, len(pool)))
        chosen = random.sample(pool, k)

        items_payload = []
        for item in chosen:
            max_qty = min(10, item["stock"])
            qty = random.randint(1, max_qty)
            items_payload.append({
                "branch_item_id": item["id"],   # field dari BranchItemResponse
                "qty": qty,
            })

        payload = {
            "branch_id":   branch_id,
            "customer_id": None,
            "tax":         0,
            "discount":    0,
            "note":        "Locust TPC-C Sale",
            "items":       items_payload,
        }

        with self.client.post(
            "/api/v1/transactions/sale",
            json=payload, headers=self.headers,
            name="/transactions/sale",
            catch_response=True,
        ) as r:
            if r.status_code == 201:
                r.success()
                # Optimistic stock decrement agar tidak over-sell
                for sold in items_payload:
                    for item in self.meta["branch_items"].get(branch_id, []):
                        if item["id"] == sold["branch_item_id"]:
                            item["stock"] = max(0, item["stock"] - sold["qty"])
                            break
            else:
                r.failure(f"sale HTTP {r.status_code}")
                self._log_fail("SALE", r, payload)

    @task(23)  # ≈Payment: restock & transfer untuk menjaga stok
    def task_restock_or_transfer(self):
        """
        Gabungan operasi restock (purchase) dan transfer.
        Hanya owner/manager yang bisa melakukan ini.
        Dipilih secara acak 50:50.
        """
        if self.user_data.get("role") not in ("owner", "manager"):
            return

        if random.random() < 0.5:
            self._do_purchase()
        else:
            self._do_transfer()

    def _do_purchase(self):
        """
        POST /api/v1/transactions/purchase
        Body: branch_id|warehouse_id, supplier_id, items[{item_id, qty, cost}]
        """
        if not self.meta["master_items"] or not self.meta["suppliers"]:
            return
        if not self.meta["branches"] and not self.meta["warehouses"]:
            return

        # Pilih lokasi tujuan (prefer branch agar sale bisa langsung pakai)
        if self.meta["branches"]:
            loc_key = "branch_id"
            loc_id  = random.choice(self.meta["branches"])["id"]
        else:
            loc_key = "warehouse_id"
            loc_id  = random.choice(self.meta["warehouses"])["id"]

        supplier = random.choice(self.meta["suppliers"])
        k = random.randint(1, min(3, len(self.meta["master_items"])))
        chosen = random.sample(self.meta["master_items"], k)

        items_payload = [
            {
                "item_id": item["id"],
                "qty":     random.randint(50, 200),        # restock besar agar stok tahan lama
                "cost":    int(float(item.get("cost") or 10000)),
            }
            for item in chosen
        ]

        payload = {
            loc_key:       loc_id,
            "supplier_id": supplier["id"],
            "tax":         0,
            "discount":    0,
            "note":        "Locust TPC-C Restock",
            "items":       items_payload,
        }

        with self.client.post(
            "/api/v1/transactions/purchase",
            json=payload, headers=self.headers,
            name="/transactions/purchase",
            catch_response=True,
        ) as r:
            if r.status_code == 201:
                r.success()
                # Refresh agar task_sale bisa pakai stok baru
                self._refresh_branch_inventory()
            else:
                r.failure(f"purchase HTTP {r.status_code}")
                self._log_fail("PURCHASE", r, payload)

    def _do_transfer(self):
        """
        POST /api/v1/transactions/transfer
        Body: source_type, source_id, dest_type, dest_id, items[{item_id, qty}]
        item_id di sini adalah master item ID (dari BranchItemResponse.item_id).
        """
        stocked = self._stocked_branch_items()
        if len(stocked) < 1 or len(self.meta["branches"]) < 2:
            return

        src_id = random.choice(list(stocked.keys()))
        # Pilih dest ≠ src
        dest_candidates = [b["id"] for b in self.meta["branches"] if b["id"] != src_id]
        if not dest_candidates:
            return
        dest_id = random.choice(dest_candidates)

        pool = stocked[src_id]
        k = random.randint(1, min(2, len(pool)))
        chosen = random.sample(pool, k)

        items_payload = [
            {
                "item_id": item["item_id"],                # master item ID
                "qty":     random.randint(1, min(20, item["stock"])),
            }
            for item in chosen
        ]

        payload = {
            "source_type": "branch",
            "source_id":   src_id,
            "dest_type":   "branch",
            "dest_id":     dest_id,
            "note":        "Locust TPC-C Transfer",
            "items":       items_payload,
        }

        with self.client.post(
            "/api/v1/transactions/transfer",
            json=payload, headers=self.headers,
            name="/transactions/transfer",
            catch_response=True,
        ) as r:
            if r.status_code == 201:
                r.success()
                # Optimistic: kurangi stok source
                for tx_item in items_payload:
                    for item in self.meta["branch_items"].get(src_id, []):
                        if item["item_id"] == tx_item["item_id"]:
                            item["stock"] = max(0, item["stock"] - tx_item["qty"])
                            break
            else:
                r.failure(f"transfer HTTP {r.status_code}")
                self._log_fail("TRANSFER", r, payload)

    @task(4)  # ≈Stock-Level: pemeriksaan ketersediaan stok
    def task_stock_level(self):
        """
        GET /api/v1/inventory/branch/:id
        Read-only, bisa dilakukan semua role.
        Setara dengan Stock-Level transaction pada TPC-C.
        """
        if not self.meta["branches"]:
            return
        branch = random.choice(self.meta["branches"])
        with self.client.get(
            f"/api/v1/inventory/branch/{branch['id']}",
            headers=self.headers,
            name="/inventory/branch (stock-level)",
            catch_response=True,
        ) as r:
            if r.status_code == 200:
                # Update local cache sekalian
                self.meta["branch_items"][branch["id"]] = r.json().get("data", [])
                r.success()
            else:
                r.failure(f"stock-level HTTP {r.status_code}")

    @task(4)  # ≈Delivery: billing/cashflow check
    def task_billing_check(self):
        """
        GET /api/v1/transactions?trans_type=SALE&branch_id={id}
        Representasi "billing" / cek saldo kas cabang.
        Setara dengan Delivery transaction pada TPC-C (read-heavy).
        """
        if not self.meta["branches"]:
            return
        branch = random.choice(self.meta["branches"])
        with self.client.get(
            "/api/v1/transactions",
            params={"trans_type": "SALE", "branch_id": branch["id"], "limit": 10},
            headers=self.headers,
            name="/transactions (billing-check)",
            catch_response=True,
        ) as r:
            if r.status_code == 200:
                r.success()
            else:
                r.failure(f"billing-check HTTP {r.status_code}")

    @task(4)  # ≈Order-Status: riwayat transaksi
    def task_order_history(self):
        """
        GET /api/v1/transactions?branch_id={id}
        Pemeriksaan riwayat transaksi.
        Setara dengan Order-Status transaction pada TPC-C.
        """
        if not self.meta["branches"]:
            return
        branch = random.choice(self.meta["branches"])
        with self.client.get(
            "/api/v1/transactions",
            params={"branch_id": branch["id"], "limit": 5},
            headers=self.headers,
            name="/transactions (order-history)",
            catch_response=True,
        ) as r:
            if r.status_code == 200:
                r.success()
            else:
                r.failure(f"order-history HTTP {r.status_code}")


# ── Hooks ─────────────────────────────────────────────────────────────────────
@events.test_start.add_listener
def on_test_start(environment, **kwargs):
    """Validasi token sebelum test dimulai."""
    if not tokens_pool:
        print("\n" + "!" * 60)
        print(" ERROR: workload/tokens.json KOSONG!")
        print(" Jalankan: python3 workload/login_generator.py <SCALE>")
        print("!" * 60 + "\n")
        environment.runner.quit()
    else:
        print(f"\n[INFO] Token pool loaded: {len(tokens_pool)} tokens\n")
