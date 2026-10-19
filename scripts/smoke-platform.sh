#!/usr/bin/env sh
# Verify both shipped executables run, and exercise a complete SQLite-backed workflow.
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$ROOT"
CLI=${IDIOS_BIN:-"$ROOT/dist/idios"}
TUI=${IDIOS_TUI_BIN:-"$ROOT/dist/idios-tui"}

if [ ! -f "$CLI" ]; then
  echo "IDIOS CLI binary not found: $CLI" >&2
  exit 2
fi
if [ ! -f "$TUI" ]; then
  echo "IDIOS TUI binary not found: $TUI" >&2
  exit 2
fi

"$CLI" help >/dev/null
version=$("$CLI" version)
if [ -n "${IDIOS_EXPECT_VERSION:-}" ] && [ "$version" != "$IDIOS_EXPECT_VERSION" ]; then
  echo "FAIL: downloaded CLI reports version $version; expected $IDIOS_EXPECT_VERSION." >&2
  exit 1
fi
IDIOS_BIN="$CLI" sh "$ROOT/scripts/smoke.sh"

TMP=$(mktemp -d "${TMPDIR:-/tmp}/idios-platform-smoke.XXXXXX")
trap 'rm -rf "$TMP"' EXIT HUP INT TERM
set +e
CONFIG_PATH="$TMP/config.json"
case "$(uname -s 2>/dev/null || true)" in
  MINGW*|MSYS*|CYGWIN*)
    if command -v cygpath >/dev/null 2>&1; then CONFIG_PATH=$(cygpath -w "$CONFIG_PATH"); fi
    ;;
esac
IDIOS_CONFIG="$CONFIG_PATH" TERM=dumb "$TUI" --db "$TMP/tui.sqlite3" >"$TMP/tui.out" 2>&1
status=$?
set -e
if [ "$status" -ne 2 ] || ! grep -qi 'needs an interactive terminal' "$TMP/tui.out"; then
  echo "FAIL: idios-tui did not start and report its non-interactive-terminal requirement (exit=$status)." >&2
  cat "$TMP/tui.out" >&2
  exit 1
fi

echo 'PASS: both executables launch; CLI + SQLite workflow succeeds; TUI reports non-interactive mode clearly.'
