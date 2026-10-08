# idios: a local-first personal learning environment
#
# Common entry points. Run `make help` to list targets.

SHELL := bash
.DEFAULT_GOAL := help
PYTHON ?= python3

.PHONY: help setup run test lint check psx examples export build clean

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'

setup: ## Create .venv and install idios with dev tools
	./scripts/setup.sh

run: ## Start the interactive learning shell
	$(PYTHON) -m idios

test: ## Run the test suite (offline)
	./scripts/test.sh

lint: ## Syntax-check every module
	$(PYTHON) -m compileall -q src tests

psx: ## Check the project's shape with psx (README, tests, CI, ...)
	psx check --fail-on error

check: lint test ## Lint + tests

examples: ## Run every example script against a throwaway data folder
	./examples/run.sh --all

build: ## Build sdist and wheel
	./scripts/build.sh

clean: ## Remove build artifacts and caches
	./scripts/clean.sh
