GO ?= go
GOCACHE ?= /tmp/pennywise-go-build-cache
BIN_DIR ?= ./bin
BACKFILL_ARGS ?=

.DEFAULT_GOAL := help

.PHONY: help run run-api run-backfill run-backfill-sources run-eval-lookup build build-api build-backfill test fmt vet check tidy vendor clean

help:
	@echo "Cipher targets:"
	@echo "  make run                   Run Cipher API server"
	@echo "  make run-backfill          Run backfill command; pass BACKFILL_ARGS=\"-data path\""
	@echo "  make run-backfill-sources  Split transactions.raw_bank_text into transaction_sources + counterparty_aliases (one DB transaction); BACKFILL_ARGS=\"-dry-run -budget <id>\""
	@echo "  make run-eval-lookup       Compare trigram and embedding matching on your aliases (read-only); pass EVAL_ARGS=\"-ollama http://host:11434\""
	@echo "  make build                 Build API and backfill binaries into ./bin"
	@echo "  make test                  Run tests"
	@echo "  make check                 Run fmt, vet, and tests"
	@echo "  make vendor                Run vendor to add the shared modules"

run: run-api

run-api:
	$(GO) run ./cmd/api

run-backfill:
	$(GO) run ./cmd/backfill $(BACKFILL_ARGS)

run-backfill-sources:
	$(GO) run ./cmd/backfill-sources $(BACKFILL_ARGS)

run-eval-lookup:
	$(GO) run ./cmd/eval-lookup $(EVAL_ARGS)

build: build-api build-backfill

build-api:
	mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_DIR)/cipher-api ./cmd/api

build-backfill:
	mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_DIR)/cipher-backfill ./cmd/backfill

test:
	GOCACHE=$(GOCACHE) $(GO) test ./...

vendor:
	GOWORK=off $(GO) mod vendor

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

check: fmt vet test

tidy:
	$(GO) mod tidy

clean:
	rm -rf $(BIN_DIR)
