#!/usr/bin/env bash
# Run the test suite. No network, no API keys.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
exec "${PYTHON_BIN:-python3}" -m pytest -q "$@"
