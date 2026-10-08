#!/usr/bin/env bash
# Remove build artifacts and caches. Never touches ~/.idios (your data).
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
rm -rf build dist .pytest_cache src/*.egg-info
find . -name __pycache__ -type d -prune -exec rm -rf {} +
echo "Cleaned."
