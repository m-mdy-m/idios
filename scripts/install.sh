#!/usr/bin/env sh
# Install the latest IDIOS CLI and TUI into a user-writable PATH directory.
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
case "$OS" in
  linux|darwin) ;;
  *) echo "Unsupported operating system: $OS (use the Windows PowerShell installer)." >&2; exit 1 ;;
esac
case "$(uname -m)" in
  x86_64|amd64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) echo "Unsupported CPU architecture: $(uname -m); IDIOS release builds support amd64 and arm64." >&2; exit 1 ;;
esac

TAG=$(curl -fsSL -H 'Accept: application/vnd.github+json' "$API_URL" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)
[ -n "$TAG" ] || { echo 'Could not read the latest IDIOS release tag from GitHub.' >&2; exit 1; }
ASSET="idios-$TAG-$OS-$ARCH.tar.gz"
BASE_URL="https://github.com/$OWNER/$REPO/releases/download/$TAG"
curl -fsSL "$BASE_URL/$ASSET" -o "$TMP_DIR/$ASSET"
curl -fsSL "$BASE_URL/SHA256SUMS.txt" -o "$TMP_DIR/SHA256SUMS.txt"

EXPECTED=$(awk -v name="$ASSET" '$2 == name || $2 == "*" name { print $1; exit }' "$TMP_DIR/SHA256SUMS.txt")
[ -n "$EXPECTED" ] || { echo "No SHA-256 checksum found for $ASSET." >&2; exit 1; }
if command -v sha256sum >/dev/null 2>&1; then
  ACTUAL=$(sha256sum "$TMP_DIR/$ASSET" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
  ACTUAL=$(shasum -a 256 "$TMP_DIR/$ASSET" | awk '{print $1}')
else
  echo 'The installer needs sha256sum (Linux) or shasum (macOS) to verify the release.' >&2
  exit 1
fi
[ "$(printf '%s' "$EXPECTED" | tr '[:upper:]' '[:lower:]')" = "$(printf '%s' "$ACTUAL" | tr '[:upper:]' '[:lower:]')" ] || {
  echo 'IDIOS release checksum verification failed; nothing was installed.' >&2; exit 1;
}

mkdir -p "$TMP_DIR/unpacked" "$INSTALL_DIR"
tar -xzf "$TMP_DIR/$ASSET" -C "$TMP_DIR/unpacked"
[ -f "$TMP_DIR/unpacked/idios" ] && [ -f "$TMP_DIR/unpacked/idios-tui" ] || {
  echo 'Release archive is missing idios or idios-tui; nothing was installed.' >&2; exit 1;
}
# Stage on the destination filesystem, then replace both binaries as one transaction.
# Keep the previous pair until both new files have been installed successfully.
install -m 0755 "$TMP_DIR/unpacked/idios" "$INSTALL_DIR/.idios.new"
install -m 0755 "$TMP_DIR/unpacked/idios-tui" "$INSTALL_DIR/.idios-tui.new"
BACKUP_CLI="$INSTALL_DIR/.idios.backup.$$"
BACKUP_TUI="$INSTALL_DIR/.idios-tui.backup.$$"
HAD_CLI=0
HAD_TUI=0
INSTALLED_CLI=0
if [ -e "$INSTALL_DIR/idios" ]; then
  mv "$INSTALL_DIR/idios" "$BACKUP_CLI" || { echo 'Could not back up the installed idios executable.' >&2; exit 1; }
  HAD_CLI=1
fi
if [ -e "$INSTALL_DIR/idios-tui" ]; then
  if ! mv "$INSTALL_DIR/idios-tui" "$BACKUP_TUI"; then
    [ "$HAD_CLI" -eq 0 ] || mv "$BACKUP_CLI" "$INSTALL_DIR/idios"
    echo 'Could not back up the installed idios-tui executable.' >&2
    exit 1
  fi
  HAD_TUI=1
fi
if mv "$INSTALL_DIR/.idios.new" "$INSTALL_DIR/idios"; then
  INSTALLED_CLI=1
else
  [ "$HAD_CLI" -eq 0 ] || mv "$BACKUP_CLI" "$INSTALL_DIR/idios"
  [ "$HAD_TUI" -eq 0 ] || mv "$BACKUP_TUI" "$INSTALL_DIR/idios-tui"
  echo 'Could not replace idios; previous version restored.' >&2
  exit 1
fi
if ! mv "$INSTALL_DIR/.idios-tui.new" "$INSTALL_DIR/idios-tui"; then
  [ "$INSTALLED_CLI" -eq 0 ] || rm -f "$INSTALL_DIR/idios"
  [ "$HAD_CLI" -eq 0 ] || mv "$BACKUP_CLI" "$INSTALL_DIR/idios"
  [ "$HAD_TUI" -eq 0 ] || mv "$BACKUP_TUI" "$INSTALL_DIR/idios-tui"
  echo 'Could not replace idios-tui; previous version restored.' >&2
  exit 1
fi
rm -f "$BACKUP_CLI" "$BACKUP_TUI"

# Make the default per-user install available in future login shells.
if [ "$INSTALL_DIR" = "$HOME/.local/bin" ]; then
  for profile in "$HOME/.profile" "$HOME/.zprofile"; do
    if [ -f "$profile" ] && grep -Fq '$HOME/.local/bin' "$profile"; then
      continue
    fi
    printf '\n# IDIOS user-local binaries\nexport PATH="$HOME/.local/bin:$PATH"\n' >> "$profile"
  done
fi

printf 'Installed IDIOS %s to %s\n' "${TAG#v}" "$INSTALL_DIR"
case ":$PATH:" in
  *":$INSTALL_DIR:"*) printf 'Run: idios\n' ;;
  *) printf 'Add %s to PATH (or start a new login shell), then run: idios\n' "$INSTALL_DIR" ;;
esac
