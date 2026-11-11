GO ?= go
BINARY := upvert

.DEFAULT_GOAL := help

.PHONY: help fmt build test clean dist

help: ## Show this help message
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z0-9_-]+:.*?##/ {printf "  %-10s %s", $$1, $$2; print ""}' $(MAKEFILE_LIST)

fmt: ## Format Go code and run go vet
	$(GO) fmt ./...
	$(GO) vet ./...

build: fmt ## Build the upvert binary
	CGO_ENABLED=0 $(GO) build -o $(BINARY) ./cmd/upvert

test: fmt ## Run the test suite
	$(GO) test ./...

clean: ## Remove the built binary
	rm -f $(BINARY)

dist: clean ## Remove the local binary and GoReleaser output
	rm -rf dist
