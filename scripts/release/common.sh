#!/usr/bin/env bash
set -euo pipefail

IDIOS_RELEASE_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

idios_prepare_target() {
  local target="${1:?target required}"
  export CGO_ENABLED=1
  case "$target" in
    linux-amd64|linux-arm64)
      export CC=gcc
      command -v gcc >/dev/null 2>&1 || { sudo apt-get update && sudo apt-get install -y gcc; }
      ;;
    darwin-amd64|darwin-arm64)
      export CC=clang
      command -v clang >/dev/null 2>&1 || { echo "Xcode Command Line Tools (clang) are required." >&2; exit 1; }
      ;;
    windows-amd64|windows-arm64)
      local msys_root="C:/msys64"
      if [[ ! -f "$msys_root/usr/bin/bash.exe" && -f "C:/tools/msys64/usr/bin/bash.exe" ]]; then
        msys_root="C:/tools/msys64"
      fi
      if [[ ! -f "$msys_root/usr/bin/bash.exe" ]]; then
        powershell.exe -NoProfile -Command "choco install msys2 --yes --no-progress --params '/InstallDir:C:\\msys64'"
      fi
      if [[ ! -f "$msys_root/usr/bin/bash.exe" && -f "C:/tools/msys64/usr/bin/bash.exe" ]]; then
        msys_root="C:/tools/msys64"
      fi
      local msys_bash="$msys_root/usr/bin/bash.exe"
      [[ -f "$msys_bash" ]] || { echo "MSYS2 was not installed at an expected path." >&2; exit 1; }
      if [[ "$target" == windows-amd64 ]]; then
        "$msys_bash" -lc 'pacman -Syu --noconfirm || true'
        "$msys_bash" -lc 'pacman -S --needed --noconfirm mingw-w64-ucrt-x86_64-gcc'
        export CC=gcc
        export PATH="$msys_root/ucrt64/bin:$PATH"
        [[ -n "${GITHUB_PATH:-}" ]] && printf '%s\n' "$msys_root/ucrt64/bin" >> "$GITHUB_PATH"
      else
        "$msys_bash" -lc 'pacman -Syu --noconfirm || true'
        "$msys_bash" -lc 'pacman -S --needed --noconfirm mingw-w64-clang-aarch64-clang'
        export CC=clang
        export PATH="$msys_root/clangarm64/bin:$PATH"
        [[ -n "${GITHUB_PATH:-}" ]] && printf '%s\n' "$msys_root/clangarm64/bin" >> "$GITHUB_PATH"
      fi
      if [[ -n "${GITHUB_ENV:-}" ]]; then
        printf 'CC=%s\nCGO_ENABLED=1\n' "$CC" >> "$GITHUB_ENV"
      fi
      ;;
    *) echo "Unknown release target: $target" >&2; exit 2 ;;
  esac
}

idios_prepare_web() {
  command -v node >/dev/null 2>&1 || { echo "Node.js 22+ is required." >&2; exit 1; }
  corepack enable
  corepack prepare pnpm@9.15.4 --activate
  (cd "$IDIOS_RELEASE_ROOT/clients/web" && pnpm install --no-frozen-lockfile && pnpm run build)
}

idios_build_binary() {
  local target="${1:?target required}"
  local version="${GITHUB_REF_NAME#v}"
  local suffix=""
  local ldflags='-s -w'
  [[ "$target" == windows-* ]] && suffix='.exe' && ldflags="$ldflags -linkmode external -extldflags=-static"
  mkdir -p "$IDIOS_RELEASE_ROOT/dist"
  (cd "$IDIOS_RELEASE_ROOT" && go build -tags=embed_web -trimpath \
    -ldflags="$ldflags -X idios/clients/cli.Version=$version -X idios/internal/updater.Version=$version" \
    -o "dist/idios${suffix}" ./cmd/idios)
}

idios_verify_target() {
  local target="${1:?target required}"
  idios_prepare_target "$target"
  cd "$IDIOS_RELEASE_ROOT"
  go mod download
  idios_prepare_web
  local unformatted
  unformatted="$(git ls-files -z '*.go' | xargs -0 gofmt -l)"
  if [[ -n "$unformatted" ]]; then
    echo "Go files need gofmt:" >&2
    printf '%s\n' "$unformatted" >&2
    exit 1
  fi
  go test ./...
  go vet ./...
  idios_build_binary "$target"
  local suffix=""
  [[ "$target" == windows-* ]] && suffix='.exe'
  IDIOS_EXPECT_VERSION="${GITHUB_REF_NAME#v}" IDIOS_BIN="dist/idios${suffix}" sh ./scripts/smoke-platform.sh
}
