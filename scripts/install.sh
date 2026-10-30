#!/usr/bin/env sh
# Install the single IDIOS CLI/Web-server executable into a user-writable PATH directory.
set -eu
OWNER=m-mdy-m
REPO=idios
INSTALL_DIR=${IDIOS_INSTALL_DIR:-"$HOME/.local/bin"}
API_URL="https://api.github.com/repos/$OWNER/$REPO/releases/latest"
TMP_DIR=$(mktemp -d "${TMPDIR:-/tmp}/idios-install.XXXXXX")
trap 'rm -rf "$TMP_DIR"' EXIT HUP INT TERM
command -v curl >/dev/null 2>&1 || { echo 'IDIOS installer requires curl.' >&2; exit 1; }
command -v tar >/dev/null 2>&1 || { echo 'IDIOS installer requires tar.' >&2; exit 1; }
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in linux|darwin) ;; *) echo "Unsupported operating system: $OS (use the Windows PowerShell installer)." >&2; exit 1 ;; esac
case "$(uname -m)" in x86_64|amd64) ARCH=amd64 ;; aarch64|arm64) ARCH=arm64 ;; *) echo "Unsupported CPU architecture: $(uname -m); IDIOS releases support amd64 and arm64." >&2; exit 1 ;; esac
TAG=$(curl -fsSL -H 'Accept: application/vnd.github+json' "$API_URL" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)
[ -n "$TAG" ] || { echo 'Could not read the latest IDIOS release tag from GitHub.' >&2; exit 1; }
ASSET="idios-$TAG-$OS-$ARCH.tar.gz"
BASE_URL="https://github.com/$OWNER/$REPO/releases/download/$TAG"
curl -fsSL "$BASE_URL/$ASSET" -o "$TMP_DIR/$ASSET"
curl -fsSL "$BASE_URL/SHA256SUMS" -o "$TMP_DIR/SHA256SUMS"
EXPECTED=$(awk -v name="$ASSET" '$2 == name || $2 == "*" name || $2 ~ ("/" name "$") || $2 ~ ("/\\*" name "$") { print $1; exit }' "$TMP_DIR/SHA256SUMS")
[ -n "$EXPECTED" ] || { echo "No SHA-256 checksum found for $ASSET." >&2; exit 1; }
if command -v sha256sum >/dev/null 2>&1; then ACTUAL=$(sha256sum "$TMP_DIR/$ASSET" | awk '{print $1}'); elif command -v shasum >/dev/null 2>&1; then ACTUAL=$(shasum -a 256 "$TMP_DIR/$ASSET" | awk '{print $1}'); else echo 'Installer needs sha256sum (Linux) or shasum (macOS).' >&2; exit 1; fi
[ "$(printf '%s' "$EXPECTED" | tr '[:upper:]' '[:lower:]')" = "$(printf '%s' "$ACTUAL" | tr '[:upper:]' '[:lower:]')" ] || { echo 'IDIOS checksum verification failed; nothing was installed.' >&2; exit 1; }
mkdir -p "$TMP_DIR/unpacked" "$INSTALL_DIR"
tar -xzf "$TMP_DIR/$ASSET" -C "$TMP_DIR/unpacked"
[ -f "$TMP_DIR/unpacked/idios" ] || { echo 'Release archive is missing idios; nothing was installed.' >&2; exit 1; }
install -m 0755 "$TMP_DIR/unpacked/idios" "$INSTALL_DIR/.idios.new"
BACKUP="$INSTALL_DIR/.idios.backup.$$"
HAD_OLD=0
if [ -e "$INSTALL_DIR/idios" ]; then mv "$INSTALL_DIR/idios" "$BACKUP" || { echo 'Could not back up the installed idios executable.' >&2; exit 1; }; HAD_OLD=1; fi
if ! mv "$INSTALL_DIR/.idios.new" "$INSTALL_DIR/idios"; then [ "$HAD_OLD" -eq 0 ] || mv "$BACKUP" "$INSTALL_DIR/idios"; echo 'Could not replace idios; previous version restored.' >&2; exit 1; fi
rm -f "$BACKUP" "$INSTALL_DIR/idios-tui" "$INSTALL_DIR/idios-tui.exe" "$INSTALL_DIR/idios-tui-demo" "$INSTALL_DIR/idios-tui-demo.exe"

if [ "$OS" = linux ]; then
  DATA_HOME=\${XDG_DATA_HOME:-"$HOME/.local/share"}
  APPLICATIONS_DIR="$DATA_HOME/applications"
  ICON_DIR="$DATA_HOME/icons/hicolor/scalable/apps"
  mkdir -p "$APPLICATIONS_DIR" "$ICON_DIR"
  if [ -f "$TMP_DIR/unpacked/idios.svg" ]; then
    install -m 0644 "$TMP_DIR/unpacked/idios.svg" "$ICON_DIR/idios.svg"
  fi
  cat > "$APPLICATIONS_DIR/idios.desktop" <<DESKTOP
[Desktop Entry]
Version=1.0
Type=Application
Name=IDIOS
GenericName=Local Workspace
Comment=Start the IDIOS local server and open the workspace
Exec="$INSTALL_DIR/idios" serve --open
Icon=idios
Terminal=false
StartupNotify=false
Categories=Development;Utility;
Keywords=Workspace;Projects;Planning;Reading;
DESKTOP
  chmod 0755 "$APPLICATIONS_DIR/idios.desktop"
  DESKTOP_DIR=""
  if command -v xdg-user-dir >/dev/null 2>&1; then DESKTOP_DIR=$(xdg-user-dir DESKTOP 2>/dev/null || true); fi
  if [ -z "$DESKTOP_DIR" ]; then DESKTOP_DIR="$HOME/Desktop"; fi
  if [ -d "$DESKTOP_DIR" ]; then
    cp "$APPLICATIONS_DIR/idios.desktop" "$DESKTOP_DIR/IDIOS.desktop"
    chmod 0755 "$DESKTOP_DIR/IDIOS.desktop"
    if command -v gio >/dev/null 2>&1; then gio set "$DESKTOP_DIR/IDIOS.desktop" metadata::trusted true 2>/dev/null || true; fi
  fi
elif [ "$OS" = darwin ] && [ -d "$TMP_DIR/unpacked/IDIOS.app" ]; then
  APP_DIR="$HOME/Applications"
  mkdir -p "$APP_DIR"
  APP_TARGET="$APP_DIR/IDIOS.app"
  APP_BACKUP="$APP_DIR/.IDIOS.app.backup.$"
  if [ -d "$APP_TARGET" ]; then mv "$APP_TARGET" "$APP_BACKUP"; fi
  if ! cp -R "$TMP_DIR/unpacked/IDIOS.app" "$APP_TARGET"; then
    [ ! -d "$APP_BACKUP" ] || mv "$APP_BACKUP" "$APP_TARGET"
    echo "Could not install the IDIOS.app launcher; the previous app was restored." >&2
    exit 1
  fi
  rm -rf "$APP_BACKUP"
fi
if [ "$INSTALL_DIR" = "$HOME/.local/bin" ]; then
  for profile in "$HOME/.profile" "$HOME/.zprofile"; do
    [ -f "$profile" ] && grep -Fq '$HOME/.local/bin' "$profile" && continue
    printf '\n# IDIOS user-local executable\nexport PATH="$HOME/.local/bin:$PATH"\n' >> "$profile"
  done
fi
printf 'Installed IDIOS %s to %s\n' "${TAG#v}" "$INSTALL_DIR"
if [ "$OS" = darwin ] && [ -d "$HOME/Applications/IDIOS.app" ]; then
  printf 'App launcher: %s/Applications/IDIOS.app\n' "$HOME"
fi
case ":$PATH:" in *":$INSTALL_DIR:"*) printf 'Run: idios serve --open\n' ;; *) printf 'Add %s to PATH, then run: idios serve --open\n' "$INSTALL_DIR" ;; esac
