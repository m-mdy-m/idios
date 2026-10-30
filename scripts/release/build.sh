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
    tar -czf "idios-${GITHUB_REF_NAME}-${target}.tar.gz" -C release .
    export VERSION="$version"
    export TARGET_GOARCH="${target#linux-}"
    sudo apt-get update
    sudo apt-get install -y rpm
    go install github.com/goreleaser/nfpm/v2/cmd/nfpm@v2.47.0
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
    tar -czf "idios-${GITHUB_REF_NAME}-${target}.tar.gz" -C release .
    bash scripts/package-macos.sh "$version" "$GITHUB_REF_NAME" "$target"
    mv "idios-${GITHUB_REF_NAME}-${target}.tar.gz" "idios-${GITHUB_REF_NAME}-${target}.pkg" "idios-${GITHUB_REF_NAME}-${target}.dmg" dist/release/
    ;;
  windows-amd64|windows-arm64)
    powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts/release/package-windows.ps1 -Target "$target" -Version "$version" -Tag "$GITHUB_REF_NAME"
    ;;
esac

find dist/release -maxdepth 1 -type f -printf '%f\n' | sort
