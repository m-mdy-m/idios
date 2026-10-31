#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
# shellcheck source=common.sh
source "$ROOT/scripts/release/common.sh"

target="${1:?target required}"
release_tag="${PIPELINE_RELEASE_TAG:-${GITHUB_REF_NAME:-v${IDIOS_VERSION:-3.2.2}}}"
mobile_root="$ROOT/clients/mobile"
output_dir="$ROOT/dist/release"
mkdir -p "$output_dir"

case "$target" in
  android-armeabi-v7a|android-arm64-v8a|android-x86|android-x86_64|ios-arm64|ios-simulator-arm64|ios-simulator-x86_64) ;;
  *) echo "Unsupported mobile release target: $target" >&2; exit 2 ;;
esac

: "${IDIOS_MOBILE_SERVER_URL:?Set IDIOS_MOBILE_SERVER_URL to an HTTPS IDIOS endpoint protected by deployment authentication.}"
python - "$IDIOS_MOBILE_SERVER_URL" <<'PY'
import sys
from urllib.parse import urlsplit
value = urlsplit(sys.argv[1])
if value.scheme != "https" or not value.hostname or value.username or value.password or value.query or value.fragment:
    raise SystemExit("IDIOS_MOBILE_SERVER_URL must be HTTPS and must not contain credentials, query parameters, or a fragment.")
PY

idios_prepare_web
command -v npm >/dev/null 2>&1 || { echo "npm is required for the pinned Capacitor toolchain." >&2; exit 1; }
(cd "$mobile_root" && npm install --no-package-lock --no-audit --no-fund)

case "$target" in
  android-*)
    command -v java >/dev/null 2>&1 || { echo "A Java JDK is required for Android builds." >&2; exit 1; }
    [[ -n "${ANDROID_HOME:-}${ANDROID_SDK_ROOT:-}" ]] || { echo "ANDROID_HOME or ANDROID_SDK_ROOT must point to an installed Android SDK." >&2; exit 1; }
    (
      cd "$mobile_root"
      [[ -d android ]] || npx cap add android
      npx cap sync android
    )
    gradle_file="$mobile_root/android/app/build.gradle"
    [[ -f "$gradle_file" ]] || { echo "Capacitor Android Gradle project was not generated." >&2; exit 1; }
    grep -Fq "gradle-abi-splits.gradle" "$gradle_file" || printf '\napply from: rootProject.file("../gradle-abi-splits.gradle")\n' >> "$gradle_file"
    export IDIOS_ANDROID_ABI="${target#android-}"
    (
      cd "$mobile_root/android"
      ./gradlew --no-daemon --stacktrace :app:assembleDebug
    )
    apk="$(find "$mobile_root/android/app/build/outputs/apk/debug" -maxdepth 1 -type f -name "*${IDIOS_ANDROID_ABI}*debug.apk" -print -quit)"
    [[ -n "$apk" && -s "$apk" ]] || { echo "Gradle did not produce a debug APK for ABI $IDIOS_ANDROID_ABI." >&2; exit 1; }
    cp "$apk" "$output_dir/idios-$release_tag-$target-debug.apk"
    ;;
  ios-*)
    [[ "$(uname -s)" == "Darwin" ]] || { echo "iOS builds require a macOS runner with Xcode." >&2; exit 1; }
    command -v xcodebuild >/dev/null 2>&1 || { echo "Xcode command-line tools are required." >&2; exit 1; }
    (
      cd "$mobile_root"
      [[ -d ios ]] || npx cap add ios
      npx cap sync ios
    )
    case "$target" in
      ios-arm64) sdk="iphoneos"; arch="arm64"; destination="generic/platform=iOS" ;;
      ios-simulator-arm64) sdk="iphonesimulator"; arch="arm64"; destination="generic/platform=iOS Simulator" ;;
      ios-simulator-x86_64) sdk="iphonesimulator"; arch="x86_64"; destination="generic/platform=iOS Simulator" ;;
    esac
    derived="$ROOT/dist/mobile/$target"
    xcodebuild -project "$mobile_root/ios/App/App.xcodeproj" -scheme App -configuration Release -sdk "$sdk" -arch "$arch" -destination "$destination" -derivedDataPath "$derived" CODE_SIGNING_ALLOWED=NO build
    app="$derived/Build/Products/Release-$sdk/App.app"
    [[ -d "$app" && -f "$app/Info.plist" ]] || { echo "Xcode did not produce App.app for $target." >&2; exit 1; }
    ditto -c -k --sequesterRsrc --keepParent "$app" "$output_dir/idios-$release_tag-$target-unsigned.app.zip"
    ;;
esac

printf 'Built mobile target %s for release %s\n' "$target" "$release_tag"
find "$output_dir" -maxdepth 1 -type f -name "idios-$release_tag-$target*" -print
