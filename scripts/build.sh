#!/usr/bin/env bash
# Build an sdist and a wheel into dist/.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
"${PYTHON_BIN:-python3}" -m pip install --quiet build
"${PYTHON_BIN:-python3}" -m build
