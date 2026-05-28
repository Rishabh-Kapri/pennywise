GO ?= go
GOCACHE ?= /tmp/pennywise-go-build-cache

.DEFAULT_GOAL := help

.PHONY: help build test fmt vet check tidy vendor

help:
	@echo "Shared module targets:"
	@echo "  make build   Compile packages"
	@echo "  make test    Run tests"
	@echo "  make check   Run fmt, vet, and tests"

build:
	$(GO) build ./...

test:
	GOCACHE=$(GOCACHE) $(GO) test ./...

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

check: fmt vet test

tidy:
	$(GO) mod tidy
