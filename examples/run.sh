#!/usr/bin/env bash
# Play example scripts against a throwaway data folder (your ~/.idios is untouched).
#   ./examples/run.sh 01            one example (by number or file name)
#   ./examples/run.sh --all         every example
set -euo pipefail
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PYTHON_BIN="${PYTHON_BIN:-python3}"
export PYTHONPATH="$here/../src${PYTHONPATH:+:$PYTHONPATH}"

play() {
  local home; home="$(mktemp -d)"
  echo "━━ $(basename "$1")"
  case "$1" in
    *.py)    IDIOS_HOME="$home" "$PYTHON_BIN" "$1" ;;
    *.idios) "$PYTHON_BIN" -m idios --home "$home" run "$1" ;;
  esac
  rm -rf "$home"; echo
}

[ $# -ge 1 ] || { sed -n 2,5p "$0"; exit 1; }
if [ "$1" = "--all" ]; then
  for f in "$here"/[0-9]*.idios "$here"/[0-9]*.py; do play "$f"; done
else
  for f in "$here"/"$1"*; do play "$f"; done
fi
