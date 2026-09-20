#!/usr/bin/env bash
# Bootstrap a local dev environment for IDIOS.
# Usage: ./scripts/bootstrap.sh
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

PYTHON_BIN="${PYTHON_BIN:-python3}"

echo "Using $($PYTHON_BIN --version)"

if [ ! -d ".venv" ]; then
  "$PYTHON_BIN" -m venv .venv
fi

# shellcheck disable=SC1091
source .venv/bin/activate

pip install --upgrade pip
pip install -e ".[dev]"

echo
echo "Environment ready. Try:"
echo "  source .venv/bin/activate"
echo "  idios status"
echo "  pytest -q"
