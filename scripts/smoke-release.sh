#!/usr/bin/env sh
# Verify only the end-user CLI and clean TUI are present and runnable.
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$ROOT"
CLI=${IDIOS_BIN:-"$ROOT/dist/idios"}
TUI=${IDIOS_TUI_BIN:-"$ROOT/dist/idios-tui"}

for binary in "$CLI" "$TUI"; do
  if [ ! -f "$binary" ]; then
    echo "Required release binary not found: $binary" >&2
    exit 2
  fi
done

CLI_DIR=$(dirname -- "$CLI")
if [ -e "$CLI_DIR/idios-tui-demo" ] || [ -e "$CLI_DIR/idios-tui-demo.exe" ]; then
  echo "FAIL: demo TUI must not be present in a user release." >&2
  exit 1
fi

# Do not chmod release binaries here: packaged/installed binaries must already
# carry executable permissions, and chmod may be prohibited on mounted paths
# in containerized package-verification runners.

"$CLI" help >/dev/null
version=$("$CLI" version)
if [ -n "${IDIOS_EXPECT_VERSION:-}" ] && [ "$version" != "$IDIOS_EXPECT_VERSION" ]; then
  echo "FAIL: release CLI reports version $version; expected $IDIOS_EXPECT_VERSION." >&2
  exit 1
fi

TMP=$(mktemp -d "${TMPDIR:-/tmp}/idios-release-smoke.XXXXXX")
trap 'rm -rf "$TMP"' EXIT HUP INT TERM
CONFIG_PATH="$TMP/config.json"
CLEAN_DB="$TMP/clean.sqlite3"

case "$(uname -s 2>/dev/null || true)" in
  MINGW*|MSYS*|CYGWIN*)
    if command -v cygpath >/dev/null 2>&1; then
      CONFIG_PATH=$(cygpath -w "$CONFIG_PATH")
      CLEAN_DB=$(cygpath -w "$CLEAN_DB")
    fi
    ;;
esac

# With no arguments, `idios` is the interactive application entry point.
set +e
IDIOS_CONFIG="$CONFIG_PATH" TERM=dumb "$CLI" >"$TMP/idios-no-args.out" 2>&1
interactive_status=$?
set -e
if [ "$interactive_status" -ne 2 ] || ! grep -qi 'needs an interactive terminal' "$TMP/idios-no-args.out"; then
  echo "FAIL: running idios without arguments did not enter the interactive TUI path (exit=$interactive_status)." >&2
  cat "$TMP/idios-no-args.out" >&2
  exit 1
fi

clean_out=$(IDIOS_CONFIG="$CONFIG_PATH" "$TUI" --check --db "$CLEAN_DB")
printf '%s\n' "$clean_out" | grep -q 'READY mode=idios-tui repositories=0' || {
  echo "FAIL: release TUI did not initialize a clean workspace." >&2
  printf '%s\n' "$clean_out" >&2
  exit 1
}

set +e
IDIOS_CONFIG="$CONFIG_PATH" TERM=dumb "$TUI" --db "$TMP/noninteractive.sqlite3" >"$TMP/tui.out" 2>&1
status=$?
set -e
if [ "$status" -ne 2 ] || ! grep -qi 'needs an interactive terminal' "$TMP/tui.out"; then
  echo "FAIL: release TUI did not reject a non-interactive terminal (exit=$status)." >&2
  cat "$TMP/tui.out" >&2
  exit 1
fi

echo 'PASS: release contains only the CLI and clean TUI; CLI and empty-workspace TUI checks passed.'
