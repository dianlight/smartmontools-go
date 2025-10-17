SHELL := /bin/bash
GO := $(or $(shell command -v go 2>/dev/null), go)
GOCMD := $(GO)
GOTEST := $(GOCMD) test
GOBUILD := $(GOCMD) build
GOMOD := $(GOCMD) mod

PKGS := $(shell $(GOCMD) list ./...)
BIN := smartmontools-go

# Where go install will place binaries. Prefer GOBIN, fall back to GOPATH/bin
GOBIN := $(shell $(GOCMD) env GOBIN)
ifeq ($(GOBIN),)
GOBIN := $(shell $(GOCMD) env GOPATH)/bin
endif

# staticcheck binary path (either on PATH or in GOBIN)
STATICCHECK := $(or $(shell command -v staticcheck 2>/dev/null),$(GOBIN)/staticcheck)

.PHONY: help build test coverage fmt vet lint tidy mod-download run-example clean

help:
	@echo "Makefile for smartmontools-go"
	@echo ""
	@echo "Available targets:"
	@echo "  build          Build the project binary"
	@echo "  test           Run unit tests for all packages"
	@echo "  coverage       Run tests and show coverage summary"
	@echo "  fmt            Run gofmt on the project"
	@echo "  vet            Run go vet on the project"
	@echo "  lint           Run staticcheck if available"
	@echo "  tidy           Run go mod tidy"
	@echo "  mod-download   Download modules (go mod download)"
	@echo "  run-example    Run the example in examples/basic"
	@echo "  clean          Remove build artifacts"

build:
	@echo "Building $(BIN)..."
	$(GOBUILD) -v -o $(BIN) ./

test:
	@echo "Running tests..."
	$(GOTEST) ./...

coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -coverprofile=coverage.out ./... || true
	@echo "Summary:"
	@if [ -f coverage.out ]; then $(GOCMD) tool cover -func=coverage.out | tail -n 1; fi

.PHONY: coverage-upload
coverage-upload: coverage
	@echo "Uploading coverage to Codecov (if CODECOV_TOKEN or public repo allowing uploads)..."
	@if [ -f coverage.out ]; then \
		if command -v bash >/dev/null 2>&1; then \
			bash -c "$(shell printf "$(shell printf '')")" >/dev/null 2>&1 || true; \
			# Try codecov uploader via bash if available; CI may provide CODECOV_TOKEN via secrets
			bash <(curl -s https://codecov.io/bash) -f coverage.out || echo "codecov upload failed"; \
		else \
			echo "bash not available; cannot upload coverage"; \
		fi \
	else \
		echo "coverage.out not found; run 'make coverage' first"; exit 1; \
	fi

fmt:
	@echo "Formatting code (gofmt)..."
	@gofmt -s -w .

.PHONY: fmt-check
fmt-check:
	@echo "Checking code format (gofmt)..."
	@! test -n "$(shell gofmt -l .)" || (echo "gofmt needs to be applied" && exit 1)

vet:
	@echo "Running go vet..."
	@$(GOCMD) vet ./...

lint:
	@echo "Running staticcheck if available..."
	@if command -v staticcheck >/dev/null 2>&1; then staticcheck ./...; else echo "staticcheck not installed; skip (install: go install honnef.co/go/tools/cmd/staticcheck@latest)"; fi

.PHONY: ci-lint
ci-lint:
	@echo "Running CI linting: fmt-check, vet, staticcheck"
	@$(MAKE) fmt-check
	@$(MAKE) vet
	@if ! command -v staticcheck >/dev/null 2>&1; then \
		echo "staticcheck not found — installing to $(GOBIN)..."; \
		$(GOCMD) install honnef.co/go/tools/cmd/staticcheck@latest; \
		if [ ! -x "$(STATICCHECK)" ]; then echo "installation failed or $(STATICCHECK) not found"; exit 1; fi; \
	fi
	@echo "Running staticcheck..."
	@"$(STATICCHECK)" ./...

.PHONY: ci
ci: tidy mod-download ci-lint test
	@echo "CI: all checks passed"

tidy:
	@echo "Running go mod tidy..."
	@$(GOMOD) tidy

mod-download:
	@echo "Downloading modules..."
	@$(GOMOD) download

run-example:
	@echo "Running examples/basic..."
	@cd examples/basic && $(GOCMD) run ./

clean:
	@echo "Cleaning..."
	@rm -f $(BIN) coverage.out
