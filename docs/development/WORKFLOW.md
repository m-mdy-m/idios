# Workflow invariants

## Issue lifecycle

```text
open ──start (prerequisites clear)──> in_progress
                                         │
                                         └─ create PR with mandatory explanation
                                                   │
                                      accept ──────┘
                                         │
                                         v
                              accepted PR + Completion on main
                                         │
                                         v
                                    Issue: done
```

An Issue cannot be manually changed to `done`. Accepting a PR writes one Completion record and changes the Issue to `done` as a single Engine operation. Closing a PR without acceptance does not create a completion.

## Structural relationships vs prerequisites vs knowledge

- A parent Issue may have child Sub-issues. The parent and children must use the same Repository. An active child affects whether its parent can be completed.
- `depends_on` is a workflow edge. It can cross repositories; an unfinished prerequisite blocks starting the dependent Issue.
- Concept and other supported semantic relationships add context without blocking an Issue.
- A Note can be linked to another entity. If marked graph-visible, it appears as a node with `note_about` edge. It is not an Issue and does not itself impose workflow rules.
- Project membership groups Issues across repositories but does not change their ownership.

## Scheduler and notification behavior

Scheduled jobs are persisted in `scheduled_jobs`; each due execution records a `job_runs` row and advances `next_run_at`. When both the global notification switch and per-job `notify` are enabled, a local inbox row is created in `notifications`. The process is driven by the TUI's in-app timer and stops when the application exits; no OS scheduler or background daemon is installed.

## What acceptance does not mean

The current release is not a Git implementation. It does not create real commits, compute a diff, push branches, merge code or synchronize with GitHub. The `main` Completion history is logical domain data.


## Native platform and release verification

The CI matrix runs formatting checks, Go tests, `go vet`, builds the CLI and both TUI variants with CGO enabled,
and runs `scripts/smoke-platform.sh` on native Linux amd64/arm64, macOS amd64/arm64, and Windows
amd64/arm64 runners. CI and release builds use the current Go 1.27.x toolchain; `go.mod` declares
Go 1.25.0 as the module's minimum. Windows jobs install an architecture-matched C compiler because
the SQLite driver uses CGO.

Pushing a semantic version tag such as `v1.0.0` starts the release workflow. Each native runner builds
and smoke-tests the CLI, clean TUI and demo TUI before packaging them. Linux and macOS assets are `.tar.gz`; Windows
assets are `.zip`. Linux release bundles are built on Ubuntu 22.04 as a conservative glibc baseline.
The artifacts are named `idios-<tag>-<target>.<archive>`, for example
`idios-v1.0.0-linux-amd64.tar.gz` and `idios-v1.0.0-windows-arm64.zip`.

Before publication, a separate native runner downloads each Actions artifact, extracts it without
installing the build toolchain, and executes all three bundled programs plus the SQLite-backed smoke
workflow. Only then does the publish job attach all bundles and `SHA256SUMS.txt` to the GitHub
Release. A final verification matrix downloads the actual published assets using the GitHub CLI,
validates each checksum, extracts the bundle, and repeats the runtime smoke test on the matching
native platform. The release cannot finish successfully unless both round trips pass.

The matrix uses `ubuntu-24.04-arm` in push/PR CI, `ubuntu-22.04-arm` for Linux ARM64 release builds,
and `windows-11-arm` for Windows ARM64. These GitHub-hosted ARM64 runner labels and the matching C
toolchains must remain available to the repository. If a runner or compiler image is unavailable,
the matrix should fail explicitly rather than silently claiming ARM64 support. The release workflow
must be green before publishing `1.0.0` as stable.
