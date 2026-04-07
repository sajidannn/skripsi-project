.PHONY: api-run-single api-run-multi api-run api-build api-tidy \
        api-up-single api-down-single api-logs-single \
        api-up-multi api-down-multi api-logs-multi \
        api-clean \
        db-up-single db-down-single db-logs-single \
        db-up-multi db-down-multi db-logs-multi \
        db-clean

# ==============================================================================
# API COMMANDS (Local Execution)
# ==============================================================================
api-run-single:
	cd api && DB_MODE=single go run ./cmd/server/...

api-run-multi:
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
api-up-single:
	cd api && docker compose -f docker-compose.single.yml up --build -d

api-down-single:
	cd api && docker compose -f docker-compose.single.yml down

api-logs-single:
	cd api && docker compose -f docker-compose.single.yml logs -f

api-up-multi:
	cd api && docker compose -f docker-compose.multi.yml up --build -d

api-down-multi:
	cd api && docker compose -f docker-compose.multi.yml down

api-logs-multi:
	cd api && docker compose -f docker-compose.multi.yml logs -f

api-clean:
	cd api && docker compose -f docker-compose.single.yml down -v
	cd api && docker compose -f docker-compose.multi.yml down -v


# ==============================================================================
# DB COMMANDS (Docker Compose) - VM 2
# ==============================================================================
db-up-single:
	cd DB && docker compose -f docker-compose.single.yml up -d

db-down-single:
	cd DB && docker compose -f docker-compose.single.yml down

db-logs-single:
	cd DB && docker compose -f docker-compose.single.yml logs -f

db-up-multi:
	cd DB && docker compose -f docker-compose.multi.yml up -d

db-down-multi:
	cd DB && docker compose -f docker-compose.multi.yml down

db-logs-multi:
	cd DB && docker compose -f docker-compose.multi.yml logs -f

db-clean:
	cd DB && docker compose -f docker-compose.single.yml down -v
	cd DB && docker compose -f docker-compose.multi.yml down -v
