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

# Launch the clean interactive workspace (no sample data is inserted)
go run ./cmd/idios-tui

# Launch a separate demo UI; its own database is seeded automatically
go run ./cmd/idios-tui-demo

# Explicitly seed whichever database is selected for the CLI (use with care)
go run ./cmd/idios --db ./idios-demo.sqlite3 seed demo
```

Or build all three entry points:

```sh
make build
./dist/idios --help
./dist/idios-tui
./dist/idios-tui-demo
```

The clean TUI database defaults to `~/.idios/idios-engine.sqlite3`; the demo TUI defaults to `~/.idios/demo/idios-demo.sqlite3`. Set `IDIOS_DB` for the standard workspace or `IDIOS_DEMO_DB` for the demo workspace. Both accept an explicit `--db PATH` override. TUI preferences are stored in `~/.idios/config.json`; set `IDIOS_CONFIG` to choose a different configuration file.

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

## Clean and demo TUI variants

IDIOS ships two entry points that share the same UI and application engine but never share a default database:

- **`idios-tui`** is the normal, clean workspace. A new database receives only the schema migrations; IDIOS does not load sample repositories, issues, notes, or quotes. Its default path is `~/.idios/idios-engine.sqlite3`, with `IDIOS_DB` or `--db PATH` available to select another workspace.
- **`idios-tui-demo`** is a demonstration build. On startup it seeds a realistic, connected sample workspace into `~/.idios/demo/idios-demo.sqlite3`. It uses `IDIOS_DEMO_DB` or `--db PATH` for an explicit database override and intentionally ignores `IDIOS_DB` when choosing its default. Seed data is atomic and idempotent.

Both variants include the Intro and quick guide, but neither has a **Load Demo Workspace** button. The demo is selected by launching the demo executable, not by mutating the normal workspace from inside the UI. `make tui` runs the clean UI and `make tui-demo` runs the demo UI. The `--check` flag initializes/checks the selected database without opening the interactive terminal; for the demo binary, that also ensures demo data is present.

The CLI command `idios seed demo` remains available for advanced users who intentionally want to seed the database selected by `IDIOS_DB` or `--db`. It can mix sample data into real work, so prefer the demo TUI or a separate database path. Existing database files are never automatically cleared or rewritten to remove older sample records.

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

Release assets are built and tested on native 64-bit runners for both x86-64 (`amd64`) and ARM64 (`arm64`). Each installation format includes all three entry points: `idios` (CLI), `idios-tui` (clean workspace with an empty database), and `idios-tui-demo` (demo workspace using its isolated sample database).

| Platform | Architectures | Installable formats | Portable format |
| --- | --- | --- | --- |
| Linux | x86-64 (`amd64`), ARM64 (`arm64`) | `.deb`, `.rpm` | `.tar.gz` |
| macOS | Intel (`amd64`), Apple Silicon (`arm64`) | `.pkg`, `.dmg` (contains the `.pkg` installer) | `.tar.gz` |
| Windows | x86-64 (`amd64`), ARM64 (`arm64`) | `-setup.exe` installer | `.zip` |

Asset names use the release tag and target architecture, for example:

- Linux: `idios-v1.0.0-linux-amd64.deb` and `idios-v1.0.0-linux-arm64.rpm`
- macOS: `idios-v1.0.0-darwin-arm64.pkg` and `idios-v1.0.0-darwin-arm64.dmg`
- Windows: `idios-v1.0.0-windows-amd64-setup.exe` and `idios-v1.0.0-windows-arm64-setup.exe`

Choose the asset that matches both your operating system and CPU architecture from the GitHub Releases page. `SHA256SUMS.txt` contains checksums for verifying all downloadable assets. The release workflow checks package contents and checksums, installs the Linux DEB and Windows setup package on native runners, and executes the included binaries against a SQLite-backed smoke workflow before and after publishing.

### Install

On Debian or Ubuntu, install the matching `.deb` package:

```sh
sudo apt install ./idios-v1.0.0-linux-amd64.deb
```

On Fedora, RHEL or another RPM-based distribution, install the matching `.rpm` package:

```sh
sudo dnf install ./idios-v1.0.0-linux-amd64.rpm
```

On macOS, open the matching `.pkg` directly, or open the `.dmg` and launch the `.pkg` included inside. The installer places the executables in `/usr/local/bin` and user guides in `/usr/local/share/doc/idios`. The current pipeline does not have Apple Developer ID signing/notarization credentials, so macOS may display a security warning for these packages.

On Windows, run the matching `-setup.exe`. It installs the three executables per-user under `%LOCALAPPDATA%\\Programs\\IDIOS` and offers Start Menu shortcuts. The `.zip` remains available for portable use without an installer. The setup executable is not Authenticode-signed by this pipeline, so Windows SmartScreen may warn about an unknown publisher.

The portable `.tar.gz` and `.zip` bundles remain available and contain all three executables, the license and core user guides. After extracting a portable bundle, run `./idios help` (PowerShell: `.\\idios.exe help`). Run `./idios-tui` for a clean workspace or `./idios-tui-demo` for the isolated sample workspace; the full-screen variants need an interactive terminal.

The SQLite driver uses CGO. Building from source requires Go 1.25+ and a supported C compiler; the prebuilt packages and portable bundles do not require a Go toolchain to run. Releases cover 64-bit x86 and ARM only; 32-bit targets are not currently provided.

## Development checks

```sh
make fmt-check
make test
make vet
make build
```

Tests use temporary SQLite databases and do not need to modify your actual IDIOS workspace. The SQLite driver uses CGO, so a C compiler is required.
