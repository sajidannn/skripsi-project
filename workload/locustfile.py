import json
import random
import os
import logging
from locust import HttpUser, task, between, events

# Configuration
TOKEN_FILE = "workload/tokens.json"

# Setup logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

def load_tokens():
    if not os.path.exists(TOKEN_FILE):
        return []
    with open(TOKEN_FILE, "r") as f:
        try:
            return json.load(f)
        except Exception as e:
            logger.error(f"Failed to parse {TOKEN_FILE}: {e}")
            return []

# Global pool of tokens
tokens_pool = load_tokens()

class POSUser(HttpUser):
    wait_time = between(1, 3) 

    def on_start(self):
        """Executed when a user is spawned."""
        # 1. Initialize attributes immediately
        self.headers = {}
        self.user_data = {"role": "guest", "token": "", "email": ""}
        self.metadata = {
            "branches": [],
            "warehouses": [],
            "branch_items": {},  # Maps branch_id -> list of items
            "master_items": [],
            "suppliers": []
        }

        if not tokens_pool:
            logger.error("No tokens found in tokens.json! Spawn aborted.")
            return

        # 2. Assign Token (Cycling)
        self.user_data = random.choice(tokens_pool)
        self.headers = {
            "Authorization": f"Bearer {self.user_data['token']}",
            "Content-Type": "application/json"
        }
        
        logger.info(f"User spawned: {self.user_data['email']} (Role: {self.user_data['role']})")
        
        # 3. Discover Metadata
        try:
            self._discover_metadata()
        except Exception as e:
            logger.error(f"Metadata discovery failed for {self.user_data['email']}: {e}")

    def _discover_metadata(self):
        """Fetch valid IDs for transactions, matching Postman paths."""
        
        # Branches
        with self.client.get("/api/v1/branches", headers=self.headers, name="[META] Get Branches", catch_response=True) as resp:
            if resp.status_code == 200:
                self.metadata["branches"] = resp.json().get("data", [])

        # Warehouses
        with self.client.get("/api/v1/warehouses", headers=self.headers, name="[META] Get Warehouses", catch_response=True) as resp:
            if resp.status_code == 200:
                self.metadata["warehouses"] = resp.json().get("data", [])

        # Suppliers
        with self.client.get("/api/v1/suppliers", headers=self.headers, name="[META] Get Suppliers", catch_response=True) as resp:
            if resp.status_code == 200:
                self.metadata["suppliers"] = resp.json().get("data", [])

        # Master Items (for Purchases/Transfer)
        with self.client.get("/api/v1/items", headers=self.headers, name="[META] Get Master Items", catch_response=True) as resp:
            if resp.status_code == 200:
                self.metadata["master_items"] = resp.json().get("data", [])

        # Branch Items for EACH branch found (to ensure Sale consistency)
        self._refresh_inventory()

    def _refresh_inventory(self):
        """Refresh local stock info to avoid 'insufficient stock' errors."""
        for branch in self.metadata["branches"]:
            b_id = branch["id"]
            with self.client.get(f"/api/v1/inventory/branch/{b_id}", headers=self.headers, name="[META] Refresh Branch Items", catch_response=True) as resp:
                if resp.status_code == 200:
                    self.metadata["branch_items"][b_id] = resp.json().get("data", [])

    def _log_failure(self, name, resp, payload=None):
        """Helper to print detailed error info to terminal."""
        if resp.status_code >= 400:
            logger.error(f"\n{'='*40}\nFAILED: {name} | Status: {resp.status_code}\nResponse: {resp.text}\nPayload: {json.dumps(payload) if payload else 'N/A'}\n{'='*40}")

    # ─── Core TPC-C Workload Tasks ──────────────────────────────────────────────

    @task(50) # 50% Sale
    def sale_transaction(self):
        if not self.metadata["branches"]:
            return

        # 1. Select a branch that HAS items with STOCK
        available_branches = []
        for b_id, items in self.metadata["branch_items"].items():
            if any(i["stock"] > 0 for i in items):
                available_branches.append(b_id)
        
        if not available_branches:
            # Try to refresh inventory if we think everything is empty
            self._refresh_inventory()
            return

        branch_id = random.choice(available_branches)
        # Filter only items with positive stock
        items_pool = [i for i in self.metadata["branch_items"][branch_id] if i["stock"] > 0]
        
        if not items_pool:
            return

        # 2. Select items
        num_items = random.randint(1, min(3, len(items_pool))) 
        selected_items = random.sample(items_pool, k=num_items)

        payload = {
            "branch_id": branch_id,
            "customer_id": None,
            "items": [{"branch_item_id": i["id"], "qty": random.randint(1, min(5, i["stock"]))} for i in selected_items],
            "tax": 0,
            "discount": 0,
            "note": "Locust TPC-C Sale"
        }
        
        with self.client.post("/api/v1/transactions/sale", json=payload, headers=self.headers, name="/transactions/sale", catch_response=True) as resp:
            if resp.status_code != 201:
                self._log_failure("SALE", resp, payload)
            else:
                # Optimistic local stock update to reduce server-side 400s
                for sold in payload["items"]:
                    for item in self.metadata["branch_items"][branch_id]:
                        if item["id"] == sold["branch_item_id"]:
                            item["stock"] -= sold["qty"]
                            break

    @task(30) # 30% Purchase (Stock-In) - Increased from 10% to keep stock up
    def purchase_transaction(self):
        if self.user_data["role"] not in ["owner", "manager"]:
            return
            
        if not self.metadata["master_items"] or not self.metadata["suppliers"]:
            return

        # Prefer branch purchase for TPC-C visibility
        if self.metadata["branches"]:
            location_key = "branch_id"
            location_id = random.choice(self.metadata["branches"])["id"]
        elif self.metadata["warehouses"]:
            location_key = "warehouse_id"
            location_id = random.choice(self.metadata["warehouses"])["id"]
        else:
            return

        supplier = random.choice(self.metadata["suppliers"])
        num_items = random.randint(1, 3)
        items_pool = self.metadata["master_items"]
        selected_items = random.sample(items_pool, k=min(num_items, len(items_pool)))

        payload = {
            location_key: location_id,
            "supplier_id": supplier["id"],
            "tax": 0,
            "discount": 0,
            "note": "Locust TPC-C Purchase",
            "items": [
                {
                    "item_id": i["id"], 
                    "qty": random.randint(20, 100), 
                    "cost": int(float(i.get("cost") or 10000))
                } 
                for i in selected_items
            ]
        }
        
        with self.client.post("/api/v1/transactions/purchase", json=payload, headers=self.headers, name="/transactions/purchase", catch_response=True) as resp:
            if resp.status_code != 201:
                self._log_failure("PURCHASE", resp, payload)
            else:
                # Refresh inventory after purchase to see new stock
                self._refresh_inventory()

    @task(10) # 10% Transfer
    def transfer_stock(self):
        if self.user_data["role"] not in ["owner", "manager"]:
            return

        if len(self.metadata["branches"]) < 2 or not self.metadata["master_items"]:
            return

        src = random.choice(self.metadata["branches"])
        dest = random.choice([b for b in self.metadata["branches"] if b["id"] != src["id"]])
        
        num_items = random.randint(1, 2)
        selected_items = random.sample(self.metadata["master_items"], k=min(num_items, len(self.metadata["master_items"])))

        payload = {
            "source_type": "branch",
            "source_id": src["id"],
            "dest_type": "branch",
            "dest_id": dest["id"],
            "note": "Locust TPC-C Transfer",
            "items": [{"item_id": i["id"], "qty": random.randint(1, 10)} for i in selected_items]
        }
        
        with self.client.post("/api/v1/transactions/transfer", json=payload, headers=self.headers, name="/transactions/transfer", catch_response=True) as resp:
            if resp.status_code != 201:
                self._log_failure("TRANSFER", resp, payload)

    @task(10) # 10% Read-only
    def stock_check(self):
        if not self.metadata["branches"]:
            return
        b_id = random.choice(self.metadata["branches"])["id"]
        self.client.get(f"/api/v1/inventory/branch/{b_id}", headers=self.headers, name="/inventory/branch")

@events.test_start.add_listener
def on_test_start(environment, **kwargs):
    if not tokens_pool:
        print("\n" + "!"*60)
        print(" ERROR: workload/tokens.json EMPTY. Run login_generator first! ")
        print("!"*60 + "\n")
        environment.runner.quit()
