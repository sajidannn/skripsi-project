"""
TPC-C Adapted Workload Generator untuk POS API
================================================
Distribusi transaksi (sesuai proposal):
  - Sale + Restock (New-Order + Payment eq)  : 65%  (sale=45, restock=20)
  - Transfer (Stock Distribution eq)         : 23%
  - Stock-level check                        :  4%
  - Billing check (cashflow)                 :  4%
  - Order History                            :  4%

Desain anti-race-condition:
  - Setiap user "terpaku" (pinned) ke branch/warehouse MILIKNYA sendiri.
  - Kasir hanya bertransaksi di branch miliknya (branch_id dari token).
  - Owner mengelola semua branch (purchase & transfer antar branch).
  - Sebelum Sale: validasi stok lokal (optimistic cache) ≥ qty yang diminta.
  - Sebelum Transfer: validasi stok sumber ≥ qty yang akan dipindahkan.
  - Purchase (Restock): selalu ke lokasi milik sendiri.
  - Keying time: 1–10 detik (antar transaksi), sesuai karakteristik TPC-C.
"""
import json
import random
import os
import logging
from locust import HttpUser, task, between, events

# ── Configuration ─────────────────────────────────────────────────────────────
TOKEN_FILE = "workload/tokens.json"

# Minimum stock yang dianggap "aman" untuk di-jual.
# Di bawah threshold ini, kasir akan skip sale dan trigger restock.
LOW_STOCK_THRESHOLD = 20

# Kata kunci pencarian (simulasi kasir mengetik nama barang di UI)
SEARCH_KEYWORDS = [
    "Produk", "Item", "Barang", "Stok", "Makanan",
    "Minuman", "Elektronik", "Pakaian", "Kosmetik", "Sembako",
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
        """
        Inisialisasi user saat di-spawn.
        Setiap user akan:
          1. Mengambil token dari pool (sudah termasuk branch_id-nya)
          2. Mem-fetch metadata awal (branch miliknya, supplier, items)
          3. Mem-fetch inventory awal dari branch/wh miliknya
        """
        self.headers = {}
        self.user_data = {
            "role":         "guest",
            "token":        "",
            "email":        "",
            "tenant_id":    0,
            "branch_id":    None,   # branch lokal kasir (None = owner)
            "branch_index": None,
        }

        # Cache metadata per-user
        self.meta = {
            # Semua branch/wh milik tenant (diperlukan owner untuk transfer)
            "all_branches":   [],
            "all_warehouses": [],
            "suppliers":      [],
            "master_items":   [],

            # Inventori branch MILIK user ini (branch_id → list items)
            # Untuk kasir: hanya 1 entry (branch sendiri)
            # Untuk owner: semua branch
            "branch_items":     {},   # branch_id(int) → list of item dicts
            "low_stock_items":  {},   # branch_id(int) → list of low-stock item dicts
        }

        if not tokens_pool:
            logger.error("No tokens found in tokens.json! Run login_generator first.")
            self.environment.runner.quit()
            return

        # Ambil token dari pool
        self.user_data = random.choice(tokens_pool)
        self.headers = {
            "Authorization": f"Bearer {self.user_data['token']}",
            "Content-Type":  "application/json",
        }

        logger.info(
            f"User spawned: {self.user_data['email']} "
            f"(Role: {self.user_data['role']}, "
            f"Branch: {self.user_data.get('branch_id', 'ALL')})"
        )

        try:
            ok = self._discover_metadata()
            if not ok:
                logger.warning(
                    f"User {self.user_data['email']} tidak bisa ambil metadata. "
                    f"User di-stop."
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
        Fetch metadata awal dari API.
        - Ambil semua branch tenant (diperlukan untuk owner melakukan transfer)
        - Ambil inventori hanya dari branch MILIK user (pinned branch untuk kasir)
        Returns True jika berhasil (minimal ada 1 branch yang bisa diakses).
        """
        h = self.headers

        # ── Semua Branch (tenant-wide) ─────────────────────────────────────────
        with self.client.get(
            "/api/v1/branches",
            params={"page": 1, "limit": 50},
            headers=h,
            name="[META] branches",
            catch_response=True,
        ) as r:
            if r.status_code == 200:
                self.meta["all_branches"] = r.json().get("data", [])
                r.success()
            else:
                r.failure(f"branches: {r.status_code}")
                return False

        if not self.meta["all_branches"]:
            return False

        # ── Semua Warehouse (tenant-wide, diperlukan owner untuk transfer) ─────
        with self.client.get(
            "/api/v1/warehouses",
            params={"page": 1, "limit": 20},
            headers=h,
            name="[META] warehouses",
            catch_response=True,
        ) as r:
            if r.status_code == 200:
                self.meta["all_warehouses"] = r.json().get("data", [])
                r.success()
            else:
                r.failure(f"warehouses: {r.status_code}")

        # ── Suppliers ──────────────────────────────────────────────────────────
        with self.client.get(
            "/api/v1/suppliers",
            params={"page": 1, "limit": 50},
            headers=h,
            name="[META] suppliers",
            catch_response=True,
        ) as r:
            if r.status_code == 200:
                self.meta["suppliers"] = r.json().get("data", [])
                r.success()
            else:
                r.failure(f"suppliers: {r.status_code}")

        # ── Master Items (untuk purchase & transfer) ───────────────────────────
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

        # ── Inventory: hanya branch MILIK user ini ─────────────────────────────
        self._refresh_my_inventory()
        return True

    def _my_branch_id(self):
        """
        Kembalikan branch_id milik user ini.
        - Kasir: branch_id dari token (sudah di-pin)
        - Owner: pilih acak dari semua branch tenant
        """
        bid = self.user_data.get("branch_id")
        if bid is not None:
            return bid
        # Owner: pilih satu branch secara acak dari semua yang ada
        if self.meta["all_branches"]:
            return random.choice(self.meta["all_branches"])["id"]
        return None

    def _refresh_my_inventory(self):
        """
        Ambil/perbarui inventori HANYA dari branch milik user.
        Kasir: 1 branch → 1 request.
        Owner: cek semua branch (untuk bisa decide mana yang perlu restock/transfer).
        """
        role = self.user_data.get("role", "cashier")

        if role == "cashier":
            # Kasir hanya perlu inventori branch-nya sendiri
            bid = self.user_data.get("branch_id")
            if bid:
                self._fetch_branch_inventory(bid)
        else:
            # Owner perlu tahu kondisi semua branch untuk keputusan transfer
            for branch in self.meta["all_branches"]:
                self._fetch_branch_inventory(branch["id"])

    def _fetch_branch_inventory(self, branch_id: int):
        """Fetch dan cache inventori dari satu branch tertentu."""
        params = {"page": 1, "limit": 100}
        # 20% kemungkinan kasir search barang spesifik (realism UI)
        if random.random() < 0.2:
            params["search"] = random.choice(SEARCH_KEYWORDS)

        with self.client.get(
            f"/api/v1/inventory/branch/{branch_id}",
            params=params,
            headers=self.headers,
            name="[META] inventory/branch",
            catch_response=True,
        ) as r:
            if r.status_code == 200:
                items = r.json().get("data", [])
                self.meta["branch_items"][branch_id] = items
                self.meta["low_stock_items"][branch_id] = [
                    i for i in items if i.get("stock", 0) < LOW_STOCK_THRESHOLD
                ]
                r.success()
            else:
                r.failure(f"inventory/branch/{branch_id}: {r.status_code}")

    # ── Helpers ───────────────────────────────────────────────────────────────

    def _log_fail(self, op: str, r, payload=None):
        """Cetak detail error untuk debugging."""
        logger.error(
            f"\n{'='*50}\n"
            f"FAIL: {op} | HTTP {r.status_code}\n"
            f"Response : {r.text[:500]}\n"
            f"Payload  : {json.dumps(payload)[:500] if payload else '-'}\n"
            f"{'='*50}"
        )

    def _stocked_items_for_branch(self, branch_id: int):
        """Kembalikan items di branch tertentu yang masih punya stok > 0."""
        return [
            i for i in self.meta["branch_items"].get(branch_id, [])
            if i.get("stock", 0) > 0
        ]

    def _get_donor_branch(self, exclude_bid=None):
        """
        Cari branch dengan stok paling banyak untuk jadi sumber transfer.
        Exclude branch milik diri sendiri (agar tidak transfer ke diri sendiri).
        """
        best_bid = None
        best_count = 0
        for bid, items in self.meta["branch_items"].items():
            if bid == exclude_bid:
                continue
            stocked = [i for i in items if i.get("stock", 0) > LOW_STOCK_THRESHOLD]
            if len(stocked) > best_count:
                best_count = len(stocked)
                best_bid = bid
        return best_bid

    # ── TPC-C Tasks ───────────────────────────────────────────────────────────
    #
    # Bobot total = 100, distribusi:
    #   sale=45, restock=20, transfer=23, stock_level=4, billing=4, history=4
    # sale + restock = 65% (≈New-Order + Payment)
    # transfer = 23% (≈Stock Distribution)
    # read tasks = 12% (4+4+4)

    @task(45)
    def task_sale(self):
        """
        POST /api/v1/transactions/sale  ← New-Order equivalent (45%)

        Transaksi penjualan dari branch MILIK kasir/owner.
        Validasi stok lokal SEBELUM submit ke API untuk meminimalkan
        kemungkinan race condition dengan kasir lain di branch sama.
        Kasir pinned ke 1 branch → hanya 1 kasir per branch → race condition minimal.
        """
        branch_id = self._my_branch_id()
        if branch_id is None:
            return

        available = self._stocked_items_for_branch(branch_id)
        if not available:
            # Stok habis → refresh lalu skip giliran ini
            self._fetch_branch_inventory(branch_id)
            return

        # Pilih 1–3 item dari branch ini
        k = random.randint(1, min(3, len(available)))
        chosen = random.sample(available, k)

        items_payload = []
        for item in chosen:
            # Ambil maksimal 10% dari stok yang ada, minimum 1
            # Ini memastikan tidak langsung menguras semua stok dalam satu transaksi
            max_qty = max(1, min(10, item["stock"] // 10 + 1))
            qty = random.randint(1, max_qty)
            # Pre-validate: pastikan qty tidak melebihi stok yang kita tahu
            if qty > item["stock"]:
                qty = item["stock"]
            if qty <= 0:
                continue
            items_payload.append({
                "branch_item_id": item["id"],
                "qty":            qty,
            })

        if not items_payload:
            return

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
                # Optimistic decrement: kurangi cache lokal agar tidak over-sell
                for sold in items_payload:
                    for item in self.meta["branch_items"].get(branch_id, []):
                        if item["id"] == sold["branch_item_id"]:
                            item["stock"] = max(0, item["stock"] - sold["qty"])
                            break
            elif r.status_code == 400 and "insufficient stock" in r.text:
                # Stok sudah berubah sejak kita cek → refresh cache
                r.success()  # Ini bukan failure workload, ini kondisi normal TPC-C
                self._fetch_branch_inventory(branch_id)
            else:
                r.failure(f"sale HTTP {r.status_code}")
                self._log_fail("SALE", r, payload)

    @task(20)
    def task_restock(self):
        """
        POST /api/v1/transactions/purchase  ← Payment/Restock equivalent (20%)

        Hanya owner yang bisa melakukan purchase (restock dari supplier).
        Owner melihat branch mana yang paling kekurangan stok, lalu
        me-restock branch tersebut.
        Jika semua branch aman, owner pilih branch acak dan restock moderat.
        """
        if self.user_data.get("role") not in ("owner",):
            # Kasir tidak bisa purchase → skip, task weight sudah diperhitungkan
            return

        if not self.meta["suppliers"]:
            return

        # Cari branch yang paling butuh restock (paling banyak low-stock item)
        target_bid = None
        max_low = 0
        for bid, low_items in self.meta["low_stock_items"].items():
            if len(low_items) > max_low:
                max_low = len(low_items)
                target_bid = bid

        # Jika tidak ada yang low-stock, pilih branch acak
        if target_bid is None and self.meta["all_branches"]:
            target_bid = random.choice(self.meta["all_branches"])["id"]

        if target_bid is None:
            return

        # Tentukan barang yang akan di-restock
        # Prioritas: item yang low-stock, fallback ke master items
        pool = self.meta["low_stock_items"].get(target_bid, [])
        if not pool:
            pool = self.meta["master_items"]
        if not pool:
            return

        supplier = random.choice(self.meta["suppliers"])
        k = random.randint(1, min(3, len(pool)))
        chosen = random.sample(pool, k)

        items_payload = [
            {
                "item_id": item.get("item_id") or item.get("id"),
                "qty":     random.randint(100, 300),   # restock besar agar stok tahan lama
                "cost":    int(float(item.get("cost") or item.get("cogs") or 10_000)),
            }
            for item in chosen
        ]

        payload = {
            "branch_id":   target_bid,
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
                # Refresh inventory setelah restock agar sale bisa pakai stok baru
                self._fetch_branch_inventory(target_bid)
            else:
                r.failure(f"purchase HTTP {r.status_code}")
                self._log_fail("PURCHASE", r, payload)

    @task(23)
    def task_transfer(self):
        """
        POST /api/v1/transactions/transfer  ← Stock Distribution equivalent (23%)

        Hanya owner yang bisa transfer antar branch.
        Transfer HANYA dilakukan dari branch yang cukup berstock ke branch
        yang kekurangan. Pre-validasi stok sumber SEBELUM submit ke API.
        Pastikan source ≠ dest (API juga memvalidasi ini).
        """
        if self.user_data.get("role") not in ("owner",):
            return

        if len(self.meta["all_branches"]) < 2:
            return

        # Cari branch tujuan (yang paling butuh stok)
        dest_bid = None
        max_low = 0
        for bid, low_items in self.meta["low_stock_items"].items():
            if len(low_items) > max_low:
                max_low = len(low_items)
                dest_bid = bid

        # Jika tidak ada yang low-stock, pilih dest acak
        if dest_bid is None:
            dest_bid = random.choice(self.meta["all_branches"])["id"]

        # Cari branch sumber: branch lain yang punya stok lebih
        src_bid = self._get_donor_branch(exclude_bid=dest_bid)
        if src_bid is None or src_bid == dest_bid:
            return

        # Ambil item yang tersedia di sumber (stok > LOW_STOCK_THRESHOLD)
        src_items = [
            i for i in self.meta["branch_items"].get(src_bid, [])
            if i.get("stock", 0) > LOW_STOCK_THRESHOLD
        ]
        if not src_items:
            return

        k = random.randint(1, min(2, len(src_items)))
        chosen = random.sample(src_items, k)

        items_payload = []
        for item in chosen:
            # Validasi stok sumber: transfer maksimal separuh stok tersedia
            # agar sumber tidak langsung kering
            available = item.get("stock", 0) - LOW_STOCK_THRESHOLD
            if available <= 0:
                continue
            qty = random.randint(1, max(1, available // 2))
            items_payload.append({
                "item_id": item["item_id"],   # master item ID
                "qty":     qty,
            })

        if not items_payload:
            return

        payload = {
            "source_type": "branch",
            "source_id":   src_bid,
            "dest_type":   "branch",
            "dest_id":     dest_bid,
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
                # Optimistic: kurangi stok sumber di cache
                for tx_item in items_payload:
                    for item in self.meta["branch_items"].get(src_bid, []):
                        if item.get("item_id") == tx_item["item_id"]:
                            item["stock"] = max(0, item["stock"] - tx_item["qty"])
                            break
                # Tambah stok tujuan di cache (agar sale bisa pakai)
                self._fetch_branch_inventory(dest_bid)
            elif r.status_code == 400 and "insufficient stock" in r.text:
                # Stok sumber sudah berubah → refresh
                r.success()
                self._fetch_branch_inventory(src_bid)
            else:
                r.failure(f"transfer HTTP {r.status_code}")
                self._log_fail("TRANSFER", r, payload)

    @task(4)
    def task_stock_level(self):
        """
        GET /api/v1/inventory/branch/:id  ← Stock-Level check (4%)

        Kasir mengecek stok branch miliknya.
        Owner memilih branch acak untuk diinspeksi.
        Sekaligus memperbarui cache lokal.
        """
        branch_id = self._my_branch_id()
        if branch_id is None:
            return

        params = {"page": random.randint(1, 3), "limit": 20}
        if random.random() < 0.5:
            params["search"] = random.choice(SEARCH_KEYWORDS)

        with self.client.get(
            f"/api/v1/inventory/branch/{branch_id}",
            params=params,
            headers=self.headers,
            name="/inventory/branch (stock-level)",
            catch_response=True,
        ) as r:
            if r.status_code == 200:
                items = r.json().get("data", [])
                # Update cache (partial page — hanya replace halaman ini)
                if branch_id not in self.meta["branch_items"]:
                    self.meta["branch_items"][branch_id] = []
                # Merge: update items yang ada berdasarkan id
                existing = {i["id"]: i for i in self.meta["branch_items"][branch_id]}
                for item in items:
                    existing[item["id"]] = item
                self.meta["branch_items"][branch_id] = list(existing.values())
                self.meta["low_stock_items"][branch_id] = [
                    i for i in self.meta["branch_items"][branch_id]
                    if i.get("stock", 0) < LOW_STOCK_THRESHOLD
                ]
                r.success()
            else:
                r.failure(f"stock-level HTTP {r.status_code}")

    @task(4)
    def task_billing_check(self):
        """
        GET /api/v1/transactions?trans_type=SALE&branch_id=...  ← Billing check (4%)

        Kasir/owner cek riwayat penjualan branch miliknya.
        """
        branch_id = self._my_branch_id()
        if branch_id is None:
            return

        with self.client.get(
            "/api/v1/transactions",
            params={
                "trans_type": "SALE",
                "branch_id":  branch_id,
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

    @task(4)
    def task_order_history(self):
        """
        GET /api/v1/transactions?branch_id=...&page=...  ← Order-History (4%)

        Kasir/owner membuka riwayat transaksi dengan pagination.
        """
        branch_id = self._my_branch_id()
        if branch_id is None:
            return

        with self.client.get(
            "/api/v1/transactions",
            params={
                "branch_id": branch_id,
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
        owner_count   = sum(1 for t in tokens_pool if t.get("role") == "owner")
        cashier_count = sum(1 for t in tokens_pool if t.get("role") == "cashier")
        pinned_count  = sum(1 for t in tokens_pool if t.get("branch_id") is not None)
        print(f"\n[INFO] Token pool loaded: {len(tokens_pool)} tokens")
        print(f"       Owner: {owner_count} | Cashier: {cashier_count}")
        print(f"       Cashier pinned to branch: {pinned_count}/{cashier_count}\n")
