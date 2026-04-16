.PHONY: api-run-single api-run-multi api-run api-build api-tidy \
        api-docker-build \
        api-single-up api-single-down api-single-logs \
        api-multi-up api-multi-down api-multi-logs \
        api-clean \
        db-single-up db-single-down db-single-clean db-single-logs db-single-logs-seeder \
        db-multi-up db-multi-down db-multi-clean db-multi-logs db-multi-logs-seeder \
        db-single-reseed db-multi-reseed \
        db-clean

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
