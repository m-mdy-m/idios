# Mobile and PWA

IDIOS has three delivery surfaces for phones:

- **PWA:** the existing Web client with an installable manifest and a service worker. This is the lightest option and works on Android and iPhone browsers that support home-screen web apps.
- **Android:** a Capacitor wrapper, built as ABI-specific debug APKs for `armeabi-v7a`, `arm64-v8a`, `x86`, and `x86_64`.
- **iOS:** a Capacitor wrapper built for ARM64 iPhone devices and ARM64/x86_64 iOS simulators.

## Important: the mobile client needs an IDIOS server

The native shells package the Web client; they do **not** embed the Go Engine or SQLite database. They load the configured IDIOS HTTPS endpoint. The Go server currently binds to loopback by default and deliberately rejects non-loopback bind addresses. Do not change that policy just to make a phone connect.

Deploy IDIOS behind a TLS reverse proxy or another authenticated access layer, keep the Go service on loopback behind that proxy, and configure the endpoint as `IDIOS_MOBILE_SERVER_URL`. The build rejects plain HTTP, URL credentials, query parameters, and URL fragments. The endpoint is not an authentication mechanism by itself; the deployment must protect the application and its data.

For GitHub release builds, add the repository Actions secret `IDIOS_MOBILE_SERVER_URL`. The shared Pipeline passes it to the mobile build command without placing the URL in a committed workflow file. For a local build, export the same environment variable in your shell.

## Install the PWA

Build the Web client:

~~~sh
make pwa-build
~~~

The generated archive is `dist/idios-pwa-web.zip`. Deploy its contents at the same HTTPS origin as the IDIOS API. Open that origin on the phone and use the browser's install / Add to Home Screen action.

The service worker caches the app shell and static assets. Requests under `/api/` are deliberately network-only, so the PWA can reopen its shell offline but cannot read or change workspace data until the IDIOS server is reachable.

## Build native clients

Requirements:

- Node.js 22 or newer and npm.
- Android: JDK, Android SDK, and Gradle tooling (the Capacitor Android project supplies the Gradle wrapper).
- iOS: macOS with Xcode and command-line tools.
- `IDIOS_MOBILE_SERVER_URL`: an authenticated HTTPS IDIOS deployment.

Examples:

~~~sh
export IDIOS_MOBILE_SERVER_URL="https://idios.example.com"
make mobile-build-android
~~~

On macOS:

~~~sh
export IDIOS_MOBILE_SERVER_URL="https://idios.example.com"
make mobile-build-ios
~~~

The more specific release targets are available through `scripts/release/build-mobile.sh`:

| Target | Output | Runner |
| --- | --- | --- |
| `android-armeabi-v7a` | ABI-specific debug APK | Linux + Android SDK |
| `android-arm64-v8a` | ABI-specific debug APK | Linux + Android SDK |
| `android-x86` | ABI-specific debug APK | Linux + Android SDK |
| `android-x86_64` | ABI-specific debug APK | Linux + Android SDK |
| `ios-arm64` | Unsigned iPhone `App.app` archive | macOS + Xcode |
| `ios-simulator-arm64` | Unsigned ARM64 simulator `App.app` archive | Apple Silicon macOS + Xcode |
| `ios-simulator-x86_64` | Unsigned x86_64 simulator `App.app` archive | Intel macOS + Xcode |

Android outputs are debug-signed APKs intended for development and native E2E verification; they are not Play Store release-signed packages. The iPhone output is unsigned because Apple distribution requires a developer team, signing identity, and provisioning profile. A signed IPA is not produced until those credentials and export settings are configured. Simulator bundles are also not iPhone distribution packages.

The Android ABI matrix creates one APK per ABI. The iOS device build targets ARM64; simulator builds target the selected simulator architecture. Hosted runner OS/architecture and output target architecture are separate fields in the Pipeline matrix.

## Select E2E behavior in CI

The shared Pipeline exposes two optional commands on the release workflow:

- `e2e-setup-command`: prepare the emulator/simulator/device, install test tools, or start a test service.
- `e2e-test-command`: install/launch the downloaded artifact and run the project's E2E suite.

The commands are supplied by the project owner at workflow dispatch or per matrix entry. They execute on the same matrix runner after Pipeline downloads that target's artifact. Pipeline records `e2e-setup` and `e2e-test` separately. A configured setup command without a test command is rejected.

The shared runner exports:

| Variable | Meaning |
| --- | --- |
| `PIPELINE_ARTIFACT_TEST_DIR` | Directory containing the downloaded target artifact |
| `PIPELINE_TARGET_OS` | Artifact OS, e.g. `android` or `ios-simulator` |
| `PIPELINE_TARGET_ARCH` | Artifact architecture |
| `PIPELINE_TARGET_ABI` | Android ABI or iOS device/simulator kind |
| `PIPELINE_TARGET_PLATFORM` | `mobile`, `pwa`, or another project-defined platform |

Example project commands (the project must provide the scripts and test framework):

~~~sh
bash scripts/mobile/prepare-e2e.sh
bash scripts/mobile/run-e2e.sh
~~~

Make the scripts target-aware. For example, a mobile-only E2E command should exit successfully for non-mobile targets, then choose the correct emulator from `PIPELINE_TARGET_OS` and `PIPELINE_TARGET_ABI`. Android setup can boot an AVD; iOS setup can boot a named simulator using `xcrun simctl`. Pipeline does not guess which device, test account, or framework the application needs.

Each stage result is saved under `.pipeline-artifacts/release-targets/<target>/` and uploaded as a diagnostic artifact. An E2E failure blocks release assembly and publication.
