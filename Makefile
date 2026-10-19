GO ?= go
PSX ?= psx
BIN_DIR ?= dist
CGO_ENABLED ?= 1
GOFILES := $(shell find cmd internal tui -type f -name '*.go' -print)

.PHONY: all build tui tui-demo test test-race test-cover vet fmt fmt-check check ci smoke psx-check psx-fix tidy clean help

all: ci

## Build
build: ## Build CLI, clean TUI, and demo TUI into $(BIN_DIR)/.
	mkdir -p $(BIN_DIR)
	CGO_ENABLED=$(CGO_ENABLED) $(GO) build -trimpath -ldflags='-s -w' -o $(BIN_DIR)/idios ./cmd/idios
	CGO_ENABLED=$(CGO_ENABLED) $(GO) build -trimpath -ldflags='-s -w' -o $(BIN_DIR)/idios-tui ./cmd/idios-tui
	CGO_ENABLED=$(CGO_ENABLED) $(GO) build -trimpath -ldflags='-s -w' -o $(BIN_DIR)/idios-tui-demo ./cmd/idios-tui-demo

## Interactive UI
tui: ## Run the full-screen, clean TUI against the standard Engine database.
	CGO_ENABLED=$(CGO_ENABLED) $(GO) run ./cmd/idios-tui

tui-demo: ## Run the demo TUI against its isolated, automatically seeded database.
	CGO_ENABLED=$(CGO_ENABLED) $(GO) run ./cmd/idios-tui-demo

## Quality
fmt: ## Format Go source files in-place.
	$(GO) fmt ./...

fmt-check: ## Fail if any Go source file is not gofmt-formatted.
	@unformatted="$$(gofmt -l $(GOFILES))"; \
	if [ -n "$$unformatted" ]; then echo "Unformatted Go files:"; echo "$$unformatted"; exit 1; fi

test: ## Run all tests.
	CGO_ENABLED=$(CGO_ENABLED) $(GO) test ./...

test-race: ## Run all tests under the race detector.
	CGO_ENABLED=$(CGO_ENABLED) $(GO) test -race ./...

test-cover: ## Run tests and write coverage.out.
	CGO_ENABLED=$(CGO_ENABLED) $(GO) test -coverprofile=coverage.out ./...

vet: ## Run go vet.
	CGO_ENABLED=$(CGO_ENABLED) $(GO) vet ./...

check: fmt-check test vet ## Run the fast local quality gate.

ci: check test-race build smoke ## Run the full local CI gate (PSX is a separate optional target).

smoke: build ## Smoke-test all executables, clean/demo database isolation, and a cross-Repository SQLite workflow.
	IDIOS_BIN=$(BIN_DIR)/idios IDIOS_TUI_BIN=$(BIN_DIR)/idios-tui IDIOS_TUI_DEMO_BIN=$(BIN_DIR)/idios-tui-demo sh ./scripts/smoke-platform.sh

psx-check: ## Check repository structure and documentation with PSX v3.
	@command -v $(PSX) >/dev/null 2>&1 || { echo "PSX not found. Install github.com/m-mdy-m/psx/cmd/psx or skip this optional check." >&2; exit 2; }
	$(PSX) check

psx-fix: ## Apply safe project-shape fixes defined by psx.yml.
	@command -v $(PSX) >/dev/null 2>&1 || { echo "PSX not found; install github.com/m-mdy-m/psx/cmd/psx first." >&2; exit 2; }
	$(PSX) fix

tidy: ## Reconcile go.mod and go.sum.
	$(GO) mod tidy

clean: ## Remove build outputs and coverage data.
	rm -rf $(BIN_DIR) coverage.out

help: ## List Make targets.
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z0-9_-]+:.*##/ {printf "  %-14s %s\n", $$1, $$2}' $(MAKEFILE_LIST)
