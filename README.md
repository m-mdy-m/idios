# IDIOS

**A local-first workspace for projects, issues, ideas and the relationships between them.**

IDIOS keeps your work in a local SQLite database and gives it structure: repositories own issues, sub-issues describe task breakdown, blocking dependencies explain what must happen first, projects group work across repositories, and concepts connect reusable knowledge. The terminal interface is keyboard-first, mouse-aware, and uses the same Go application engine as the CLI.

## Start here

### Requirements

- Go **1.25 or newer**
- A C compiler and CGO enabled for the SQLite driver (`gcc`/Clang on Unix-like systems; a supported C toolchain on Windows)
- An interactive terminal for the full-screen UI

### Run IDIOS from source

```sh
# Open the command-line interface help
go run ./cmd/idios --help

# Launch the interactive terminal workspace
go run ./cmd/idios-tui

# Add a connected demo workspace to the current database
go run ./cmd/idios seed demo
```

Or build both entry points:

```sh
make build
./dist/idios --help
./dist/idios-tui
```

The default database is `~/.idios/idios-engine.sqlite3`. Set `IDIOS_DB` or pass `--db PATH` to choose a different database. The first TUI launch creates `~/.idios/config.json` with user preferences; set `IDIOS_CONFIG` to choose a different configuration file.

## What you can do

- **Overview:** edit your workspace profile and README, pin important repositories and issues, see active work, and review recent activity.
- **Repositories:** each repository has a structured README-style overview, stats, recent Issues and Pull Requests, and repository-specific graph, labels, milestones, settings and activity. New repositories use a configurable overview scaffold.
- **Issue templates:** start from `task`, `bug`, `feature` or `research` templates with detailed context, plan and acceptance criteria; edit the multiline body before creating the Issue.
- **Pull Request templates:** new PR forms include a configurable summary, implementation, verification, risk and reviewer checklist.
- **Issues and sub-issues:** break a task into smaller child issues. A child issue must share its parent's repository.
- **Blocking dependencies:** connect issues with `depends_on`; an unfinished prerequisite prevents an issue from starting, including across repositories.
- **Pull requests and completions:** explain the outcome, review and accept the logical pull request. Acceptance creates a completion record on `main` and marks the issue done.
- **Projects:** group related issues from multiple repositories around one goal.
- **Concepts and graph:** link reusable knowledge and inspect the different relationship types.
- **Notes and quotes:** capture thoughts, link notes to work or the graph, and rotate saved quotes on startup.
- **Jobs and notifications:** configure local reminders, review the notification inbox, and inspect UI logs. Scheduled reminders run while the TUI is open; they are not an OS background service.
- **Customization:** select a theme, change layout density, edit semantic color tokens, configure `REPOSITORY_TEMPLATE`, `ISSUE_TEMPLATE` and `PULL_REQUEST_TEMPLATE`, and customize keyboard shortcuts without changing Go source.

## Try the sample data

Run `go run ./cmd/idios seed demo`, or select **Load demo workspace** from the Intro, Overview or Settings. The seed is atomic and idempotent. It creates three repositories, eight issues, a parent/sub-issue relationship, blocking and non-blocking graph edges, a project spanning repositories, concepts, labels, a milestone, open and accepted pull requests, a completion, notes, quotes, jobs, a notification, pins and activity events.

Demo content is added to the selected database, not to a separate disposable database. Use `--db /path/to/demo.sqlite3` or `IDIOS_DB` to keep the sample separate from your own workspace.

## Important scope notes

IDIOS is a **local work-tracking engine**, not a Git client or remote-hosting clone. Pull-request acceptance records a logical completion; it does not create Git commits, merge source files, check out branches, or sync with GitHub/GitLab. Notifications are kept in a local inbox, and scheduled jobs are evaluated while the TUI is running.

## Documentation

- [User documentation](docs/user/README.md) — quick start, workflows, settings, shortcuts and CLI usage.
- [Repository, Issue and Pull Request templates](docs/user/ISSUE_AND_PR_TEMPLATES.md) — customize repository overview scaffolds and detailed Issue / PR forms.
- [Developer documentation](docs/development/README.md) — package boundaries, SQLite migrations, tests and extension points.
- [Changelog](CHANGELOG.md)
- [Security policy](SECURITY.md)
- [Code of conduct](CODE_OF_CONDUCT.md)

## Downloads and supported platforms

Once a version tag passes the release workflow, assets are published for the following 64-bit desktop targets:

| Platform | Architectures | Asset format |
| --- | --- | --- |
| Linux | x86-64 (`amd64`), ARM64 (`arm64`) | `.tar.gz` |
| macOS | Intel (`amd64`), Apple Silicon (`arm64`) | `.tar.gz` |
| Windows | x86-64 (`amd64`), ARM64 (`arm64`) | `.zip` |

Assets follow the naming pattern `idios-<tag>-<target>.<archive>`, such as
`idios-v1.0.0-linux-amd64.tar.gz` or `idios-v1.0.0-windows-arm64.zip`. Each bundle contains both
`idios` (CLI) and `idios-tui` (interactive terminal UI), plus the license and core user guides.
When a release is available, download the asset matching both your operating system and CPU
architecture from the GitHub Releases page. `SHA256SUMS.txt` contains checksums for verifying
integrity. The release pipeline builds and tests on native OS/architecture runners, then downloads
the published asset again on the corresponding runner and executes the included binaries and a
SQLite-backed workflow. This catches packaging and runtime-loader issues before the release
pipeline reports success.

After extraction, run `./idios help` to confirm the CLI starts. In PowerShell, use `.\idios.exe help`.
Run `./idios-tui` from an interactive terminal (PowerShell: `.\idios-tui.exe`). The binaries
are placed at the archive root, alongside the license and user guides.

The SQLite driver uses CGO. Building from source therefore requires Go 1.25+ and a supported C
compiler; the prebuilt release bundles do not require a Go toolchain to run. Releases cover 64-bit
x86 and ARM only; 32-bit targets are not currently provided.

## Development checks

```sh
make fmt-check
make test
make vet
make build
```

Tests use temporary SQLite databases and do not need to modify your actual IDIOS workspace. The SQLite driver uses CGO, so a C compiler is required.
