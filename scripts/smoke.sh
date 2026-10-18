#!/usr/bin/env sh
# End-to-end smoke test for the required cross-repository PR workflow.
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
BIN=${IDIOS_BIN:-"$ROOT/dist/idios"}
if [ ! -x "$BIN" ]; then
  echo "IDIOS binary not found/executable: $BIN" >&2
  echo "Run 'make build' first or set IDIOS_BIN." >&2
  exit 2
fi
TMP=$(mktemp -d "${TMPDIR:-/tmp}/idios-smoke.XXXXXX")
trap 'rm -rf "$TMP"' EXIT HUP INT TERM
DB="$TMP/engine.sqlite3"
run() { "$BIN" --db "$DB" "$@"; }

run repo create --name CPU --description 'CPU learning repository' >/dev/null
run repo create --name ARLIZ --description 'Writing repository' >/dev/null
run concept create --name 'Electricity fundamentals' >/dev/null
run issue create --repo CPU --title 'Learn basic electricity' >/dev/null
run issue create --repo ARLIZ --title 'Write chapter three' >/dev/null
run issue concept --issue ARLIZ#1 --concept 'Electricity fundamentals' >/dev/null
run issue depends --issue ARLIZ#1 --on CPU#1 >/dev/null

if run issue start --id ARLIZ#1 >"$TMP/blocked.out" 2>"$TMP/blocked.err"; then
  echo 'FAIL: dependent Issue started before its prerequisite was done.' >&2
  exit 1
fi
if ! grep -qi 'blocked\|prerequisite\|depend' "$TMP/blocked.out" "$TMP/blocked.err"; then
  echo 'FAIL: start failed, but output did not explain the dependency blocker.' >&2
  cat "$TMP/blocked.out" "$TMP/blocked.err" >&2
  exit 1
fi

run issue start --id CPU#1 >/dev/null
run pr create --issue CPU#1 --description 'Reviewed the basics and documented the result.' >/dev/null
run pr accept --id CPU!1 --summary 'Electricity fundamentals reviewed' >/dev/null
run issue start --id ARLIZ#1 >/dev/null
run pr create --issue ARLIZ#1 --description 'Drafted chapter three using the linked concepts.' >/dev/null
run pr accept --id ARLIZ!1 --summary 'Chapter three drafted and reviewed' >/dev/null

run --json issue show --id CPU#1 >"$TMP/cpu.json"
run --json issue show --id ARLIZ#1 >"$TMP/arliz.json"
if ! grep -q '"status": "done"' "$TMP/cpu.json"; then
  echo 'FAIL: CPU Issue did not become done after PR acceptance.' >&2
  cat "$TMP/cpu.json" >&2
  exit 1
fi
if ! grep -q '"status": "done"' "$TMP/arliz.json"; then
  echo 'FAIL: ARLIZ Issue did not become done after PR acceptance.' >&2
  cat "$TMP/arliz.json" >&2
  exit 1
fi
run graph --from issue:ARLIZ#1 --depth 3 >"$TMP/graph.txt"
if ! grep -q 'depends_on' "$TMP/graph.txt" || ! grep -q 'completion:CPU/main/1' "$TMP/graph.txt"; then
  echo 'FAIL: cross-repository dependency/completion was not present in the graph.' >&2
  cat "$TMP/graph.txt" >&2
  exit 1
fi
run branch list --repo ARLIZ >"$TMP/branches.txt"
if ! grep -q 'main' "$TMP/branches.txt"; then
  echo 'FAIL: default main Branch missing.' >&2
  cat "$TMP/branches.txt" >&2
  exit 1
fi
run completion list >"$TMP/completions.txt"
if ! grep -q 'CPU/main #1' "$TMP/completions.txt" || ! grep -q 'ARLIZ/main #1' "$TMP/completions.txt"; then
  echo 'FAIL: completion history missing accepted work.' >&2
  cat "$TMP/completions.txt" >&2
  exit 1
fi

echo 'PASS: cross-repository blockers, required PRs, CompletionRecords on main, and graph all work.'
