# dotstate Makefile
# Run `make help` for available targets

.DEFAULT_GOAL := help
.PHONY: help all build build-local run test test-v test-cover lint fmt vet \
        check secrets deps clean install-tools doctor

# Build info (injected at compile time)
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -s -w -X 'github.com/dnery/dotstate/dot/internal/cli.version=$(VERSION)' \
           -X 'github.com/dnery/dotstate/dot/internal/cli.commit=$(COMMIT)' \
           -X 'github.com/dnery/dotstate/dot/internal/cli.date=$(DATE)'

# Directories
BIN_DIR     := bin
COVER_DIR   := coverage
CMD_DIR     := ./cmd/dot

# Go settings
GO          := go
GOFLAGS     := -trimpath
CGO_ENABLED := 0

# Colors for output
GREEN  := \033[0;32m
YELLOW := \033[0;33m
CYAN   := \033[0;36m
RESET  := \033[0m

##@ General

help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\n$(CYAN)Usage:$(RESET)\n  make $(GREEN)<target>$(RESET)\n"} \
		/^[a-zA-Z_0-9-]+:.*?##/ { printf "  $(GREEN)%-15s$(RESET) %s\n", $$1, $$2 } \
		/^##@/ { printf "\n$(YELLOW)%s$(RESET)\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

all: lint test build-local ## Run lint, test, and build

run: ## Run the CLI (dev mode)
	$(GO) run $(CMD_DIR) $(ARGS)

run-verbose: ## Run the CLI with verbose output
	$(GO) run $(CMD_DIR) --verbose $(ARGS)

##@ Building

build-local: ## Build for current platform
	CGO_ENABLED=$(CGO_ENABLED) $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/dot $(CMD_DIR)

build: ## Build for all platforms (linux, darwin, windows)
	@mkdir -p $(BIN_DIR)/{linux,darwin,windows}
	GOOS=linux   GOARCH=amd64 CGO_ENABLED=0 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/linux/dot $(CMD_DIR)
	GOOS=darwin  GOARCH=arm64 CGO_ENABLED=0 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/darwin/dot $(CMD_DIR)
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/windows/dot.exe $(CMD_DIR)
	@echo "$(GREEN)Built binaries in $(BIN_DIR)/$(RESET)"

##@ Testing

test: ## Run tests
	$(GO) test -race -shuffle=on ./...

test-v: ## Run tests with verbose output
	$(GO) test -race -shuffle=on -v ./...

test-cover: ## Run tests with coverage report
	@mkdir -p $(COVER_DIR)
	$(GO) test -race -shuffle=on -coverprofile=$(COVER_DIR)/coverage.out -covermode=atomic ./...
	$(GO) tool cover -html=$(COVER_DIR)/coverage.out -o $(COVER_DIR)/coverage.html
	$(GO) tool cover -func=$(COVER_DIR)/coverage.out | tail -1
	@echo "$(GREEN)Coverage report: $(COVER_DIR)/coverage.html$(RESET)"

test-short: ## Run short tests only (skip integration tests)
	$(GO) test -race -shuffle=on -short ./...

##@ Code Quality

lint: ## Run golangci-lint
	golangci-lint run --fix

lint-check: ## Run golangci-lint without fixes (CI mode)
	golangci-lint run

fmt: ## Format code
	$(GO) fmt ./...
	goimports -w -local github.com/dnery/dotstate .

vet: ## Run go vet
	$(GO) vet ./...

check: fmt vet lint test ## Run all checks (format, vet, lint, test)

##@ Security

secrets: ## Scan for secrets with gitleaks
	gitleaks detect --source . --config .gitleaks.toml --verbose

secrets-check: ## Scan for secrets (CI mode, fail on findings)
	gitleaks detect --source . --config .gitleaks.toml --exit-code 1

##@ Dependencies

deps: ## Tidy and verify dependencies
	$(GO) mod tidy
	$(GO) mod verify

deps-update: ## Update all dependencies
	$(GO) get -u ./...
	$(GO) mod tidy

deps-graph: ## Show dependency graph (requires graphviz)
	$(GO) mod graph | sed -Ee 's/@[^[:space:]]+//g' | sort -u | grep -v "^github.com/dnery" | head -20

##@ Tools

install-tools: ## Install development tools
	@echo "$(CYAN)Installing development tools...$(RESET)"
	$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	$(GO) install github.com/gitleaks/gitleaks/v8@latest
	$(GO) install golang.org/x/tools/cmd/goimports@latest
	@echo "$(GREEN)Tools installed$(RESET)"

doctor: build-local ## Check prerequisites and run dot doctor
	./$(BIN_DIR)/dot doctor

##@ Cleanup

clean: ## Remove build artifacts
	rm -rf $(BIN_DIR) $(COVER_DIR)
	$(GO) clean -cache -testcache

clean-all: clean ## Remove all artifacts including module cache
	$(GO) clean -modcache
