#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
# shellcheck source=common.sh
source "$ROOT/scripts/release/common.sh"
target="${1:?target required}"
# The shared Pipeline supplies a canonical tag for both release tags and branch dry-runs.
release_tag="${PIPELINE_RELEASE_TAG:-${GITHUB_REF_NAME:-v${IDIOS_VERSION:-3.2.2}}}"
version="${release_tag#v}"
cd "$ROOT"

case "$target" in
  pwa-web)
    idios_prepare_web
    mkdir -p dist/release
    (cd clients/web/dist && zip -qr "$ROOT/dist/release/idios-$release_tag-pwa-web.zip" .)
    ;;
  android-*|ios-*)
    bash scripts/release/build-mobile.sh "$target"
    ;;
  *)
    idios_prepare_target "$target"
    idios_prepare_web
    idios_build_binary "$target"
    mkdir -p dist/release
    ;;
esac

case "$target" in
  pwa-web|android-*|ios-*)
    printf 'Built %s for %s\n' "$target" "$release_tag"
    find dist/release -maxdepth 1 -type f -name "idios-$release_tag-*" -print
    exit 0
    ;;
esac

case "$target" in
  linux-amd64|linux-arm64)
    mkdir -p release
    cp dist/idios README.md LICENSE release/
    cp packaging/linux/idios.desktop release/
    cp clients/web/public/brand/idios-mark.svg release/idios.svg
    tar -czf "idios-${release_tag}-${target}.tar.gz" -C release .
    export VERSION="$version"
    export TARGET_GOARCH="${target#linux-}"
    sudo apt-get update
    sudo apt-get install -y rpm
    # nfpm v2.47.0 requires Go 1.26.4. Allow the Go command to switch toolchains
    # for this packaging tool without raising IDIOS's own go.mod minimum version.
    GOTOOLCHAIN=auto go install github.com/goreleaser/nfpm/v2/cmd/nfpm@v2.47.0
    export PATH="$(go env GOPATH)/bin:$PATH"
    nfpm package --config packaging/nfpm.yaml --packager deb --target "idios-${release_tag}-${target}.deb"
    nfpm package --config packaging/nfpm.yaml --packager rpm --target "idios-${release_tag}-${target}.rpm"
    dpkg-deb --info "idios-${release_tag}-${target}.deb" >/dev/null
    rpm -qp --qf '%{NAME} %{VERSION} %{ARCH}\n' "idios-${release_tag}-${target}.rpm"
    mv "idios-${release_tag}-${target}.tar.gz" "idios-${release_tag}-${target}.deb" "idios-${release_tag}-${target}.rpm" dist/release/
    ;;
  darwin-amd64|darwin-arm64)
    mkdir -p release
    cp dist/idios README.md LICENSE release/
    bash scripts/package-macos.sh "$version" "$release_tag" "$target"
    tar -czf "idios-${release_tag}-${target}.tar.gz" -C release .
    mv "idios-${release_tag}-${target}.tar.gz" "idios-${release_tag}-${target}.pkg" "idios-${release_tag}-${target}.dmg" dist/release/
    ;;
  windows-amd64|windows-arm64)
    powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts/release/package-windows.ps1 -Target "$target" -Version "$version" -Tag "$release_tag"
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
