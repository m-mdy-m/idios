#!/usr/bin/env sh
# Smoke-test the single CLI entry point, Web server command and SQLite workflow.
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
CLI=${IDIOS_BIN:-"$ROOT/dist/idios"}
[ -f "$CLI" ] || { echo "IDIOS executable not found: $CLI" >&2; exit 2; }
case "$(uname -s 2>/dev/null || true)" in Linux|Darwin) chmod u+x "$CLI" ;; esac

"$CLI" help >/dev/null
version=$("$CLI" version)
if [ -n "${IDIOS_EXPECT_VERSION:-}" ] && [ "$version" != "$IDIOS_EXPECT_VERSION" ]; then
  echo "FAIL: IDIOS reports version $version; expected $IDIOS_EXPECT_VERSION." >&2; exit 1
fi
"$CLI" >"${TMPDIR:-/tmp}/idios-cli-help.$$.txt"
grep -q 'Usage:' "${TMPDIR:-/tmp}/idios-cli-help.$$.txt" || { echo 'FAIL: bare idios invocation must print CLI help.' >&2; rm -f "${TMPDIR:-/tmp}/idios-cli-help.$$.txt"; exit 1; }
rm -f "${TMPDIR:-/tmp}/idios-cli-help.$$.txt"
IDIOS_BIN="$CLI" sh "$ROOT/scripts/smoke.sh"

echo 'PASS: single CLI entry point, bare invocation, and SQLite workflow passed.'
