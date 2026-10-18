# IDIOS Engine 1.0.0 (Go)

**IDIOS is a local-first personal work and knowledge graph.** It brings repositories, issues, cross-repository prerequisites, projects, concepts, labels, milestones, pull-request reviews and a visible graph into one lightweight application.

Version 1.0.0 is the first stable Go Engine release, implemented from the architecture agreed for IDIOS. The previous Python/SQLite application is not modified by this source tree. Its task records can be imported into a separate Engine database.

## V1 scope

- Independent Go packages for domain rules, application use cases, storage, CLI and TUI.
- Local SQLite storage, schema versioning, transactions and activity events.
- One `main` branch per Repository. The Branch model has room for future base/source branch relationships, but branch creation, checkout and merging are intentionally not enabled yet.
- Issues and Sub-issues, cross-repository `depends_on` prerequisites, cycle protection, and readiness/blocker status.
- A PR is required to finish an Issue. The PR contains the review/explanation; accepting it atomically creates a `CompletionRecord` on `main` and marks the Issue `done`.
- This is a workflow record model, **not Git**: no commit hashes, file snapshots, source-code diffs or real merge are created.
- Projects spanning repositories; Concepts; typed knowledge relations; global and Repository labels; Repository- and Project-scoped milestones; Repository settings; activity history.
- A full-screen, mouse-aware TUI with persistent GitHub-inspired tabs, dashboard cards, clickable forms, Repository pages and a typed Graph Explorer. The CLI has a versioned JSON contract suitable for future GUI clients.
- Read-only import of legacy IDIOS Concepts and Tasks.

## Requirements

- Go 1.25+
- A C compiler (`gcc` or `clang`) on `PATH`. SQLite is bundled by
  `github.com/mattn/go-sqlite3`, so no system SQLite library or headers are required.
- `make` is used by the examples below; on Windows run `go build ./cmd/idios` (or
  `./scripts/build.ps1`) and `go test ./...` directly.

## Build and test

```sh
make test
make build
./dist/idios help
./dist/idios-tui
```

`make build` writes both binaries to `dist/`; `make smoke` runs the end-to-end
cross-Repository workflow against `dist/idios`. To run checks directly:

```sh
go test ./...
go test -race ./...
go vet ./...
```

## Quick start: cross-Repository prerequisite

Create two repositories and related concepts/issues:

```sh
idios repo create --name CPU --description 'CPU and hardware learning'
idios repo create --name ARLIZ --description 'Book and chapter writing'
idios concept create --name 'Electricity fundamentals'
idios issue create --repo CPU --title 'Learn basic electricity'
idios issue create --repo ARLIZ --title 'Write ARLIZ chapter three'
idios issue concept --issue ARLIZ#1 --concept 'Electricity fundamentals'
idios issue depends --issue ARLIZ#1 --on CPU#1
```

Attempting to start the ARLIZ Issue now returns a `blocked` error. Complete the CPU prerequisite through the required PR workflow:

```sh
idios issue start --id CPU#1
idios pr create --issue CPU#1 --description 'Explain the work, what was learned, and the outcome.'
idios pr accept --id CPU!1 --summary 'Electricity fundamentals reviewed'
idios issue start --id ARLIZ#1
```

After writing the chapter, finish it through its own PR:

```sh
idios pr create --issue ARLIZ#1 --description 'Chapter contents, supporting concepts, and review notes.'
idios pr accept --id ARLIZ!1 --summary 'Chapter three drafted and reviewed'
idios completion list
```

Repository-local issue references use `Repository#number`; PR references use `Repository!number`. Stable entity IDs are also accepted by most commands.

## CLI and TUI

```sh
idios --db ./idios.sqlite3 repo list
idios --db ./idios.sqlite3 --json issue list
idios --db ./idios.sqlite3 graph --from issue:ARLIZ#1 --depth 2
idios-tui --db ./idios.sqlite3
# from source: make tui
```

The default database is `~/.idios/idios-engine.sqlite3`; `IDIOS_DB` or the global `--db PATH` can override it. Global flags can appear before or after the command. The TUI is a full-screen Bubble Tea application with clickable tabs/buttons, a Workspace sidebar, in-place forms, selectable lists and a typed Graph Explorer. It uses the same Application Core as the CLI and never invokes the CLI as a subprocess. It requires an interactive terminal; `idios-tui --no-color` renders a plain screen with no escape sequences. See [TUI guide](docs/TUI.md) for navigation, mouse support, forms and graph semantics.

Read more:

- [Architecture](docs/ARCHITECTURE.md)
- [Commands](docs/COMMANDS.md)
- [Terminal UI and Graph Explorer](docs/TUI.md)
- [SQLite storage](docs/STORAGE.md)
- [Workflow and invariants](docs/WORKFLOW.md)
- [Extending IDIOS](docs/EXTENDING.md)
- [Legacy migration](docs/MIGRATION.md)
- [JSON protocol for GUI clients](docs/JSON_PROTOCOL.md)

## Status and limitations

This is the first Go Engine implementation, not feature parity with GitHub or the previous Python app. V1 does not include non-main Branch operations, real Git merging, auth/multi-user collaboration, sync, external Git hosting, advanced graph analytics or the future GUI. The module/port boundaries keep those as separate, incremental extensions.
