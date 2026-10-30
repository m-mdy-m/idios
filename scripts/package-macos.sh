#!/usr/bin/env bash
# Build a native macOS installer package plus a DMG containing that installer.
set -euo pipefail

if [[ $# -ne 3 ]]; then
  echo "Usage: package-macos.sh <version-without-v> <release-tag> <asset-name>" >&2
  exit 2
fi

VERSION=$1
RELEASE_TAG=$2
ASSET=$3
OUT_DIR=.
WORK_DIR=$(mktemp -d "${TMPDIR:-/tmp}/idios-package.XXXXXX")
trap 'rm -rf "$WORK_DIR"' EXIT

mkdir -p "$WORK_DIR/root/bin" "$WORK_DIR/root/share/doc/idios" "$WORK_DIR/dmg"
install -m 0755 dist/idios "$WORK_DIR/root/bin/idios"
install -m 0644 README.md "$WORK_DIR/root/share/doc/idios/README.md"
install -m 0644 LICENSE "$WORK_DIR/root/share/doc/idios/LICENSE"

PKG="$OUT_DIR/idios-${RELEASE_TAG}-${ASSET}.pkg"
DMG="$OUT_DIR/idios-${RELEASE_TAG}-${ASSET}.dmg"

# Installs binaries into /usr/local/bin and documentation into
# /usr/local/share/doc/idios. Packages are intentionally not signed here;
# signing/notarization require the maintainer's Apple Developer credentials.
pkgbuild \
  --root "$WORK_DIR/root" \
  --identifier "io.github.m-mdy-m.idios" \
  --version "$VERSION" \
  --install-location /usr/local \
  "$PKG"

cp "$PKG" "$WORK_DIR/dmg/"
cp README.md LICENSE "$WORK_DIR/dmg/"
hdiutil create \
  -volname "IDIOS $RELEASE_TAG $ASSET" \
  -srcfolder "$WORK_DIR/dmg" \
  -ov -format UDZO \
  "$DMG"

echo "Created $PKG"
echo "Created $DMG"
