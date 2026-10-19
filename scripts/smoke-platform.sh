#!/usr/bin/env sh
# Verify all shipped entry points, clean/demo database isolation, and a SQLite-backed workflow.
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$ROOT"
CLI=${IDIOS_BIN:-"$ROOT/dist/idios"}
TUI=${IDIOS_TUI_BIN:-"$ROOT/dist/idios-tui"}
TUI_DEMO=${IDIOS_TUI_DEMO_BIN:-"$ROOT/dist/idios-tui-demo"}

if [ ! -f "$CLI" ]; then
  echo "IDIOS CLI binary not found: $CLI" >&2
  exit 2
fi
if [ ! -f "$TUI" ]; then
  echo "IDIOS clean TUI binary not found: $TUI" >&2
  exit 2
fi
if [ ! -f "$TUI_DEMO" ]; then
  echo "IDIOS demo TUI binary not found: $TUI_DEMO" >&2
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
CONFIG_PATH="$TMP/config.json"
CLEAN_DB="$TMP/clean.sqlite3"
DEMO_DB="$TMP/demo.sqlite3"
case "$(uname -s 2>/dev/null || true)" in
  MINGW*|MSYS*|CYGWIN*)
    if command -v cygpath >/dev/null 2>&1; then
      CONFIG_PATH=$(cygpath -w "$CONFIG_PATH")
      CLEAN_DB=$(cygpath -w "$CLEAN_DB")
      DEMO_DB=$(cygpath -w "$DEMO_DB")
    fi
    ;;
esac

# A fresh standard TUI database must remain empty; standard TUI never seeds sample records.
clean_out=$(IDIOS_CONFIG="$CONFIG_PATH" "$TUI" --check --db "$CLEAN_DB")
printf '%s\n' "$clean_out" | grep -q 'READY mode=idios-tui repositories=0' || {
  echo "FAIL: standard TUI did not initialize an empty workspace." >&2
  printf '%s\n' "$clean_out" >&2
  exit 1
}

# Demo TUI must honor IDIOS_DEMO_DB, ignore IDIOS_DB, seed separately, and remain idempotent.
demo_out=$(IDIOS_DB="$CLEAN_DB" IDIOS_DEMO_DB="$DEMO_DB" IDIOS_CONFIG="$CONFIG_PATH" "$TUI_DEMO" --check)
printf '%s\n' "$demo_out" | grep -Eq 'READY mode=idios-tui-demo repositories=[1-9][0-9]*' || {
  echo "FAIL: demo TUI did not populate its isolated sample database." >&2
  printf '%s\n' "$demo_out" >&2
  exit 1
}
demo_again=$(IDIOS_DB="$CLEAN_DB" IDIOS_DEMO_DB="$DEMO_DB" IDIOS_CONFIG="$CONFIG_PATH" "$TUI_DEMO" --check)
printf '%s\n' "$demo_again" | grep -Eq 'READY mode=idios-tui-demo repositories=[1-9][0-9]*' || {
  echo "FAIL: demo TUI could not reopen its seeded database." >&2
  printf '%s\n' "$demo_again" >&2
  exit 1
}

# Seeding the demo database must not mutate the clean database.
clean_again=$(IDIOS_CONFIG="$CONFIG_PATH" "$TUI" --check --db "$CLEAN_DB")
printf '%s\n' "$clean_again" | grep -q 'READY mode=idios-tui repositories=0' || {
  echo "FAIL: demo data leaked into the standard TUI database." >&2
  printf '%s\n' "$clean_again" >&2
  exit 1
}

# Full-screen variants must still clearly reject a non-interactive terminal.
set +e
IDIOS_CONFIG="$CONFIG_PATH" TERM=dumb "$TUI" --db "$TMP/noninteractive.sqlite3" >"$TMP/tui.out" 2>&1
status=$?
set -e
if [ "$status" -ne 2 ] || ! grep -qi 'needs an interactive terminal' "$TMP/tui.out"; then
  echo "FAIL: idios-tui did not report its non-interactive-terminal requirement (exit=$status)." >&2
  cat "$TMP/tui.out" >&2
  exit 1
fi

echo 'PASS: CLI + SQLite workflow succeeds; clean TUI starts with zero demo repositories; demo TUI seeds an isolated database idempotently; both UI variants are packaged.'
