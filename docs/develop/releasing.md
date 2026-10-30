# Releasing IDIOS

Releases are built from version tags. The CLI and updater version must match the tag, and the Web assets are embedded in the executable.

## Before a release

1. Update `CHANGELOG.md` with a section matching the intended version, for example `## [3.0.1] — 2026-10-30`.
2. Update the default CLI/updater version when preparing a new release line.
3. Run `make ci` or `pipeline run ci` when the shared CLI is installed.
4. Confirm that native toolchains are available for the target platforms.
5. Check installer scripts, checksums, and release notes.

## Create a tag

Use a semantic version tag:

```sh
git tag -a v3.0.1 -m "IDIOS v3.0.1"
git push origin v3.0.1
```

The release workflow builds Linux amd64/arm64, macOS Intel/Apple Silicon, and Windows amd64/arm64 artifacts. Linux releases include DEB and RPM packages; macOS includes PKG and DMG installers; Windows includes ZIP and setup installer assets.

## Verify artifacts

The release workflow runs tests/builds on each platform and smoke-checks the packaged outputs before publishing. It publishes SHA-256 checksums with the release. Verify downloads before installation.

Do not publish a release if a platform build or installer smoke test failed. Security signing/notarization may require credentials not configured by the repository; report that state clearly.
