.PHONY: api-run-single api-run-multi api-run api-build api-tidy \
        api-docker-build \
        api-single-up api-single-down api-single-logs \
        api-multi-up api-multi-down api-multi-logs \
        api-clean \
        db-single-up db-single-down db-single-clean db-single-logs db-single-logs-seeder \
        db-multi-up db-multi-down db-multi-clean db-multi-logs db-multi-logs-seeder \
        db-single-reseed db-multi-reseed \
        db-clean \
        db-exporters-up db-exporters-down db-exporters-single db-exporters-multi \
        monitoring-up monitoring-down monitoring-logs monitoring-reload

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
# DB EXPORTERS (Docker Compose) - VM 2
# Deploy node_exporter + postgres_exporter agar bisa di-scrape Prometheus VM 1
# Jalankan SEKALI dan biarkan berjalan selama semua eksperimen
# ==============================================================================

# Start exporters pointing ke single-DB postgres (port 5432)
db-exporters-single:
	cd DB && docker compose -f docker-compose.exporters.yml down 2>/dev/null || true
	cd DB && DATA_SOURCE_NAME="postgresql://postgres:supersecret@localhost:5432/pos_single?sslmode=disable" \
		docker compose -f docker-compose.exporters.yml up -d
	@echo "DB exporters started → scrape target: pos_single (port 5432)"

# Start exporters pointing ke multi-DB postgres (port 5433)
db-exporters-multi:
	cd DB && docker compose -f docker-compose.exporters.yml down 2>/dev/null || true
	cd DB && DATA_SOURCE_NAME="postgresql://postgres:supersecret@localhost:5433/pos_master?sslmode=disable" \
		docker compose -f docker-compose.exporters.yml up -d
	@echo "DB exporters started → scrape target: pos_master (port 5433)"

# Alias: sama dengan db-exporters-single (default)
db-exporters-up: db-exporters-single

db-exporters-down:
	cd DB && docker compose -f docker-compose.exporters.yml down


# ==============================================================================
# MONITORING STACK (Docker Compose) - VM 1
# Deploy Prometheus + Grafana + Node Exporter + cAdvisor
# Jalankan SEKALI sebelum mulai eksperimen, biarkan berjalan
# Grafana: http://localhost:3000 (admin/admin)
# Prometheus: http://localhost:9090
# ==============================================================================

monitoring-up:
	cd monitoring && docker compose up -d
	@echo ""
	@echo "=== Monitoring stack started ==="
	@echo "  Grafana:    http://localhost:3000  (admin/admin)"
	@echo "  Prometheus: http://localhost:9090"
	@echo ""

monitoring-down:
	cd monitoring && docker compose down

monitoring-logs:
	cd monitoring && docker compose logs -f

# Reload Prometheus config tanpa restart (berguna saat edit prometheus.yml)
monitoring-reload:
	curl -X POST http://localhost:9090/-/reload
