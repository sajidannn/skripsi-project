.PHONY: run run-single run-multi build tidy \
        docker-single-up docker-single-down \
        docker-multi-up docker-multi-down \
        docker-single-logs docker-multi-logs \
        docker-build

## run-single: start the server in single-DB mode (local)
run-single:
	DB_MODE=single go run ./cmd/server/...

## run-multi: start the server in multi-DB mode (local)
run-multi:
	DB_MODE=multi go run ./cmd/server/...

## run: default (reads from .env)
run:
	go run ./cmd/server/...

## build: compile to ./bin/server
build:
	go build -o bin/server ./cmd/server/...

## tidy: tidy and vendor dependencies
tidy:
	go mod tidy

## docker-build: build the API Docker image only
docker-build:
	docker build -t pos-api .

## docker-single-up: start API + single shared Postgres via Docker Compose
docker-single-up:
	docker compose -f docker-compose.single.yml up --build -d

## docker-single-down: tear down single-DB stack (keeps volume)
docker-single-down:
	docker compose -f docker-compose.single.yml down

## docker-single-logs: tail logs for the single-DB stack
docker-single-logs:
	docker compose -f docker-compose.single.yml logs -f

## docker-multi-up: start API + multi-DB Postgres via Docker Compose
docker-multi-up:
	docker compose -f docker-compose.multi.yml up --build -d

## docker-multi-down: tear down multi-DB stack (keeps volume)
docker-multi-down:
	docker compose -f docker-compose.multi.yml down

## docker-multi-logs: tail logs for the multi-DB stack
docker-multi-logs:
	docker compose -f docker-compose.multi.yml logs -f

## docker-clean: remove all containers AND volumes for both stacks
docker-clean:
	docker compose -f docker-compose.single.yml down -v
	docker compose -f docker-compose.multi.yml down -v
