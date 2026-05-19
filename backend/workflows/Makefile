GO ?= go
GOCACHE ?= /tmp/pennywise-go-build-cache
BIN_DIR ?= ./bin

.DEFAULT_GOAL := help

.PHONY: help run run-worker build build-worker test fmt vet check tidy vendor clean

help:
	@echo "Workflow targets:"
	@echo "  make run     Run Temporal worker"
	@echo "  make build   Build worker binary into ./bin"
	@echo "  make test    Run tests"
	@echo "  make check   Run fmt, vet, and tests"

run: run-worker

run-worker:
	$(GO) run ./cmd/worker

build: build-worker

build-worker:
	mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_DIR)/worker ./cmd/worker

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
