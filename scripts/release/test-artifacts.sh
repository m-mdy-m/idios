#!/usr/bin/env bash
set -euo pipefail
TARGET_ID="$TARGET_ID"
ARTIFACT_DIR="$IDIOS_ARTIFACT_DIR"
[ -n "$TARGET_ID" ] || { echo "TARGET_ID is required" >&2; exit 2; }
[ -n "$ARTIFACT_DIR" ] || { echo "IDIOS_ARTIFACT_DIR is required" >&2; exit 2; }
WORK_DIR="$(mktemp -d "/tmp/idios-release-test.XXXXXX")"
PORT=43178
BASE_URL="http://127.0.0.1:$PORT"
SERVER_PID=""
MOUNT_POINT="$WORK_DIR/dmg"
cleanup() {
  if [ -n "$SERVER_PID" ]; then kill -TERM "$SERVER_PID" 2>/dev/null || true; wait "$SERVER_PID" 2>/dev/null || true; fi
  if [ -d "$MOUNT_POINT" ] && mount | grep -Fq "$MOUNT_POINT"; then hdiutil detach "$MOUNT_POINT" -quiet 2>/dev/null || true; fi
  rm -rf "$WORK_DIR"
}
trap cleanup EXIT INT TERM
fail() { echo "FAIL [$TARGET_ID] $*" >&2; exit 1; }
pass() { echo "PASS [$TARGET_ID] $*"; }

case "$TARGET_ID" in
  linux-amd64|linux-arm64)
    ARCHIVE="$(find "$ARTIFACT_DIR" -maxdepth 1 -type f -name "idios-v*-$TARGET_ID.tar.gz" -print -quit)"
    DEB="$(find "$ARTIFACT_DIR" -maxdepth 1 -type f -name "idios-v*-$TARGET_ID.deb" -print -quit)"
    RPM="$(find "$ARTIFACT_DIR" -maxdepth 1 -type f -name "idios-v*-$TARGET_ID.rpm" -print -quit)"
    [ -n "$ARCHIVE" ] && [ -n "$DEB" ] && [ -n "$RPM" ] || fail "expected tar.gz, .deb and .rpm artifacts were not downloaded"
    mkdir -p "$WORK_DIR/archive"
    tar -xzf "$ARCHIVE" -C "$WORK_DIR/archive"
    CLI="$WORK_DIR/archive/idios"
    [ -x "$CLI" ] || fail "portable archive does not contain executable idios"
    [ -f "$WORK_DIR/archive/idios.desktop" ] || fail "portable archive is missing idios.desktop"
    [ -f "$WORK_DIR/archive/idios.svg" ] || fail "portable archive is missing idios.svg"
    grep -Fq 'Exec=idios serve --open' "$WORK_DIR/archive/idios.desktop" || fail "desktop Exec is not idios serve --open"
    grep -Fq 'Terminal=false' "$WORK_DIR/archive/idios.desktop" || fail "desktop launcher opens a terminal"
    DEB_ROOT="$WORK_DIR/deb-root"
    mkdir -p "$DEB_ROOT"
    dpkg-deb -x "$DEB" "$DEB_ROOT"
    [ -x "$DEB_ROOT/usr/bin/idios" ] || fail "DEB is missing /usr/bin/idios"
    [ -f "$DEB_ROOT/usr/share/applications/idios.desktop" ] || fail "DEB is missing its desktop entry"
    [ -f "$DEB_ROOT/usr/share/icons/hicolor/scalable/apps/idios.svg" ] || fail "DEB is missing its icon"
    RPM_FILES="$(rpm -qpl "$RPM")"
    printf '%s\n' "$RPM_FILES" | grep -Fq '/usr/share/applications/idios.desktop' || fail "RPM is missing its desktop entry"
    printf '%s\n' "$RPM_FILES" | grep -Fq '/usr/share/icons/hicolor/scalable/apps/idios.svg' || fail "RPM is missing its icon"
    ;;
  darwin-amd64|darwin-arm64)
    ARCHIVE="$(find "$ARTIFACT_DIR" -maxdepth 1 -type f -name "idios-v*-$TARGET_ID.tar.gz" -print -quit)"
    PKG="$(find "$ARTIFACT_DIR" -maxdepth 1 -type f -name "idios-v*-$TARGET_ID.pkg" -print -quit)"
    DMG="$(find "$ARTIFACT_DIR" -maxdepth 1 -type f -name "idios-v*-$TARGET_ID.dmg" -print -quit)"
    [ -n "$ARCHIVE" ] && [ -n "$PKG" ] && [ -n "$DMG" ] || fail "expected tar.gz, .pkg and .dmg artifacts were not downloaded"
    mkdir -p "$WORK_DIR/archive"
    tar -xzf "$ARCHIVE" -C "$WORK_DIR/archive"
    CLI="$WORK_DIR/archive/idios"
    APP="$WORK_DIR/archive/IDIOS.app"
    [ -x "$CLI" ] || fail "portable archive is missing executable idios"
    [ -x "$APP/Contents/MacOS/IDIOS" ] || fail "archive is missing an executable IDIOS.app"
    [ -x "$APP/Contents/Resources/idios" ] || fail "IDIOS.app is missing its bundled server"
    [ -f "$APP/Contents/Info.plist" ] || fail "IDIOS.app is missing Info.plist"
    [ -f "$APP/Contents/Resources/IDIOS.icns" ] || fail "IDIOS.app is missing its icon"
    plutil -lint "$APP/Contents/Info.plist" >/dev/null || fail "Info.plist is invalid"
    PKG_ROOT="$WORK_DIR/pkg-expanded"
    pkgutil --expand-full "$PKG" "$PKG_ROOT"
    find "$PKG_ROOT" -path '*/Applications/IDIOS.app/Contents/MacOS/IDIOS' -type f -print -quit | grep -q . || fail "PKG does not install IDIOS.app"
    find "$PKG_ROOT" -path '*/usr/local/bin/idios' -type f -print -quit | grep -q . || fail "PKG does not install the CLI"
    mkdir -p "$MOUNT_POINT"
    hdiutil attach "$DMG" -readonly -nobrowse -mountpoint "$MOUNT_POINT" -quiet
    [ -d "$MOUNT_POINT/IDIOS.app" ] || fail "DMG does not contain IDIOS.app"
    [ -f "$MOUNT_POINT/$(basename "$PKG")" ] || fail "DMG does not contain its PKG"
    hdiutil detach "$MOUNT_POINT" -quiet
    ;;
  *) fail "unsupported Unix target: $TARGET_ID" ;;
esac

FAKE_BIN="$WORK_DIR/fake-bin"
mkdir -p "$FAKE_BIN"
if [[ "$TARGET_ID" == linux-* ]]; then
  cat > "$FAKE_BIN/xdg-open" <<'OPEN'
#!/bin/sh
printf '%s\n' "$1" >> "$IDIOS_BROWSER_TEST_LOG"
OPEN
  chmod +x "$FAKE_BIN/xdg-open"
else
  cat > "$FAKE_BIN/open" <<'OPEN'
#!/bin/sh
printf '%s\n' "$1" >> "$IDIOS_BROWSER_TEST_LOG"
OPEN
  chmod +x "$FAKE_BIN/open"
fi

export IDIOS_CONFIG="$WORK_DIR/config.json"
export IDIOS_DB="$WORK_DIR/engine.sqlite3"
export IDIOS_BROWSER_TEST_LOG="$WORK_DIR/browser.log"
export PATH="$FAKE_BIN:$PATH"
printf '{"server":{"host":"127.0.0.1","port":%s,"open_browser":false}}\n' "$PORT" > "$IDIOS_CONFIG"
if [[ "$TARGET_ID" == darwin-* ]]; then
  "$APP/Contents/MacOS/IDIOS"
else
  "$CLI" serve --open --addr "127.0.0.1:$PORT" --db "$WORK_DIR/engine.sqlite3" >"$WORK_DIR/server.log" 2>&1 &
  SERVER_PID=$!
fi
ready=0
for _ in $(seq 1 100); do
  if curl --silent --fail "$BASE_URL/api/health" | grep -Fq '"status":"ok"'; then ready=1; break; fi
  sleep 0.2
done
[ "$ready" -eq 1 ] || { cat "$WORK_DIR/server.log" 2>/dev/null || true; fail "server did not become healthy at $BASE_URL"; }
opened=0
for _ in $(seq 1 50); do
  if [ -s "$IDIOS_BROWSER_TEST_LOG" ] && grep -Fq "$BASE_URL" "$IDIOS_BROWSER_TEST_LOG"; then opened=1; break; fi
  sleep 0.1
done
[ "$opened" -eq 1 ] || { cat "$WORK_DIR/server.log" 2>/dev/null || true; fail "browser handler was not called with $BASE_URL"; }

# A second click must reuse the existing server, not attempt to bind its port again.
if [[ "$TARGET_ID" == darwin-* ]]; then
  "$APP/Contents/MacOS/IDIOS"
else
  "$CLI" serve --open --addr "127.0.0.1:$PORT" --db "$WORK_DIR/engine.sqlite3"
fi
sleep 0.3
opened_twice="$(grep -Fc "$BASE_URL" "$IDIOS_BROWSER_TEST_LOG" || true)"
[ "$opened_twice" -ge 2 ] || fail "second launcher click did not reopen the existing workspace (browser calls: $opened_twice)"
"$CLI" server stop >/dev/null 2>&1 || true
pass "downloaded artifacts, shortcut startup, browser URL and repeat-click reuse"
