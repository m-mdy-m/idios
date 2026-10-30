#!/usr/bin/env sh
# Verify the release bundle contains the single end-user executable and required documentation.
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$ROOT"
CLI=${IDIOS_BIN:-"$ROOT/dist/idios"}
[ -f "$CLI" ] || { echo "Required release executable not found: $CLI" >&2; exit 2; }
CLI_DIR=$(dirname -- "$CLI")
for stale in idios-tui idios-tui.exe idios-tui-demo idios-tui-demo.exe; do
  if [ -e "$CLI_DIR/$stale" ]; then echo "FAIL: retired TUI artifact remains in release: $stale" >&2; exit 1; fi
done
"$CLI" help >/dev/null
version=$("$CLI" version)
if [ -n "${IDIOS_EXPECT_VERSION:-}" ] && [ "$version" != "$IDIOS_EXPECT_VERSION" ]; then
  echo "FAIL: IDIOS reports version $version; expected $IDIOS_EXPECT_VERSION." >&2; exit 1
fi
TMP=$(mktemp -d "${TMPDIR:-/tmp}/idios-release-smoke.XXXXXX")
trap 'rm -rf "$TMP"' EXIT HUP INT TERM
CONFIG_PATH="$TMP/config.json"
case "$(uname -s 2>/dev/null || true)" in MINGW*|MSYS*|CYGWIN*) command -v cygpath >/dev/null 2>&1 && CONFIG_PATH=$(cygpath -w "$CONFIG_PATH") ;; esac
# A bare invocation is the scriptable CLI help path and never needs a TTY.
IDIOS_CONFIG="$CONFIG_PATH" TERM=dumb "$CLI" >"$TMP/idios-no-args.out"
grep -q 'Usage:' "$TMP/idios-no-args.out" || { echo 'FAIL: bare invocation did not print CLI help.' >&2; cat "$TMP/idios-no-args.out" >&2; exit 1; }
IDIOS_CONFIG="$CONFIG_PATH" "$CLI" --db "$TMP/engine.sqlite3" repo list >/dev/null
printf '%s\n' 'PASS: release contains one CLI executable, clean help path and working SQLite initialization.'
