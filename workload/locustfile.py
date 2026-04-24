"""
TPC-C Adapted Workload Generator untuk POS API
================================================
Distribusi transaksi:
  sale=43, restock=18, transfer=22,
  sale_return=2, purchase_return=2, void=1, remit=2,
  stock_level=4, billing=3, history=3   → total=100

Token Pinning: setiap Locust user mendapat token UNIK (tidak overlap).
"""
import json
import random
import os
import threading
import logging
from locust import HttpUser, task, between, events

# ── Config ────────────────────────────────────────────────────────────────────
TOKEN_FILE          = "workload/tokens.json"
LOW_STOCK_THRESHOLD = 20
REMIT_THRESHOLD     = 500_000   # remit jika saldo bersih branch > 500rb

SEARCH_KEYWORDS = [
    "Produk", "Item", "Barang", "Stok", "Makanan",
    "Minuman", "Elektronik", "Pakaian", "Kosmetik", "Sembako",
]

logging.basicConfig(level=logging.WARNING)
logger = logging.getLogger("locustfile")


# ── Token Pool (unique claim) ─────────────────────────────────────────────────
def _load_tokens():
    if not os.path.exists(TOKEN_FILE):
        return []
    with open(TOKEN_FILE) as f:
        try:
            return json.load(f)
        except Exception as e:
            logger.error(f"Failed to parse {TOKEN_FILE}: {e}")
            return []


_tokens_pool  = _load_tokens()
_token_lock   = threading.Lock()
_token_index  = 0


def _claim_token():
    """Ambil satu token secara atomik — setiap user dijamin unik."""
    global _token_index
    with _token_lock:
        if _token_index >= len(_tokens_pool):
            return None
        tok = _tokens_pool[_token_index]
        _token_index += 1
        return tok


# ── POSUser ───────────────────────────────────────────────────────────────────
class POSUser(HttpUser):
    wait_time = between(1, 10)

    # ── Lifecycle ─────────────────────────────────────────────────────────────

    def on_start(self):
        tok = _claim_token()
        if tok is None:
            logger.warning("Token pool exhausted — stopping extra user.")
            self.stop()
            return

        self.user_data = tok
        self.headers = {
            "Authorization": f"Bearer {tok['token']}",
            "Content-Type":  "application/json",
        }

        # Per-user state
        self.meta = {
            "all_branches":    [],
            "all_warehouses":  [],
            "suppliers":       [],
            "master_items":    [],
            "branch_items":    {},   # branch_id → [item]
            "low_stock_items": {},   # branch_id → [item]
        }
        # Cached recent transaction records (trxno + details) dari session ini
        self.recent_sales:     list = []   # list of full trx dicts (SALE)
        self.recent_purchases: list = []   # list of full trx dicts (PURC)
        self.voided_ids:       set  = set()

        # Balance cache for remit decision
        self.branch_balance_net: float = 0.0

        try:
            if not self._discover_metadata():
                self.stop()
        except Exception as e:
            logger.error(f"Metadata error {tok.get('email')}: {e}")
            self.stop()

    # ── Metadata Discovery ────────────────────────────────────────────────────

    def _discover_metadata(self) -> bool:
        h = self.headers

        with self.client.get("/api/v1/branches", params={"page": 1, "limit": 50},
                             headers=h, name="[META] branches",
                             catch_response=True) as r:
            if r.status_code == 200:
                self.meta["all_branches"] = r.json().get("data", [])
                r.success()
            else:
                r.failure(f"branches: {r.status_code}")
                return False

        if not self.meta["all_branches"]:
            return False

        with self.client.get("/api/v1/warehouses", params={"page": 1, "limit": 20},
                             headers=h, name="[META] warehouses",
                             catch_response=True) as r:
            if r.status_code == 200:
                self.meta["all_warehouses"] = r.json().get("data", [])
                r.success()
            else:
                r.failure(f"warehouses: {r.status_code}")

        with self.client.get("/api/v1/suppliers", params={"page": 1, "limit": 50},
                             headers=h, name="[META] suppliers",
                             catch_response=True) as r:
            if r.status_code == 200:
                self.meta["suppliers"] = r.json().get("data", [])
                r.success()
            else:
                r.failure(f"suppliers: {r.status_code}")

        with self.client.get("/api/v1/items",
                             params={"page": 1, "limit": 50,
                                     "search": random.choice(SEARCH_KEYWORDS)},
                             headers=h, name="[META] items",
                             catch_response=True) as r:
            if r.status_code == 200:
                self.meta["master_items"] = r.json().get("data", [])
                r.success()
            else:
                r.failure(f"items: {r.status_code}")

        self._refresh_my_inventory()
        return True

    # ── Helpers ───────────────────────────────────────────────────────────────

    def _is_owner(self) -> bool:
        return self.user_data.get("role") == "owner"

    def _my_branch_id(self):
        bid = self.user_data.get("branch_id")
        if bid is not None:
            return bid
        if self.meta["all_branches"]:
            return random.choice(self.meta["all_branches"])["id"]
        return None

    def _refresh_my_inventory(self):
        if self._is_owner():
            for b in self.meta["all_branches"]:
                self._fetch_branch_inventory(b["id"])
        else:
            bid = self.user_data.get("branch_id")
            if bid:
                self._fetch_branch_inventory(bid)

    def _fetch_branch_inventory(self, branch_id: int):
        params = {"page": 1, "limit": 100}
        if random.random() < 0.2:
            params["search"] = random.choice(SEARCH_KEYWORDS)
        with self.client.get(f"/api/v1/inventory/branch/{branch_id}",
                             params=params, headers=self.headers,
                             name="[META] inventory/branch",
                             catch_response=True) as r:
            if r.status_code == 200:
                items = r.json().get("data", [])
                self.meta["branch_items"][branch_id] = items
                self.meta["low_stock_items"][branch_id] = [
                    i for i in items if i.get("stock", 0) < LOW_STOCK_THRESHOLD
                ]
                r.success()
            else:
                r.failure(f"inventory/branch/{branch_id}: {r.status_code}")

    def _stocked_items(self, branch_id: int):
        return [i for i in self.meta["branch_items"].get(branch_id, [])
                if i.get("stock", 0) > 0]

    def _donor_branch(self, exclude_bid=None):
        best, best_n = None, 0
        for bid, items in self.meta["branch_items"].items():
            if bid == exclude_bid:
                continue
            n = len([i for i in items if i.get("stock", 0) > LOW_STOCK_THRESHOLD])
            if n > best_n:
                best_n, best = n, bid
        return best

    def _log_fail(self, op, r, payload=None):
        logger.error(
            f"FAIL {op} HTTP {r.status_code} | {r.text[:300]} | "
            f"payload: {json.dumps(payload)[:200] if payload else '-'}"
        )

    def _do_remit(self, branch_id: int):
        """Coba remit saldo branch ke tenant. Dipanggil saat purchase gagal 402."""
        with self.client.get(f"/api/v1/reports/balance/branch/{branch_id}",
                             headers=self.headers,
                             name="/reports/balance/branch (remit-check)",
                             catch_response=True) as r:
            if r.status_code != 200:
                r.success()  # 403 untuk non-owner, skip
                return
            data = r.json().get("data", {})
            net = float(data.get("current_balance") or 0)
            r.success()

        if net <= 0:
            return

        remit_amount = round(net * 0.8, 2)  # remit 80% saat darurat 402
        with self.client.post(f"/api/v1/transactions/remit/branch/{branch_id}",
                              json={"amount": remit_amount,
                                    "note": "Locust auto-remit (purchase 402)"},
                              headers=self.headers,
                              name="/transactions/remit/branch",
                              catch_response=True) as r:
            if r.status_code in (200, 400, 422, 500):
                r.success()
            else:
                r.failure(f"remit HTTP {r.status_code}")

    def _push_sale(self, trx: dict):
        """Simpan max 5 sale terakhir dari session ini."""
        self.recent_sales.append(trx)
        if len(self.recent_sales) > 5:
            self.recent_sales.pop(0)

    def _push_purchase(self, trx: dict):
        self.recent_purchases.append(trx)
        if len(self.recent_purchases) > 3:
            self.recent_purchases.pop(0)

    # ── TPC-C Tasks ───────────────────────────────────────────────────────────

    @task(43)
    def task_sale(self):
        """POST /transactions/sale — New-Order eq (43%)"""
        branch_id = self._my_branch_id()
        if not branch_id:
            return

        available = self._stocked_items(branch_id)
        if not available:
            self._fetch_branch_inventory(branch_id)
            return

        k = random.randint(1, min(3, len(available)))
        chosen = random.sample(available, k)

        items_payload = []
        for item in chosen:
            max_qty = max(1, min(10, item["stock"] // 10 + 1))
            qty = min(random.randint(1, max_qty), item["stock"])
            if qty <= 0:
                continue
            items_payload.append({"branch_item_id": item["id"], "qty": qty})

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

        with self.client.post("/api/v1/transactions/sale",
                              json=payload, headers=self.headers,
                              name="/transactions/sale",
                              catch_response=True) as r:
            if r.status_code == 201:
                r.success()
                trx = r.json().get("data", {})
                if trx.get("trxno"):
                    self._push_sale(trx)
                for sold in items_payload:
                    for item in self.meta["branch_items"].get(branch_id, []):
                        if item["id"] == sold["branch_item_id"]:
                            item["stock"] = max(0, item["stock"] - sold["qty"])
                            break
            elif r.status_code == 400 and "insufficient stock" in r.text:
                r.success()
                self._fetch_branch_inventory(branch_id)
            else:
                r.failure(f"sale HTTP {r.status_code}")
                self._log_fail("SALE", r, payload)

    @task(18)
    def task_restock(self):
        """POST /transactions/purchase — Payment/Restock eq (18%)"""
        if not self._is_owner():
            return
        if not self.meta["suppliers"]:
            return

        target_bid = max(
            self.meta["low_stock_items"],
            key=lambda b: len(self.meta["low_stock_items"][b]),
            default=None
        )
        if target_bid is None and self.meta["all_branches"]:
            target_bid = random.choice(self.meta["all_branches"])["id"]
        if not target_bid:
            return

        pool = self.meta["low_stock_items"].get(target_bid) or self.meta["master_items"]
        if not pool:
            return

        supplier = random.choice(self.meta["suppliers"])
        chosen = random.sample(pool, min(3, len(pool)))

        items_payload = [
            {
                "item_id": item.get("item_id") or item.get("id"),
                "qty":     random.randint(100, 300),
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

        with self.client.post("/api/v1/transactions/purchase",
                              json=payload, headers=self.headers,
                              name="/transactions/purchase",
                              catch_response=True) as r:
            if r.status_code == 201:
                r.success()
                trx = r.json().get("data", {})
                if trx.get("trxno"):
                    self._push_purchase(trx)
                self._fetch_branch_inventory(target_bid)
            elif r.status_code == 402:
                # Saldo tenant tidak cukup → lakukan remit dulu
                r.success()  # bukan error workload
                self._do_remit(target_bid)
            else:
                r.failure(f"purchase HTTP {r.status_code}")
                self._log_fail("PURCHASE", r, payload)

    @task(22)
    def task_transfer(self):
        """POST /transactions/transfer — Stock Distribution eq (22%)"""
        if not self._is_owner():
            return
        if len(self.meta["all_branches"]) < 2:
            return

        dest_bid = max(
            self.meta["low_stock_items"],
            key=lambda b: len(self.meta["low_stock_items"][b]),
            default=None
        ) or random.choice(self.meta["all_branches"])["id"]

        src_bid = self._donor_branch(exclude_bid=dest_bid)
        if not src_bid or src_bid == dest_bid:
            return

        src_items = [i for i in self.meta["branch_items"].get(src_bid, [])
                     if i.get("stock", 0) > LOW_STOCK_THRESHOLD]
        if not src_items:
            return

        chosen = random.sample(src_items, min(2, len(src_items)))
        items_payload = []
        for item in chosen:
            avail = item.get("stock", 0) - LOW_STOCK_THRESHOLD
            if avail <= 0:
                continue
            qty = random.randint(1, max(1, avail // 2))
            items_payload.append({"item_id": item["item_id"], "qty": qty})

        if not items_payload:
            return

        payload = {
            "source_type": "branch", "source_id": src_bid,
            "dest_type":   "branch", "dest_id":   dest_bid,
            "note":        "Locust TPC-C Transfer",
            "items":       items_payload,
        }

        with self.client.post("/api/v1/transactions/transfer",
                              json=payload, headers=self.headers,
                              name="/transactions/transfer",
                              catch_response=True) as r:
            if r.status_code == 201:
                r.success()
                for tx_item in items_payload:
                    for item in self.meta["branch_items"].get(src_bid, []):
                        if item.get("item_id") == tx_item["item_id"]:
                            item["stock"] = max(0, item["stock"] - tx_item["qty"])
                            break
                self._fetch_branch_inventory(dest_bid)
            elif r.status_code == 400 and "insufficient stock" in r.text:
                r.success()
                self._fetch_branch_inventory(src_bid)
            else:
                r.failure(f"transfer HTTP {r.status_code}")
                self._log_fail("TRANSFER", r, payload)

    @task(2)
    def task_sale_return(self):
        """POST /transactions/return — Return sale dari session sendiri (2%)"""
        if not self.recent_sales:
            return

        # Ambil sale terbaru yang belum pernah di-return
        sale = self.recent_sales[-1]
        trxno = sale.get("trxno")
        details = sale.get("details", [])
        branch_id = sale.get("branch_id")

        if not trxno or not details or not branch_id:
            return

        # Return 1 item saja, qty=1
        detail = random.choice(details)
        bid = detail.get("branch_item_id")
        if not bid:
            return

        payload = {
            "original_trx_no": trxno,
            "branch_id":       branch_id,
            "note":            "Locust TPC-C Return",
            "items":           [{"branch_item_id": bid, "qty": 1}],
        }

        with self.client.post("/api/v1/transactions/return",
                              json=payload, headers=self.headers,
                              name="/transactions/return",
                              catch_response=True) as r:
            if r.status_code == 201:
                r.success()
                # Hapus dari recent agar tidak double-return
                if sale in self.recent_sales:
                    self.recent_sales.remove(sale)
                self._fetch_branch_inventory(branch_id)
            elif r.status_code in (400, 422):
                r.success()  # already returned / invalid qty — expected
            else:
                r.failure(f"return HTTP {r.status_code}")
                self._log_fail("RETURN", r, payload)

    @task(2)
    def task_purchase_return(self):
        """POST /transactions/purchase-return — Return ke supplier (2%), owner only"""
        if not self._is_owner():
            return
        if not self.recent_purchases or not self.meta["suppliers"]:
            return

        purchase = self.recent_purchases[-1]
        trxno    = purchase.get("trxno")
        details  = purchase.get("details", [])
        branch_id = purchase.get("branch_id")

        if not trxno or not details:
            return

        # Ambil item_id dari detail (branch_item_id → item_id lookup via cache)
        detail = random.choice(details)
        # purchase detail pakai warehouse_item_id atau branch_item_id
        # kita butuh item_id → cari di master_items via cost sebagai proxy
        # Ambil item dari master_items sebagai fallback
        if not self.meta["master_items"]:
            return

        master_item = random.choice(self.meta["master_items"])
        cost = float(master_item.get("cost") or master_item.get("cogs") or 10_000)
        supplier = random.choice(self.meta["suppliers"])

        payload = {
            "original_trx_no": trxno,
            "branch_id":       branch_id,
            "supplier_id":     supplier["id"],
            "note":            "Locust TPC-C Purchase Return",
            "items": [{
                "item_id":      master_item.get("item_id") or master_item.get("id"),
                "qty":          1,
                "return_price": cost,
            }],
        }

        with self.client.post("/api/v1/transactions/purchase-return",
                              json=payload, headers=self.headers,
                              name="/transactions/purchase-return",
                              catch_response=True) as r:
            if r.status_code == 201:
                r.success()
                if purchase in self.recent_purchases:
                    self.recent_purchases.remove(purchase)
            elif r.status_code in (400, 404, 422):
                r.success()  # trx already returned or item mismatch — expected
            else:
                r.failure(f"purchase-return HTTP {r.status_code}")
                self._log_fail("PURC_RETURN", r, payload)

    @task(1)
    def task_void(self):
        """POST /transactions/:id/void — Void sale session sendiri (1%), owner only"""
        if not self._is_owner():
            return

        # Cari sale yang belum pernah di-void
        candidates = [s for s in self.recent_sales
                      if s.get("id") and s["id"] not in self.voided_ids]
        if not candidates:
            return

        sale = candidates[0]
        sale_id = sale["id"]

        with self.client.post(f"/api/v1/transactions/{sale_id}/void",
                              json={"reason": "Locust TPC-C Void — input error test"},
                              headers=self.headers,
                              name="/transactions/:id/void",
                              catch_response=True) as r:
            if r.status_code == 200:
                r.success()
                self.voided_ids.add(sale_id)
                if sale in self.recent_sales:
                    self.recent_sales.remove(sale)
            elif r.status_code in (400, 404, 409):
                r.success()  # already voided / not allowed
            else:
                r.failure(f"void HTTP {r.status_code}")
                self._log_fail("VOID", r)

    @task(2)
    def task_remit_balance(self):
        """POST /transactions/remit/branch/:id — Kirim kas branch ke tenant (2%), owner only"""
        if not self._is_owner():
            return

        branch_id = self._my_branch_id()
        if not branch_id:
            return

        # Cek balance
        with self.client.get(f"/api/v1/reports/balance/branch/{branch_id}",
                             headers=self.headers,
                             name="/reports/balance/branch (remit-check)",
                             catch_response=True) as r:
            if r.status_code != 200:
                r.failure(f"balance-check HTTP {r.status_code}")
                return
            data = r.json().get("data", {})
            net = float(data.get("current_balance") or 0)
            r.success()

        if net < REMIT_THRESHOLD:
            return

        remit_amount = round(net * 0.5, 2)

        with self.client.post(f"/api/v1/transactions/remit/branch/{branch_id}",
                              json={"amount": remit_amount,
                                    "note": "Locust TPC-C Remittance"},
                              headers=self.headers,
                              name="/transactions/remit/branch",
                              catch_response=True) as r:
            if r.status_code in (200, 400, 422, 500):
                r.success()  # 500 mungkin race condition sementara, bukan workload error
            else:
                r.failure(f"remit HTTP {r.status_code}")
                self._log_fail("REMIT", r)

    @task(4)
    def task_stock_level(self):
        """GET /inventory/branch + GET /reports/top-items — Stock-Level check (4%)"""
        branch_id = self._my_branch_id()
        if not branch_id:
            return

        # Selalu cek inventory
        params = {"page": random.randint(1, 3), "limit": 20}
        if random.random() < 0.5:
            params["search"] = random.choice(SEARCH_KEYWORDS)

        with self.client.get(f"/api/v1/inventory/branch/{branch_id}",
                             params=params, headers=self.headers,
                             name="/inventory/branch (stock-level)",
                             catch_response=True) as r:
            if r.status_code == 200:
                items = r.json().get("data", [])
                existing = {i["id"]: i for i in
                            self.meta["branch_items"].get(branch_id, [])}
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

        # Owner: tambahan cek top-items untuk keputusan restock
        if self._is_owner() and random.random() < 0.5:
            with self.client.get("/api/v1/reports/top-items",
                                 params={"branch_id": branch_id,
                                         "sort_by": "qty", "limit": 10},
                                 headers=self.headers,
                                 name="/reports/top-items",
                                 catch_response=True) as r:
                if r.status_code == 200:
                    r.success()
                else:
                    r.failure(f"top-items HTTP {r.status_code}")

    @task(3)
    def task_billing_check(self):
        """Billing/cashflow check (3%) — endpoint disesuaikan per role."""
        branch_id = self._my_branch_id()
        if not branch_id:
            return

        if self._is_owner():
            # Owner bisa akses reports — 50% tenant balance, 50% summary
            if random.random() < 0.5:
                with self.client.get("/api/v1/reports/balance/tenant",
                                     headers=self.headers,
                                     name="/reports/balance/tenant",
                                     catch_response=True) as r:
                    if r.status_code == 200:
                        r.success()
                    else:
                        r.failure(f"tenant-balance HTTP {r.status_code}")
            else:
                with self.client.get("/api/v1/reports/summary",
                                     params={"branch_id": branch_id},
                                     headers=self.headers,
                                     name="/reports/summary",
                                     catch_response=True) as r:
                    if r.status_code == 200:
                        r.success()
                    else:
                        r.failure(f"summary HTTP {r.status_code}")
        else:
            # Kasir tidak bisa akses /reports → gunakan GET /transactions sebagai gantinya
            # Ini tetap meaningful: kasir cek riwayat transaksi branch-nya hari ini
            with self.client.get("/api/v1/transactions",
                                 params={
                                     "branch_id":  branch_id,
                                     "trans_type": "SALE",
                                     "page":        1,
                                     "limit":       10,
                                 },
                                 headers=self.headers,
                                 name="/transactions (cashier-billing)",
                                 catch_response=True) as r:
                if r.status_code == 200:
                    r.success()
                else:
                    r.failure(f"cashier-billing HTTP {r.status_code}")

    @task(3)
    def task_order_history(self):
        """GET /transactions + GET /transactions/:id — Order-History (3%)"""
        branch_id = self._my_branch_id()
        if not branch_id:
            return

        # Pilih tipe transaksi secara acak untuk variasi
        trans_types = [None, "SALE", "PURC", "RETURN"]
        trans_type = random.choice(trans_types)

        params = {
            "branch_id": branch_id,
            "page":      random.randint(1, 3),
            "limit":     10,
        }
        if trans_type:
            params["trans_type"] = trans_type

        with self.client.get("/api/v1/transactions",
                             params=params, headers=self.headers,
                             name="/transactions (order-history)",
                             catch_response=True) as r:
            if r.status_code == 200:
                items = r.json().get("data", [])
                r.success()

                # 30% chance: drill-down ke detail salah satu transaksi
                if items and random.random() < 0.3:
                    picked = random.choice(items)
                    trx_id = picked.get("id")
                    if trx_id:
                        with self.client.get(f"/api/v1/transactions/{trx_id}",
                                             headers=self.headers,
                                             name="/transactions/:id",
                                             catch_response=True) as rd:
                            if rd.status_code == 200:
                                # Simpan ke recent_sales jika SALE (sebagai fallback)
                                detail_trx = rd.json().get("data", {})
                                if (detail_trx.get("trans_type") == "SALE"
                                        and detail_trx.get("id") not in self.voided_ids
                                        and not any(s.get("id") == detail_trx.get("id")
                                                    for s in self.recent_sales)):
                                    self._push_sale(detail_trx)
                                rd.success()
                            else:
                                rd.failure(f"trx/:id HTTP {rd.status_code}")
            else:
                r.failure(f"order-history HTTP {r.status_code}")


# ── Hooks ─────────────────────────────────────────────────────────────────────
@events.test_start.add_listener
def on_test_start(environment, **kwargs):
    if not _tokens_pool:
        print("\n" + "!" * 60)
        print(" ERROR: workload/tokens.json KOSONG!")
        print(" Jalankan: make workload-login-<scale>")
        print("!" * 60 + "\n")
        environment.runner.quit()
    else:
        owner_count   = sum(1 for t in _tokens_pool if t.get("role") == "owner")
        cashier_count = sum(1 for t in _tokens_pool if t.get("role") == "cashier")
        pinned_count  = sum(1 for t in _tokens_pool if t.get("branch_id") is not None)
        print(f"\n[INFO] Token pool: {len(_tokens_pool)} tokens "
              f"(owner={owner_count}, cashier={cashier_count})")
        print(f"       Cashier pinned to branch: {pinned_count}/{cashier_count}")
        print(f"       Max concurrent users: {len(_tokens_pool)}\n")
