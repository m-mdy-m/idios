#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
# shellcheck source=common.sh
source "$ROOT/scripts/release/common.sh"

target="${1:?target required}"
cd "$ROOT"
idios_prepare_web
for asset in index.html site.webmanifest sw.js idios-192.png idios-512.png; do
  [[ -f "clients/web/dist/$asset" ]] || { echo "Missing PWA asset: $asset" >&2; exit 1; }
done

case "$target" in
  pwa-web) ;;
  android-*)
    command -v java >/dev/null 2>&1 || { echo "Java is required for Android packaging." >&2; exit 1; }
    [[ -n "${ANDROID_HOME:-}${ANDROID_SDK_ROOT:-}" ]] || { echo "Android SDK is missing (ANDROID_HOME / ANDROID_SDK_ROOT)." >&2; exit 1; }
    ;;
  ios-*)
    [[ "$(uname -s)" == "Darwin" ]] || { echo "iOS verification requires macOS." >&2; exit 1; }
    command -v xcodebuild >/dev/null 2>&1 || { echo "Xcode is required for iOS packaging." >&2; exit 1; }
    xcodebuild -version
    ;;
  *) echo "Unsupported mobile verification target: $target" >&2; exit 2 ;;
esac

echo "PASS [$target] PWA assets and the target toolchain are available."
