.PHONY: run run-single run-multi build tidy

## run-single: start the server in single-DB mode
run-single:
	DB_MODE=single go run ./cmd/server/...

## run-multi: start the server in multi-DB mode
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
