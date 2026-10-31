#!/usr/bin/env bash
set -euo pipefail

target="${TARGET_ID:?TARGET_ID is required}"
artifact_dir="${PIPELINE_ARTIFACT_TEST_DIR:?PIPELINE_ARTIFACT_TEST_DIR is required}"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
fail() { echo "FAIL [$target] $*" >&2; exit 1; }
pass() { echo "PASS [$target] $*"; }

case "$target" in
  pwa-web)
    archive="$(find "$artifact_dir" -maxdepth 2 -type f -name 'idios-*-pwa-web.zip' -print -quit)"
    [[ -n "$archive" ]] || fail "PWA ZIP was not downloaded."
    unzip -t "$archive" >/dev/null || fail "PWA ZIP is corrupt."
    listing="$(unzip -Z1 "$archive")"
    for required in index.html site.webmanifest sw.js idios-192.png idios-512.png; do
      grep -Fxq "$required" <<<"$listing" || fail "PWA archive is missing $required."
    done
    pass "PWA shell, manifest, service worker and install icons are present"
    ;;
  android-*)
    apk="$(find "$artifact_dir" -maxdepth 2 -type f -name "idios-*-${target}-debug.apk" -print -quit)"
    [[ -n "$apk" && -s "$apk" ]] || fail "ABI-specific APK was not downloaded."
    unzip -t "$apk" >/dev/null || fail "APK is not a valid ZIP archive."
    listing="$(unzip -Z1 "$apk")"
    grep -Fxq 'AndroidManifest.xml' <<<"$listing" || fail "APK has no AndroidManifest.xml."
    grep -Fq 'classes.dex' <<<"$listing" || fail "APK has no compiled application classes."
    pass "Android APK structure and ABI-specific artifact are valid"
    ;;
  ios-*)
    archive="$(find "$artifact_dir" -maxdepth 2 -type f -name "idios-*-${target}-unsigned.app.zip" -print -quit)"
    [[ -n "$archive" && -s "$archive" ]] || fail "iOS App.app archive was not downloaded."
    unzip -t "$archive" >/dev/null || fail "iOS app archive is corrupt."
    mkdir -p "$work/app"
    unzip -q "$archive" -d "$work/app"
    plist="$(find "$work/app" -type f -path '*/App.app/Info.plist' -print -quit)"
    [[ -n "$plist" ]] || fail "iOS archive is missing App.app/Info.plist."
    plutil -lint "$plist" >/dev/null || fail "iOS Info.plist is invalid."
    pass "iOS app bundle is structurally valid (unsigned)"
    ;;
  *) fail "unsupported mobile target: $target" ;;
esac
