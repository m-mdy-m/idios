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
install -m 0755 dist/idios-tui "$WORK_DIR/root/bin/idios-tui"
install -m 0755 dist/idios-tui-demo "$WORK_DIR/root/bin/idios-tui-demo"
install -m 0644 README.md "$WORK_DIR/root/share/doc/idios/README.md"
install -m 0644 LICENSE "$WORK_DIR/root/share/doc/idios/LICENSE"
install -m 0644 docs/user/GETTING_STARTED.md "$WORK_DIR/root/share/doc/idios/GETTING_STARTED.md"
install -m 0644 docs/user/WORKSPACE_GUIDE.md "$WORK_DIR/root/share/doc/idios/WORKSPACE_GUIDE.md"
install -m 0644 docs/user/SETTINGS.md "$WORK_DIR/root/share/doc/idios/SETTINGS.md"

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
cp README.md LICENSE docs/user/GETTING_STARTED.md docs/user/WORKSPACE_GUIDE.md docs/user/SETTINGS.md "$WORK_DIR/dmg/"
cat > "$WORK_DIR/dmg/INSTALL.txt" <<EOF
IDIOS $RELEASE_TAG ($ASSET)

Double-click the included idios-$RELEASE_TAG-$ASSET.pkg to install IDIOS.
The installer places:
  idios           -> /usr/local/bin/idios
  idios-tui       -> /usr/local/bin/idios-tui
  idios-tui-demo  -> /usr/local/bin/idios-tui-demo
  user guides     -> /usr/local/share/doc/idios/

After installation, open Terminal and run:
  idios help

Launch the clean workspace from an interactive terminal with:
  idios-tui

Launch the sample-data workspace from an interactive terminal with:
  idios-tui-demo

This package is not code-signed or notarized. macOS may show a security
warning when opening the installer. Production distribution should add
Developer ID signing and Apple notarization.
EOF

hdiutil create \
  -volname "IDIOS $RELEASE_TAG $ASSET" \
  -srcfolder "$WORK_DIR/dmg" \
  -ov -format UDZO \
  "$DMG"

echo "Created $PKG"
echo "Created $DMG"
