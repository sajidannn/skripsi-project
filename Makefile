.PHONY: api-run-single api-run-multi api-run api-build api-tidy \
        api-docker-build \
        api-single-up api-single-down api-single-logs \
        api-multi-up api-multi-down api-multi-logs \
        api-clean \
        db-single-up db-single-down db-single-clean db-single-logs db-single-logs-seeder \
        db-multi-up db-multi-down db-multi-clean db-multi-logs db-multi-logs-seeder \
        db-single-reseed db-multi-reseed \
        db-clean \
        exporters-api-up exporters-api-down \
        exporters-db-single-up exporters-db-multi-up exporters-db-down \
        monitoring-up monitoring-down monitoring-reload \
        workload-small workload-medium workload-large \
        workload-small-ui workload-medium-ui workload-large-ui

# Data scale for seeding: small | medium | large  (default: small)
# Usage: make db-single-up SCALE=medium
SCALE ?= small

# ==============================================================================
# API COMMANDS (Local Execution)
# ==============================================================================
api-single-run	:
	cd api && DB_MODE=single go run ./cmd/server/...

api-multi-run:
	cd api && DB_MODE=multi go run ./cmd/server/...

api-run:
	cd api && go run ./cmd/server/...

api-build:
	cd api && go build -o bin/server ./cmd/server/...

api-tidy:
	cd api && go mod tidy

api-docker-build:
	cd api && docker build -t pos-api .


# ==============================================================================
# API COMMANDS (Docker Compose) - VM 1
# ==============================================================================
api-single-up	:
	cd api && docker compose -f docker-compose.single.yml up --build -d

api-single-down:
	cd api && docker compose -f docker-compose.single.yml down

api-single-logs:
	cd api && docker compose -f docker-compose.single.yml logs -f

api-multi-up:
	cd api && docker compose -f docker-compose.multi.yml up --build -d

api-multi-down:
	cd api && docker compose -f docker-compose.multi.yml down

api-multi-logs:
	cd api && docker compose -f docker-compose.multi.yml logs -f

api-clean:
	cd api && docker compose -f docker-compose.single.yml down -v
	cd api && docker compose -f docker-compose.multi.yml down -v


# ==============================================================================
# DB COMMANDS (Docker Compose) - VM 2
# Override data scale with: make db-single-up SCALE=medium
# ==============================================================================

# ── Single-DB ─────────────────────────────────────────────────────────────────
db-single-up:
	cd DB && SCALE=$(SCALE) docker compose -f docker-compose.single.yml up --build -d

db-single-down:
	cd DB && docker compose -f docker-compose.single.yml down

db-single-clean:
	cd DB && docker compose -f docker-compose.single.yml down -v

db-single-logs:
	cd DB && docker compose -f docker-compose.single.yml logs -f

db-single-logs-seeder:
	cd DB && docker compose -f docker-compose.single.yml logs -f seeder

# Clean data + re-seed with chosen scale (forces fresh Postgres volume)
db-single-reseed:
	cd DB && docker compose -f docker-compose.single.yml down -v
	cd DB && SCALE=$(SCALE) docker compose -f docker-compose.single.yml up --build -d

# ── Multi-DB ──────────────────────────────────────────────────────────────────
db-multi-up:
	cd DB && SCALE=$(SCALE) docker compose -f docker-compose.multi.yml up --build -d

db-multi-down:
	cd DB && docker compose -f docker-compose.multi.yml down

db-multi-clean:
	cd DB && docker compose -f docker-compose.multi.yml down -v

db-multi-logs:
	cd DB && docker compose -f docker-compose.multi.yml logs -f

db-multi-logs-seeder:
	cd DB && docker compose -f docker-compose.multi.yml logs -f seeder

# Clean data + re-seed with chosen scale (forces fresh Postgres volume)
db-multi-reseed:
	cd DB && docker compose -f docker-compose.multi.yml down -v
	cd DB && SCALE=$(SCALE) docker compose -f docker-compose.multi.yml up --build -d

# ── Both ──────────────────────────────────────────────────────────────────────
db-clean:
	cd DB && docker compose -f docker-compose.single.yml down -v
	cd DB && docker compose -f docker-compose.multi.yml down -v


# ==============================================================================
# REMOTES MONITORING SETUP
# ==============================================================================

# VM 1: Start API Exporters (Node Exporter + cAdvisor)
exporters-api-up:
	cd monitoring && docker compose -f docker-compose.api-exporters.yml up -d

exporters-api-down:
	cd monitoring && docker compose -f docker-compose.api-exporters.yml down

# VM 2: Start DB Exporters (Node Exporter + Postgres Exporter)
exporters-db-single-up:
	cd monitoring && DATA_SOURCE_NAME="postgresql://postgres:supersecret@localhost:5432/pos_single?sslmode=disable" \
		docker compose -f docker-compose.db-exporters.yml up -d
	@echo "DB exporters started → scrape target: pos_single (port 5432)"

exporters-db-multi-up:
	cd monitoring && DATA_SOURCE_NAME="postgresql://postgres:supersecret@localhost:5433/pos_master?sslmode=disable" \
		docker compose -f docker-compose.db-exporters.yml up -d
	@echo "DB exporters started → scrape target: pos_master (port 5433)"

exporters-db-down:
	cd monitoring && docker compose -f docker-compose.db-exporters.yml down

# LAPTOP: Start Central Monitoring (Prometheus + Grafana)
monitoring-up:
	cd monitoring && docker compose up -d
	@echo ""
	@echo "=== Monitoring Hub started (LOCAL) ==="
	@echo "  Grafana:    http://localhost:3000  (admin/admin)"
	@echo "  Prometheus: http://localhost:9090"
	@echo ""

monitoring-down:
	cd monitoring && docker compose down -v

monitoring-reload:
	curl -X POST http://localhost:9090/-/reload

# ==============================================================================
# WORKLOAD GENERATOR (Locust)
# ==============================================================================

WORKLOAD_API_URL ?= http://192.168.10.183:8080

# S1/S2 - Baseline (5 tenant, 50 user)
workload-small:
	@API_URL=$(WORKLOAD_API_URL) TAG=small SCALE=5 USERS=50 RUN_TIME=5m ./workload/run_test.sh

# S3/S5 - Skalabilitas 10 tenant (100 user)
workload-medium:
	@API_URL=$(WORKLOAD_API_URL) TAG=medium SCALE=10 USERS=100 RUN_TIME=5m ./workload/run_test.sh

# S4/S6 - Skalabilitas 50 tenant (200 user)
workload-large:
	@API_URL=$(WORKLOAD_API_URL) TAG=large SCALE=50 USERS=200 RUN_TIME=5m ./workload/run_test.sh

# Mode UI untuk monitoring dashboard Locust
workload-small-ui:
	@API_URL=$(WORKLOAD_API_URL) TAG=small SCALE=5 USERS=50 RUN_TIME=5m HEADLESS=false ./workload/run_test.sh

workload-medium-ui:
	@API_URL=$(WORKLOAD_API_URL) TAG=medium SCALE=10 USERS=100 RUN_TIME=5m HEADLESS=false ./workload/run_test.sh

workload-large-ui:
	@API_URL=$(WORKLOAD_API_URL) TAG=large SCALE=50 USERS=200 RUN_TIME=5m HEADLESS=false ./workload/run_test.sh
