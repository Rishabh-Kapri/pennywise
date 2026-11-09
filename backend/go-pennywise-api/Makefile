GO ?= go
GOCACHE ?= /tmp/pennywise-go-build-cache
BIN_DIR ?= ./bin

.DEFAULT_GOAL := help

.PHONY: help run build test fmt vet check tidy vendor clean migrate-up migrate-one migrate-down migrate-redo migrate-status migrate-version migrate-create migrate-baseline

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
	@echo "  make migrate-create name=add_x  Create the next migration in backend/shared/migrations/sql"
	@echo "  make migrate-baseline One-off: adopt the squashed history on a DB built by the old migrations"
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
	$(GO) run ./cmd/migrations up

migrate-one:
	$(GO) run ./cmd/migrations up-by-one

migrate-down:
	$(GO) run ./cmd/migrations down

migrate-redo:
	$(GO) run ./cmd/migrations redo

migrate-status:
	$(GO) run ./cmd/migrations status

migrate-version:
	$(GO) run ./cmd/migrations version

migrate-create:
	@test -n "$(name)" || { echo "usage: make migrate-create name=add_something"; exit 1; }
	$(GO) run ./cmd/migrations create $(name)

migrate-baseline:
	$(GO) run ./cmd/migrations baseline
