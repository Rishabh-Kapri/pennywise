GO ?= go
GOCACHE ?= /tmp/pennywise-go-build-cache
BIN_DIR ?= ./bin

.DEFAULT_GOAL := help

.PHONY: help run build test fmt vet check tidy vendor clean

help:
	@echo "Go Gmail targets:"
	@echo "  make run     Run Gmail watcher/API"
	@echo "  make build   Build binary into ./bin"
	@echo "  make test    Run tests"
	@echo "  make check   Run fmt, vet, and tests"
	@echo "  make vendor  Run vendor to add the shared modules"

run:
	$(GO) run ./cmd

build:
	mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_DIR)/go-gmail ./cmd

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
