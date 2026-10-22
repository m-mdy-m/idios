GO ?= go
PSX ?= psx
BIN_DIR ?= dist
DESTDIR ?=
CGO_ENABLED ?= 1
PNPM ?= pnpm
WEB_DIR = clients/web
FMT_DIRS = cmd internal clients

# cmd.exe cannot run "VAR=1 prog", so Windows exports the variables with set first.
ifeq ($(OS),Windows_NT)
RUN_GO = cmd /c set CGO_ENABLED=$(CGO_ENABLED)&& set GOOS=$(GOOS)&&
else
RUN_GO = env CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS)
endif


ifeq ($(OS),Windows_NT)
PREFIX ?= $(if $(LOCALAPPDATA),$(LOCALAPPDATA)/Programs/IDIOS,$(USERPROFILE)/AppData/Local/Programs/IDIOS)
GOOS ?= windows
EXE_EXT ?= .exe
VERSION ?= $(if $(GITHUB_REF_NAME),$(patsubst v%,%,$(GITHUB_REF_NAME)),$(if $(IDIOS_VERSION),$(IDIOS_VERSION),2.0.0))
else
PREFIX ?= /usr/local
GOOS ?= $(shell uname -s | tr '[:upper:]' '[:lower:]')
EXE_EXT ?= $(if $(filter windows,$(GOOS)),.exe,)
VERSION ?= $(shell tag=$$(git describe --tags --abbrev=0 2>/dev/null); if [ -n "$$tag" ]; then printf '%s' "$$tag" | sed 's/^v//'; else printf '2.0.0'; fi)
endif
CLI_LDFLAGS = -s -w -X idios/clients/cli.Version=$(VERSION) -X idios/internal/updater.Version=$(VERSION)
TUI_LDFLAGS = -s -w -X idios/internal/updater.Version=$(VERSION)
CLI_BIN = $(BIN_DIR)/idios$(EXE_EXT)
TUI_BIN = $(BIN_DIR)/idios-tui$(EXE_EXT)
DEMO_BIN = $(BIN_DIR)/idios-tui-demo$(EXE_EXT)

.PHONY: all build web-install web-build web-lint install install-user uninstall tui tui-demo test test-race test-cover vet fmt fmt-check check ci smoke psx-check psx-fix tidy clean help

all: ci

## Web frontend
web-install: ## Install the web UI's development/build dependencies (pnpm only; no Node runtime is needed after packaging).
	cd $(WEB_DIR) && $(PNPM) install --no-frozen-lockfile

web-build: web-install ## Build the React/TypeScript UI into the static bundle embedded in the Go server.
	cd $(WEB_DIR) && $(PNPM) run build

web-lint: web-install ## Type/lint-check the web UI source.
	cd $(WEB_DIR) && $(PNPM) run lint

## Build
ifeq ($(OS),Windows_NT)
build: ## Build the bundled Web UI, CLI, clean TUI, and demo TUI using PowerShell on Windows.
	powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts/build.ps1 -Version "$(VERSION)"
else
build: web-build ## Build the bundled Web UI, CLI, clean TUI, and demo TUI into $(BIN_DIR)/.
	mkdir -p $(BIN_DIR)
	$(RUN_GO) $(GO) build -tags=embed_web -trimpath -ldflags='$(CLI_LDFLAGS)' -o $(CLI_BIN) ./cmd/idios
	$(RUN_GO) $(GO) build -trimpath -ldflags='$(TUI_LDFLAGS)' -o $(TUI_BIN) ./cmd/idios-tui
	$(RUN_GO) $(GO) build -trimpath -ldflags='$(TUI_LDFLAGS)' -o $(DEMO_BIN) ./cmd/idios-tui-demo
endif


## Installation
ifeq ($(OS),Windows_NT)
install: build ## Install idios and idios-tui for the current Windows user (PREFIX can be overridden).
	powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts/install-local.ps1 -SourceDir "$(BIN_DIR)" -InstallDir "$(PREFIX)"

install-user: install ## Install for the current user without administrator privileges on Windows.

uninstall: ## Remove IDIOS executables installed by make on Windows.
	powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts/install-local.ps1 -InstallDir "$(PREFIX)" -Uninstall
else
install: build ## Install idios and idios-tui into $(DESTDIR)$(PREFIX)/bin (may require sudo).
	install -d "$(DESTDIR)$(PREFIX)/bin"
	install -m 0755 "$(CLI_BIN)" "$(DESTDIR)$(PREFIX)/bin/idios"
	install -m 0755 "$(TUI_BIN)" "$(DESTDIR)$(PREFIX)/bin/idios-tui"

install-user: PREFIX = $(HOME)/.local
install-user: install ## Install for the current user without administrator privileges.
	@for profile in "$(HOME)/.profile" "$(HOME)/.zprofile"; do \
	  if [ ! -f "$$profile" ] || ! grep -Fq '$$HOME/.local/bin' "$$profile"; then \
	    printf '\n# IDIOS user-local binaries\nexport PATH="$$HOME/.local/bin:$$PATH"\n' >> "$$profile"; \
	  fi; \
	done
	@echo "Installed IDIOS in $(HOME)/.local/bin; reopen your terminal if PATH is not updated."

uninstall: ## Remove the installed IDIOS executables from $(DESTDIR)$(PREFIX)/bin.
	rm -f "$(DESTDIR)$(PREFIX)/bin/idios" "$(DESTDIR)$(PREFIX)/bin/idios-tui"
endif

## Interactive UI
tui: ## Run the full-screen, clean TUI against the standard Engine database.
	$(RUN_GO) $(GO) run ./cmd/idios-tui

tui-demo: ## Run the demo TUI against its isolated, automatically seeded database.
	$(RUN_GO) $(GO) run ./cmd/idios-tui-demo

## Quality
fmt: ## Format Go source files in-place.
	$(GO) fmt ./...

ifeq ($(OS),Windows_NT)
fmt-check: ## Fail if any Go source file is not gofmt-formatted.
	powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts/fmt_check.ps1
else
fmt-check: ## Fail if any Go source file is not gofmt-formatted.
	@unformatted="$$(gofmt -l $(FMT_DIRS))"; \
	if [ -n "$$unformatted" ]; then echo "Unformatted Go files:"; echo "$$unformatted"; exit 1; fi
endif

test: ## Run all tests.
	$(RUN_GO) $(GO) test ./...

test-race: ## Run all tests under the race detector.
	$(RUN_GO) $(GO) test -race ./...

test-cover: ## Run tests and write coverage.out.
	$(RUN_GO) $(GO) test -coverprofile=coverage.out ./...

vet: ## Run go vet.
	$(RUN_GO) $(GO) vet ./...

check: fmt-check test vet web-lint ## Run Go and web-source quality checks.

ci: check test-race build smoke ## Run the full local CI gate (PSX is a separate optional target).

# Windows runs the POSIX smoke script through Git Bash (the script handles MSYS paths via cygpath).
ifeq ($(OS),Windows_NT)
GIT_BASH ?= C:/Program Files/Git/bin/bash.exe
endif

ifeq ($(OS),Windows_NT)
smoke: build ## Smoke-test all executables, clean/demo database isolation, and a cross-Repository SQLite workflow (run inside Git Bash on Windows).
	set IDIOS_BIN=$(CLI_BIN)&& set IDIOS_TUI_BIN=$(TUI_BIN)&& set IDIOS_TUI_DEMO_BIN=$(DEMO_BIN)&& "$(GIT_BASH)" ./scripts/smoke-platform.sh
else
smoke: build ## Smoke-test all executables, clean/demo database isolation, and a cross-Repository SQLite workflow.
	IDIOS_BIN=$(CLI_BIN) IDIOS_TUI_BIN=$(TUI_BIN) IDIOS_TUI_DEMO_BIN=$(DEMO_BIN) sh ./scripts/smoke-platform.sh
endif

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
