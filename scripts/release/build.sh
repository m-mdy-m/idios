#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
# shellcheck source=common.sh
source "$ROOT/scripts/release/common.sh"
target="${1:?target required}"
idios_prepare_target "$target"
cd "$ROOT"
version="${GITHUB_REF_NAME#v}"
: "${GITHUB_REF_NAME:?GITHUB_REF_NAME must contain the release tag}"
idios_prepare_web
idios_build_binary "$target"
mkdir -p dist/release

case "$target" in
  linux-amd64|linux-arm64)
    mkdir -p release
    cp dist/idios README.md LICENSE release/
    cp packaging/linux/idios.desktop release/
    cp clients/web/public/brand/idios-mark.svg release/idios.svg
    tar -czf "idios-${GITHUB_REF_NAME}-${target}.tar.gz" -C release .
    export VERSION="$version"
    export TARGET_GOARCH="${target#linux-}"
    sudo apt-get update
    sudo apt-get install -y rpm
    # nfpm v2.47.0 requires Go 1.26.4. Allow the Go command to switch toolchains
    # for this packaging tool without raising IDIOS's own go.mod minimum version.
    GOTOOLCHAIN=auto go install github.com/goreleaser/nfpm/v2/cmd/nfpm@v2.47.0
    export PATH="$(go env GOPATH)/bin:$PATH"
    nfpm package --config packaging/nfpm.yaml --packager deb --target "idios-${GITHUB_REF_NAME}-${target}.deb"
    nfpm package --config packaging/nfpm.yaml --packager rpm --target "idios-${GITHUB_REF_NAME}-${target}.rpm"
    dpkg-deb --info "idios-${GITHUB_REF_NAME}-${target}.deb" >/dev/null
    rpm -qp --qf '%{NAME} %{VERSION} %{ARCH}\n' "idios-${GITHUB_REF_NAME}-${target}.rpm"
    mv "idios-${GITHUB_REF_NAME}-${target}.tar.gz" "idios-${GITHUB_REF_NAME}-${target}.deb" "idios-${GITHUB_REF_NAME}-${target}.rpm" dist/release/
    ;;
  darwin-amd64|darwin-arm64)
    mkdir -p release
    cp dist/idios README.md LICENSE release/
    bash scripts/package-macos.sh "$version" "$GITHUB_REF_NAME" "$target"
    tar -czf "idios-${GITHUB_REF_NAME}-${target}.tar.gz" -C release .
    mv "idios-${GITHUB_REF_NAME}-${target}.tar.gz" "idios-${GITHUB_REF_NAME}-${target}.pkg" "idios-${GITHUB_REF_NAME}-${target}.dmg" dist/release/
    ;;
  windows-amd64|windows-arm64)
    powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts/release/package-windows.ps1 -Target "$target" -Version "$version" -Tag "$GITHUB_REF_NAME"
    ;;
esac

# Keep the asset listing portable: macOS ships BSD find, which has no -printf.
shopt -s nullglob
release_assets=(dist/release/*)
if ((${#release_assets[@]} == 0)); then
  echo "No release artifacts were produced in dist/release." >&2
  exit 1
fi
printf '%s\n' "${release_assets[@]##*/}" | LC_ALL=C sort
