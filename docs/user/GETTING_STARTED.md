# Getting started

## 1. Launch IDIOS

After installation, run `idios` to open the normal interactive workspace. Commands with explicit arguments (for example, `idios version` or `idios --help`) remain scriptable CLI commands. For a source checkout, use:

```sh
# Normal workspace: starts with no sample records in a new database
go run ./cmd/idios

# Demo workspace: creates sample data in a separate database automatically
go run ./cmd/idios-tui-demo
```

The standard TUI opens in Intro and does not include a “Load Demo Workspace” button. Launch the demo executable when you want to explore sample repositories, linked issues, concepts, projects, pull requests and graph relationships. The two apps share the interface but use different default database paths.

For prebuilt development binaries, run `make build`, then launch `dist/idios` or `dist/idios-tui-demo` (Windows: `dist\idios.exe` or `dist\idios-tui-demo.exe`).

## 2. Choose a database

The normal workspace defaults to `~/.idios/idios-engine.sqlite3`. It honors `IDIOS_DB` or `--db PATH`. A fresh database receives schema migrations only; the standard TUI does not seed demo records.

The demo workspace defaults to `~/.idios/demo/idios-demo.sqlite3`. Set `IDIOS_DEMO_DB` to override its default. The demo binary intentionally ignores `IDIOS_DB`, so setting the normal workspace path does not redirect its default seed into your real workspace. You can also pass `--db PATH` directly to either executable for an explicit override; do not point both at the same file unless you deliberately want to share one workspace.

Both binaries support `--check` for automation and diagnostics. It initializes and checks the selected database without requiring an interactive terminal. The demo binary also ensures that sample data exists:

```sh
./dist/idios-tui --check --db ./clean-workspace.sqlite3
IDIOS_DEMO_DB=./demo-workspace.sqlite3 ./dist/idios-tui-demo --check
```

PowerShell:

```powershell
.\dist\idios-tui.exe --check --db .\clean-workspace.sqlite3
$env:IDIOS_DEMO_DB = "$PWD\demo-workspace.sqlite3"
.\dist\idios-tui-demo.exe --check
```

## 3. Explore demo data

Launch `idios-tui-demo` (or run `go run ./cmd/idios-tui-demo`). Its seed is atomic and idempotent. The sample includes:

- The IDIOS, CDIN and ZINX repositories.
- Parent and child Issues, plus `depends_on` prerequisite examples.
- A cross-repository Project with Issues from multiple repositories.
- Concepts, typed relationships, labels and a milestone.
- Open and accepted Pull Requests and a Completion record.
- Notes linked to Issues, Concepts and Repositories, including graph-visible Notes.
- Rotating Quotes, scheduled Jobs, a Notification, important pins and recent Activity.

The demo database persists between launches so you can explore changes. It is separate from the normal workspace by default. If you explicitly run `idios seed demo`, the CLI seeds the database selected by `IDIOS_DB` or `--db`; use that command only when you intend to add sample data to that database. IDIOS never automatically deletes an existing database or removes sample records previously added by an older build.

## 4. Keyboard basics

| Key | Action |
| --- | --- |
| `↑` / `↓` or `k` / `j` | Move through a list |
| `Enter` / `Space` | Open the selected item or activate the focused control |
| `F6` or `Ctrl+Tab` | Focus visible buttons and form controls; use `↑` / `↓`, then `Enter` |
| `Tab` / `Shift+Tab` | Move between main destinations |
| `/` | Search or filter the current list |
| `Esc` | Back out or return to Overview |
| `?` | Open the quick guide |
| `Ctrl+N` | Open the creation hub |
| `Ctrl+Shift+N` / `Ctrl+Shift+Q` | Open Notes / Quotes |
| `Ctrl+J` / `Ctrl+L` | Open scheduled Jobs / Logs |
| `q` | Quit |

Shortcuts are configurable in `~/.idios/config.json`. The Settings screen displays the active key mapping.

## 5. First useful workflow

1. Create a **Repository** for an area of work.
2. Add an **Issue** with a clear title and expected outcome.
3. Add child Issues when you need to break the work into steps.
4. If an Issue cannot start until another one is complete, add a **blocking prerequisite** (`depends_on`).
5. Start an unblocked Issue, create a **Pull Request** with a useful explanation, then accept it to record the completion.
6. Add the Issue to a **Project** if it belongs to a larger goal. Link a **Concept** for reusable knowledge, or attach a **Note** for a thought worth keeping.

See [Workspace guide](WORKSPACE_GUIDE.md) for the distinctions between these relationships. Before using templates heavily, see [Repository, Issue and Pull Request templates](ISSUE_AND_PR_TEMPLATES.md) to customize the default briefs, acceptance criteria and review checklist.


## Download a prebuilt release

After a release is published, open the project's GitHub Releases page and select the archive that
matches your operating system and processor:

- Linux: `linux-amd64` or `linux-arm64` (`.tar.gz`)
- macOS: `darwin-amd64` for Intel or `darwin-arm64` for Apple Silicon (`.tar.gz`)
- Windows: `windows-amd64` or `windows-arm64` (`.zip`)

For user-local installation and automatic in-app updates, use `curl -fsSL https://github.com/m-mdy-m/idios/releases/latest/download/install.sh | sh` on Linux/macOS or `irm https://github.com/m-mdy-m/idios/releases/latest/download/install.ps1 | iex` on Windows. To use a portable bundle, extract it and run `./idios` (PowerShell: `.\idios.exe`) to open the normal workspace; use `./idios help` for CLI help and `./idios-tui-demo` for the isolated sample workspace. The release archive includes the CLI and the clean TUI; the demo TUI is provided for development/source use only. `SHA256SUMS.txt` is published alongside the archives for integrity checks.

## In-app upgrades

The standard TUI checks GitHub Releases on startup. When a newer stable release is found, use the small **Upgrade** control in the lower-right corner to view the version changelog and start a verified upgrade. The updater checks the release archive against `SHA256SUMS.txt`, stages the new CLI and TUI together, then relaunches the app. User-local installs are recommended for one-click upgrades; system-owned `.deb`, `.rpm`, and `.pkg` installs may require the OS package manager or administrator permission.
