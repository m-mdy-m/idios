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
    # The shared workflow provisions MSYS2 and runs both release commands in
    # the matching "msys2 {0}" shell. Do not run pacman -Syu here: a runtime
    # upgrade terminates the active MSYS2 shell.
    windows-amd64)
      [[ "${MSYSTEM:-}" == "UCRT64" ]] || {
        echo "windows-amd64 requires the MSYS2 UCRT64 shell (msystem: UCRT64)." >&2
        exit 1
      }
      command -v gcc >/dev/null 2>&1 || {
        echo "UCRT64 GCC is not on PATH; use msys2/setup-msys2@v2 with mingw-w64-ucrt-x86_64-gcc." >&2
        exit 1
      }
      export CC=gcc
      ;;
    windows-arm64)
      [[ "${MSYSTEM:-}" == "CLANGARM64" ]] || {
        echo "windows-arm64 requires the MSYS2 CLANGARM64 shell (msystem: CLANGARM64)." >&2
        exit 1
      }
      command -v clang >/dev/null 2>&1 || {
        echo "CLANGARM64 Clang is not on PATH; use msys2/setup-msys2@v2 with mingw-w64-clang-aarch64-clang." >&2
        exit 1
      }
      export CC=clang
      ;;
    *) echo "Unknown release target: $target" >&2; exit 2 ;;
  esac

  if [[ -n "${GITHUB_ENV:-}" ]]; then
    printf 'CC=%s\nCGO_ENABLED=1\n' "$CC" >> "$GITHUB_ENV"
  fi
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
