# Getting started

## 1. Launch IDIOS

From a source checkout, run:

```sh
go run ./cmd/idios-tui
```

The TUI opens in an Intro screen by default. Choose **Enter Overview** to enter the workspace, **Customize IDIOS** to change preferences, or **Load demo workspace** to explore realistic connected data.

For a prebuilt version, build both apps with `make build` and launch `dist/idios-tui` (or `dist\\idios-tui.exe` on Windows).

## 2. Choose a database

The default database is `~/.idios/idios-engine.sqlite3`. Use `IDIOS_DB=/path/to/demo.sqlite3` or the CLI `--db PATH` flag to work with a different database. The app creates tables and applies schema migrations at startup.

The demo seed is written into the currently selected database. To keep it separate from real work, choose a new database path before seeding:

```sh
IDIOS_DB=./idios-demo.sqlite3 go run ./cmd/idios seed demo
IDIOS_DB=./idios-demo.sqlite3 go run ./cmd/idios-tui
```

PowerShell:

```powershell
$env:IDIOS_DB = "$PWD\\idios-demo.sqlite3"
go run ./cmd/idios seed demo
go run ./cmd/idios-tui
```

## 3. Load demo data

Run `go run ./cmd/idios seed demo` in the selected database, or choose **Load demo workspace** in the UI. Seeding is atomic and designed to be idempotent: repeating it does not add duplicate demo records.

The sample includes:

- The IDIOS, CDIN and ZINX repositories.
- A child Issue and its parent, plus `depends_on` prerequisite examples.
- A cross-repository Project with Issues from multiple repositories.
- Concepts, typed relationships, labels and a milestone.
- An open PR, an accepted PR and a completion record.
- Notes linked to an Issue, a Concept and a Repository; some Notes are visible in the graph.
- Rotating Quotes, scheduled Jobs, a Notification, important pins and recent activity.

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
