GO ?= go
GOCACHE ?= /tmp/pennywise-go-build-cache
BIN_DIR ?= ./bin
MIGRATIONS_DIR ?= .

.DEFAULT_GOAL := help

.PHONY: help run build test fmt vet check tidy vendor clean migrate-up migrate-one migrate-down migrate-redo migrate-status migrate-version migrate-baseline

help:
	@echo "Go API targets:"
	@echo "  make run               Run API server"
	@echo "  make build            Build API binary into ./bin"
	@echo "  make test             Run tests"
	@echo "  make check            Run fmt, vet, and tests"
	@echo "  make migrate-up       Apply all pending migrations"
	@echo "  make migrate-one      Apply one pending migration"
	@echo "  make migrate-down     Roll back one migration"
	@echo "  make migrate-status   Show migration status"
	@echo "  make migrate-version  Show current migration version"
	@echo "  make migrate-baseline Mark seed migrations as applied"
	@echo "  make vendor           Run vendor to add the shared modules"

run:
	$(GO) run ./cmd/api

build:
	mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_DIR)/api ./cmd/api

test:
	GOCACHE=$(GOCACHE) $(GO) test ./...

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

check: fmt vet test

tidy:
	$(GO) mod tidy

vendor:
	GOWORK=off $(GO) mod vendor

clean:
	rm -rf $(BIN_DIR)

migrate-up:
	$(GO) run ./cmd/migrations -dir $(MIGRATIONS_DIR) up

migrate-one:
	$(GO) run ./cmd/migrations -dir $(MIGRATIONS_DIR) up-by-one

migrate-down:
	$(GO) run ./cmd/migrations -dir $(MIGRATIONS_DIR) down

migrate-redo:
	$(GO) run ./cmd/migrations -dir $(MIGRATIONS_DIR) redo

migrate-status:
	$(GO) run ./cmd/migrations -dir $(MIGRATIONS_DIR) status

migrate-version:
	$(GO) run ./cmd/migrations -dir $(MIGRATIONS_DIR) version

migrate-baseline:
	$(GO) run ./cmd/migrations -dir $(MIGRATIONS_DIR) baseline
