ifneq (,$(wildcard .env))
	include .env
	export
endif

.PHONY: run test lint fmt migrate-up migrate-down build help

BINARY       := bin/server
MIGRATE_BIN  := bin/migrate

.DEFAULT_GOAL := help

help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

run: ## Run the HTTP/WS server
	go run ./cmd/server

build: ## Compile server and migrate binaries into bin/
	go build -o $(BINARY) ./cmd/server
	go build -o $(MIGRATE_BIN) ./cmd/migrate

test: ## Run tests with race detector and coverage
	go test ./... -race -cover -count=1

lint: ## Run go vet and staticcheck
	go vet ./...
	staticcheck ./...

fmt: ## Format all Go source files
	gofmt -s -w .

migrate-up: ## Apply all pending migrations
	go run ./cmd/migrate up

migrate-down: ## Roll back the last applied migration
	go run ./cmd/migrate down
