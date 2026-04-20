"""
TPC-C Adapted Workload Generator untuk POS API
================================================
Sesuai Proposal Skripsi - Tabel 3.4 Rasio Workload:
  - Sale (New-Order equivalent)      : 65%
  - Transfer/Restock (Payment eq)    : 23%
  - Stock-level check                :  4%
  - Billing check (cashflow)         :  4%
  - Order History                    :  4%

Keying time: 1–10 detik (antar transaksi), sesuai karakteristik TPC-C.

Perbaikan Realism:
  - Semua query metadata menggunakan pagination (page + limit)
  - Search filter menggunakan kata kunci acak (simulasi kasir mencari barang)
  - Purchase & Transfer dipicu dari hasil cek low_stock (lebih realistis)
  - Inventory browse menggunakan search + pagination (simulasi browsing nyata)
"""
import json
import random
import os
import logging
from locust import HttpUser, task, between, events

# ── Configuration ─────────────────────────────────────────────────────────────
TOKEN_FILE = "workload/tokens.json"

# Kata kunci pencarian (simulasi kasir yang mengetik nama/SKU barang)
# Diambil dari prefix pola nama item yang di-generate seeder
SEARCH_KEYWORDS = [
    "Produk", "Item", "Barang", "Stok", "SKU",
    "A", "B", "C", "D", "E", "M", "S",
]

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
    # TPC-C keying time: 1–10 detik antar transaksi
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
            # branch_id → list of {id(=branch_item_id), item_id, stock, cost, final_price}
            "branch_items": {},
            # branch_id → list of low-stock items (stock < threshold)
            "low_stock_items": {},
        }

        if not tokens_pool:
            logger.error("No tokens found in tokens.json! Run login_generator first.")
            self.environment.runner.quit()
            return

        # Pilih token secara acak dari pool
        self.user_data = random.choice(tokens_pool)
        self.headers = {
            "Authorization": f"Bearer {self.user_data['token']}",
            "Content-Type":  "application/json",
        }

        logger.info(
            f"User spawned: {self.user_data['email']} "
            f"(Role: {self.user_data['role']})"
        )

        # Ambil metadata awal — stop user jika gagal
        try:
            ok = self._discover_metadata()
            if not ok:
                logger.warning(
                    f"User {self.user_data['email']} tidak bisa ambil metadata "
                    f"(branches kosong / token invalid). User di-stop."
                )
                self.stop()
        except Exception as e:
            logger.error(
                f"Metadata discovery exception for {self.user_data['email']}: {e}"
            )
            self.stop()

    # ── Metadata Discovery ────────────────────────────────────────────────────

    def _discover_metadata(self) -> bool:
        """
        Ambil data master dari API (dengan pagination) untuk dipakai dalam transaksi.
        Returns:
            True  → berhasil, minimal ada 1 branch
            False → gagal (token tidak valid / tenant DB belum ada)
        """
        h = self.headers

        # Branches — indikator utama kesehatan koneksi tenant
        # Pakai page=1, limit=20 agar realistis seperti UI yang paginate
        with self.client.get(
            "/api/v1/branches",
            params={"page": 1, "limit": 20},
            headers=h,
            name="[META] branches",
            catch_response=True,
        ) as r:
            if r.status_code == 200:
                self.meta["branches"] = r.json().get("data", [])
                r.success()
            else:
                r.failure(f"branches: {r.status_code}")
                return False

        if not self.meta["branches"]:
            return False

        # Warehouses
        with self.client.get(
            "/api/v1/warehouses",
            params={"page": 1, "limit": 20},
            headers=h,
            name="[META] warehouses",
            catch_response=True,
        ) as r:
            if r.status_code == 200:
                self.meta["warehouses"] = r.json().get("data", [])
                r.success()
            else:
                r.failure(f"warehouses: {r.status_code}")

        # Suppliers
        with self.client.get(
            "/api/v1/suppliers",
            params={"page": 1, "limit": 20},
            headers=h,
            name="[META] suppliers",
            catch_response=True,
        ) as r:
            if r.status_code == 200:
                self.meta["suppliers"] = r.json().get("data", [])
                r.success()
            else:
                r.failure(f"suppliers: {r.status_code}")

        # Master Items (untuk purchase & transfer) — random search keyword + limit
        keyword = random.choice(SEARCH_KEYWORDS)
        with self.client.get(
            "/api/v1/items",
            params={"page": 1, "limit": 50, "search": keyword},
            headers=h,
            name="[META] items",
            catch_response=True,
        ) as r:
            if r.status_code == 200:
                self.meta["master_items"] = r.json().get("data", [])
                r.success()
            else:
                r.failure(f"items: {r.status_code}")

        # Branch Inventory (initial load)
        self._refresh_branch_inventory()
        return True

    def _refresh_branch_inventory(self):
        """
        Refresh stok branch lokal (dengan pagination + search acak untuk realism).
        Setiap panggilan juga memperbarui daftar low_stock_items.
        """
        for b in self.meta["branches"]:
            bid = b["id"]
            # Kadang browse dengan search keyword (simulasi kasir cari barang spesifik)
            params = {"page": 1, "limit": 100}
            if random.random() < 0.3:  # 30% kemungkinan pakai search
                params["search"] = random.choice(SEARCH_KEYWORDS)

            with self.client.get(
                f"/api/v1/inventory/branch/{bid}",
                params=params,
                headers=self.headers,
                name="[META] inventory/branch",
                catch_response=True,
            ) as r:
                if r.status_code == 200:
                    items = r.json().get("data", [])
                    self.meta["branch_items"][bid] = items
                    # Catat item yang low stock (stock < 20) untuk dipakai restock/transfer
                    self.meta["low_stock_items"][bid] = [
                        i for i in items if i.get("stock", 0) < 20
                    ]
                    r.success()
                else:
                    r.failure(f"inventory/branch: {r.status_code}")

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
        Kembalikan dict branch_id → [items dengan stock > 0].
        Hanya branch yang punya setidaknya 1 item berstock.
        """
        result = {}
        for bid, items in self.meta["branch_items"].items():
            stocked = [i for i in items if i.get("stock", 0) > 0]
            if stocked:
                result[bid] = stocked
        return result

    def _get_low_stock_branch(self):
        """
        Kembalikan (branch_id, [low_stock_items]) secara acak,
        atau None jika tidak ada branch dengan low-stock.
        """
        candidates = {
            bid: items
            for bid, items in self.meta["low_stock_items"].items()
            if items
        }
        if not candidates:
            return None, []
        bid = random.choice(list(candidates.keys()))
        return bid, candidates[bid]

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
            # Refresh stok jika semua habis, lalu skip giliran ini
            self._refresh_branch_inventory()
            return

        branch_id = random.choice(list(stocked.keys()))
        pool = stocked[branch_id]

        # Pilih 1–3 item secara acak
        k = random.randint(1, min(3, len(pool)))
        chosen = random.sample(pool, k)

        items_payload = []
        for item in chosen:
            max_qty = min(10, item["stock"])
            qty = random.randint(1, max_qty)
            items_payload.append({
                "branch_item_id": item["id"],   # BranchItemResponse.id
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
                # Optimistic stock decrement agar tidak over-sell di iterasi berikutnya
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
        Gabungan operasi purchase dan transfer.
        Hanya owner/manager yang bisa melakukan ini.
        Diprioritaskan ke branch yang punya low-stock items (lebih realistis).
        """
        if self.user_data.get("role") not in ("owner", "manager"):
            return

        # Cek apakah ada branch dengan low stock
        low_bid, low_items = self._get_low_stock_branch()

        if low_bid and random.random() < 0.6:
            # 60% → lakukan purchase ke branch yang low-stock
            self._do_purchase(target_branch_id=low_bid, low_stock_items=low_items)
        elif random.random() < 0.5:
            # 50% dari sisa → purchase biasa
            self._do_purchase()
        else:
            # sisanya → transfer antar branch
            self._do_transfer()

    def _do_purchase(self, target_branch_id=None, low_stock_items=None):
        """
        POST /api/v1/transactions/purchase
        Jika low_stock_items diberikan, barang yang dibeli adalah item low-stock.
        Body: branch_id, supplier_id, items[{item_id, qty, cost}]
        """
        if not self.meta["suppliers"]:
            return

        # Tentukan lokasi tujuan
        if target_branch_id:
            loc_key = "branch_id"
            loc_id  = target_branch_id
        elif self.meta["branches"]:
            loc_key = "branch_id"
            loc_id  = random.choice(self.meta["branches"])["id"]
        elif self.meta["warehouses"]:
            loc_key = "warehouse_id"
            loc_id  = random.choice(self.meta["warehouses"])["id"]
        else:
            return

        supplier = random.choice(self.meta["suppliers"])

        # Pilih item: utamakan low-stock, fallback ke master_items
        if low_stock_items:
            pool = low_stock_items
        elif self.meta["master_items"]:
            pool = self.meta["master_items"]
        else:
            return

        k = random.randint(1, min(3, len(pool)))
        chosen = random.sample(pool, k)

        items_payload = [
            {
                "item_id": item.get("item_id") or item.get("id"),
                "qty":     random.randint(50, 200),   # restock besar agar stok tahan lama
                "cost":    int(float(item.get("cost") or 10_000)),
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
                # Refresh inventory agar sale bisa pakai stok baru
                self._refresh_branch_inventory()
            else:
                r.failure(f"purchase HTTP {r.status_code}")
                self._log_fail("PURCHASE", r, payload)

    def _do_transfer(self):
        """
        POST /api/v1/transactions/transfer
        Transfer stok dari branch berstock ke branch yang low-stock.
        item_id di sini adalah master item ID (BranchItemResponse.item_id).
        """
        stocked = self._stocked_branch_items()
        low_bid, _ = self._get_low_stock_branch()

        if not stocked:
            return

        src_id = random.choice(list(stocked.keys()))

        # Lebih realistis: kirim ke branch yang low-stock jika ada
        if low_bid and low_bid != src_id:
            dest_id = low_bid
        else:
            dest_candidates = [b["id"] for b in self.meta["branches"] if b["id"] != src_id]
            if not dest_candidates:
                return
            dest_id = random.choice(dest_candidates)

        pool = stocked[src_id]
        k = random.randint(1, min(2, len(pool)))
        chosen = random.sample(pool, k)

        items_payload = [
            {
                "item_id": item["item_id"],    # master item ID
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

    @task(4)  # ≈Stock-Level: pemeriksaan stok + search (simulasi kasir nyari barang)
    def task_stock_level(self):
        """
        GET /api/v1/inventory/branch/:id?search=<keyword>&page=1&limit=20
        Simulasi kasir yang membuka halaman inventaris dan mengetik nama barang.
        Sekaligus memperbarui cache low_stock_items.
        """
        if not self.meta["branches"]:
            return

        branch = random.choice(self.meta["branches"])
        # 50% kemungkinan pakai search keyword, sisanya browse halaman random
        params = {"page": random.randint(1, 3), "limit": 20}
        if random.random() < 0.5:
            params["search"] = random.choice(SEARCH_KEYWORDS)

        with self.client.get(
            f"/api/v1/inventory/branch/{branch['id']}",
            params=params,
            headers=self.headers,
            name="/inventory/branch (stock-level)",
            catch_response=True,
        ) as r:
            if r.status_code == 200:
                items = r.json().get("data", [])
                # Update local cache
                self.meta["branch_items"][branch["id"]] = items
                self.meta["low_stock_items"][branch["id"]] = [
                    i for i in items if i.get("stock", 0) < 20
                ]
                r.success()
            else:
                r.failure(f"stock-level HTTP {r.status_code}")

    @task(4)  # ≈Delivery: billing / riwayat transaksi cabang
    def task_billing_check(self):
        """
        GET /api/v1/transactions?trans_type=SALE&branch_id={id}&page=1&limit=10
        Representasi "billing" / kasir cek transaksi penjualan hari ini.
        """
        if not self.meta["branches"]:
            return

        branch = random.choice(self.meta["branches"])
        with self.client.get(
            "/api/v1/transactions",
            params={
                "trans_type": "SALE",
                "branch_id":  branch["id"],
                "page":       1,
                "limit":      10,
            },
            headers=self.headers,
            name="/transactions (billing-check)",
            catch_response=True,
        ) as r:
            if r.status_code == 200:
                r.success()
            else:
                r.failure(f"billing-check HTTP {r.status_code}")

    @task(4)  # ≈Order-Status: riwayat transaksi dengan pagination
    def task_order_history(self):
        """
        GET /api/v1/transactions?branch_id={id}&page={p}&limit=5
        Simulasi pengelola yang membuka riwayat transaksi dan bolak-balik halaman.
        """
        if not self.meta["branches"]:
            return

        branch = random.choice(self.meta["branches"])
        with self.client.get(
            "/api/v1/transactions",
            params={
                "branch_id": branch["id"],
                "page":      random.randint(1, 3),
                "limit":     5,
            },
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
        print(" Jalankan: python3 workload/login_generator.py <num_tenants>")
        print("!" * 60 + "\n")
        environment.runner.quit()
    else:
        print(f"\n[INFO] Token pool loaded: {len(tokens_pool)} tokens\n")
