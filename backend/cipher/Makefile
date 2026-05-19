GO ?= go
GOCACHE ?= /tmp/pennywise-go-build-cache
BIN_DIR ?= ./bin
BACKFILL_ARGS ?=

.DEFAULT_GOAL := help

.PHONY: help run run-api run-agent run-backfill run-embed-entities build build-api build-agent build-backfill build-embed-entities test fmt vet check tidy vendor clean

help:
	@echo "Cipher targets:"
	@echo "  make run                   Run Cipher API server"
	@echo "  make run-agent             Run local agent runtime"
	@echo "  make run-backfill          Run backfill command; pass BACKFILL_ARGS=\"-data path\""
	@echo "  make run-embed-entities    Embed all budget entities into pgvector; requires BUDGET_ID env var"
	@echo "  make build                 Build API, agent, backfill, and embed-entities binaries into ./bin"
	@echo "  make test                  Run tests"
	@echo "  make check                 Run fmt, vet, and tests"

run: run-api

run-api:
	$(GO) run ./cmd/api

run-agent:
	$(GO) run ./agent/runtime

run-backfill:
	$(GO) run ./cmd/backfill $(BACKFILL_ARGS)

run-embed-entities:
	$(GO) run ./cmd/embed-entities

build: build-api build-agent build-backfill build-embed-entities

build-api:
	mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_DIR)/cipher-api ./cmd/api

build-agent:
	mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_DIR)/cipher-agent ./agent/runtime

build-backfill:
	mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_DIR)/cipher-backfill ./cmd/backfill

build-embed-entities:
	mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_DIR)/cipher-embed-entities ./cmd/embed-entities

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
	$(GO) mod vendor

clean:
	rm -rf $(BIN_DIR)
