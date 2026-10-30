#!/usr/bin/env bash
set -euo pipefail
if [[ $# -ne 3 ]]; then echo "Usage: package-macos.sh <version> <tag> <asset>" >&2; exit 2; fi
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VERSION=$1; RELEASE_TAG=$2; ASSET=$3
WORK_DIR=$(mktemp -d "${TMPDIR:-/tmp}/idios-package.XXXXXX")
trap 'rm -rf "$WORK_DIR"' EXIT
case "$ASSET" in darwin-amd64) TARGET_GOARCH=amd64 ;; darwin-arm64) TARGET_GOARCH=arm64 ;; *) echo "Unsupported target: $ASSET" >&2; exit 2 ;; esac
PAYLOAD="$WORK_DIR/payload"
APP="$PAYLOAD/Applications/IDIOS.app"
mkdir -p "$PAYLOAD/usr/local/bin" "$PAYLOAD/usr/local/share/doc/idios" "$APP/Contents/MacOS" "$APP/Contents/Resources" "$WORK_DIR/dmg"
install -m 0755 "$ROOT/dist/idios" "$PAYLOAD/usr/local/bin/idios"
install -m 0644 "$ROOT/README.md" "$PAYLOAD/usr/local/share/doc/idios/README.md"
install -m 0644 "$ROOT/LICENSE" "$PAYLOAD/usr/local/share/doc/idios/LICENSE"
install -m 0755 "$ROOT/dist/idios" "$APP/Contents/Resources/idios"
(
  cd "$ROOT"
  GOOS=darwin GOARCH="$TARGET_GOARCH" CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o "$APP/Contents/MacOS/IDIOS" ./packaging/macos/launcher
)
chmod 0755 "$APP/Contents/MacOS/IDIOS"
ICONSET="$WORK_DIR/IDIOS.iconset"
mkdir -p "$ICONSET"
SOURCE_ICON="$ROOT/clients/web/public/favicon.png"
for size in 16 32 128 256 512; do
  sips -z "$size" "$size" "$SOURCE_ICON" --out "$ICONSET/icon_${size}x${size}.png" >/dev/null
  double=$((size * 2))
  sips -z "$double" "$double" "$SOURCE_ICON" --out "$ICONSET/icon_${size}x${size}@2x.png" >/dev/null
done
iconutil -c icns "$ICONSET" -o "$APP/Contents/Resources/IDIOS.icns"
cat > "$APP/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleExecutable</key><string>IDIOS</string>
  <key>CFBundleIdentifier</key><string>io.github.m-mdy-m.idios.desktop</string>
  <key>CFBundleName</key><string>IDIOS</string>
  <key>CFBundleDisplayName</key><string>IDIOS</string>
  <key>CFBundlePackageType</key><string>APPL</string>
  <key>CFBundleVersion</key><string>$VERSION</string>
  <key>CFBundleShortVersionString</key><string>$VERSION</string>
  <key>CFBundleIconFile</key><string>IDIOS.icns</string>
  <key>LSMinimumSystemVersion</key><string>11.0</string>
  <key>LSUIElement</key><true/>
</dict>
</plist>
PLIST
/usr/libexec/PlistBuddy -c 'Print :CFBundleExecutable' "$APP/Contents/Info.plist" >/dev/null
mkdir -p "$ROOT/release"
cp -R "$APP" "$ROOT/release/IDIOS.app"
PKG="$ROOT/idios-${RELEASE_TAG}-${ASSET}.pkg"
DMG="$ROOT/idios-${RELEASE_TAG}-${ASSET}.dmg"
pkgbuild --root "$PAYLOAD" --identifier "io.github.m-mdy-m.idios" --version "$VERSION" --install-location / "$PKG"
cp -R "$APP" "$WORK_DIR/dmg/IDIOS.app"
cp "$PKG" "$WORK_DIR/dmg/"
cp "$ROOT/README.md" "$ROOT/LICENSE" "$WORK_DIR/dmg/"
hdiutil create -volname "IDIOS $RELEASE_TAG $ASSET" -srcfolder "$WORK_DIR/dmg" -ov -format UDZO "$DMG"
echo "Created $PKG"
echo "Created $DMG and IDIOS.app"
