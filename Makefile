# dotstate Makefile
# Run `make help` for available targets

.DEFAULT_GOAL := help
.PHONY: help
.PHONY: all
.PHONY: build
.PHONY: build-all
.PHONY: install
.PHONY: install-dot
.PHONY: install-senv
.PHONY: run
.PHONY: test
.PHONY: test-v
.PHONY: test-cover
.PHONY: test-e2e
.PHONY: test-e2e-fast
.PHONY: test-e2e-deep
.PHONY: test-e2e-capture
.PHONY: test-e2e-verify
.PHONY: test-e2e-record
.PHONY: docs-check
.PHONY: lint
.PHONY: fmt
.PHONY: vet
.PHONY: check
.PHONY: secrets
.PHONY: deps
.PHONY: clean
.PHONY: install-tools
.PHONY: doctor

# Build info (injected at compile time)
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT	?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE	?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -s -w -X 'github.com/dnery/dotstate/dot/internal/cli.version=$(VERSION)' \
			X 'github.com/dnery/dotstate/dot/internal/cli.commit=$(COMMIT)' \
			X 'github.com/dnery/dotstate/dot/internal/cli.date=$(DATE)'

# Directories and targets
BIN_DIR			:= bin
DOT_CMD			:= dot
SENV_CMD		:= senv
DOT_CMD_DIR		:= ./cmd/$(DOT_CMD)
SENV_CMD_DIR 	:= ./cmd/$(SENV_CMD)
INSTALL_DIR		?= $(HOME)/.local/bin
COVER_DIR		:= coverage

# Go settings
GO			:= go
GOFLAGS		:= -trimpath
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

all: lint test build

run: ## Run the CLI (dev mode)
	$(GO) run $(DOT_CMD_DIR) $(ARGS)

run-verbose: ## Run the CLI with verbose output
	$(GO) run $(DOT_CMD_DIR) --verbose $(ARGS)

##@ Building

install: install-dot install-senv ## Install all binaries to INSTALL_DIR (default: ~/.local/bin)

install-dot: ## Install dot to INSTALL_DIR (default: ~/.local/bin)
	@mkdir -p "$(INSTALL_DIR)"
	CGO_ENABLED=$(CGO_ENABLED) $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o "$(INSTALL_DIR)/$(DOT_CMD)" $(DOT_CMD_DIR)
	@echo "$(GREEN)Installed dot to $(INSTALL_DIR)/$(DOT_CMD)$(RESET)"

install-senv: ## Install senv to INSTALL_DIR (default: ~/.local/bin)
	@mkdir -p "$(INSTALL_DIR)"
	CGO_ENABLED=$(CGO_ENABLED) $(GO) build $(GOFLAGS) -o "$(INSTALL_DIR)/$(SENV_CMD)" $(SENV_CMD_DIR)
	@echo "$(GREEN)Installed senv to $(INSTALL_DIR)/$(SENV_CMD)$(RESET)"

build: ## Build for current platform
	CGO_ENABLED=$(CGO_ENABLED) $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(DOT_CMD) $(DOT_CMD_DIR)
	CGO_ENABLED=$(CGO_ENABLED) $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(SENV_CMD) $(SENV_CMD_DIR)
	@echo "$(GREEN)Built binaries in $(BIN_DIR)/$(RESET)"

build-all: ## Build for all platforms (linux, darwin, windows)
	@mkdir -p $(BIN_DIR)/{linux,darwin,windows}
	GOOS=linux   GOARCH=amd64 CGO_ENABLED=0 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/linux/$(DOT_CMD) $(DOT_CMD_DIR)
	GOOS=darwin  GOARCH=arm64 CGO_ENABLED=0 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/darwin/$(DOT_CMD) $(DOT_CMD_DIR)
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/windows/$(DOT_CMD).exe $(DOT_CMD_DIR)
	GOOS=linux   GOARCH=amd64 CGO_ENABLED=0 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/linux/$(SENV_CMD) $(SENV_CMD_DIR)
	GOOS=darwin  GOARCH=arm64 CGO_ENABLED=0 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/darwin/$(SENV_CMD) $(SENV_CMD_DIR)
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/windows/$(SENV_CMD).exe $(SENV_CMD_DIR)
	@echo "$(GREEN)Built cross-platform binaries in $(BIN_DIR)/$(RESET)"


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

test-e2e: build-local ## Run discover harness for all scenarios
	./test/e2e/discover_harness.sh --dot-bin ./bin/$(DOT_CMD) --scenario all

test-e2e-fast: build-local ## Run discover harness fast scenario
	./test/e2e/discover_harness.sh --dot-bin ./bin/$(DOT_CMD) --scenario discover-fast

test-e2e-deep: build-local ## Run discover harness deep scenario
	./test/e2e/discover_harness.sh --dot-bin ./bin/$(DOT_CMD) --scenario discover-deep

test-e2e-capture: build-local ## Run discover harness capture-loop scenario
	./test/e2e/discover_harness.sh --dot-bin ./bin/$(DOT_CMD) --scenario capture-loop

test-e2e-verify: build-local ## Run macOS verification harness scenario
	./test/e2e/discover_harness.sh --dot-bin ./bin/$(DOT_CMD) --scenario macos-verification

test-e2e-record: build-local ## Run discover harness with asciinema recording (opt-in upload)
	./test/e2e/discover_harness.sh --dot-bin ./bin/$(DOT_CMD) --scenario all --record

docs-check: ## Validate documentation structure, pointers, and local links
	./test/docs/docs_check.sh

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
	$(GO) install github.com/zricethezav/gitleaks/v8@latest
	$(GO) install golang.org/x/tools/cmd/goimports@latest
	@echo "$(GREEN)Tools installed$(RESET)"

doctor: build-local ## Check prerequisites and run dot doctor
	./$(BIN_DIR)/$(DOT_CMD) doctor

##@ Cleanup

clean: ## Remove build artifacts
	rm -rf $(BIN_DIR) $(COVER_DIR)
	$(GO) clean -cache -testcache

clean-all: clean ## Remove all artifacts including module cache
	$(GO) clean -modcache
