# Architecture

IDIOS is a local-first work and knowledge graph. The Go Engine owns domain rules and persistence; CLI and TUI are independent adapters. A Pull Request is currently a review/acceptance record. Accepting it records a Completion against the logical `main` branch; it does not perform a real Git merge.

## Dependency direction

```text
cmd/idios ──────> internal/cli ──────────────┐
                                             ├──> internal/application ──> internal/domain
cmd/idios-tui ──> tui (Bubble Tea adapter) ──┘              │
                                                           v
                                                    internal/ports
                                                           ^
                                                           │
                                              internal/storage/sqlite
                                                           │
                                              database/sql + go-sqlite3
```

- `cmd/idios` and `cmd/idios-tui` are composition roots. Both open the same SQLite-backed Store and create `application.Service`.
- `internal/cli` owns command parsing, flags, user help and CLI/JSON output selection.
- `tui/` owns the terminal-only presentation: view layout, navigation, forms, keyboard/mouse events, styling, built-in UI copy, diagnostics and the in-app scheduler. It is deliberately outside `internal/`, and must call Application use cases rather than execute SQL.
- `internal/application` coordinates use cases and validates workflow transitions. New rules belong here, not in a button callback.
- `internal/domain` defines entities, IDs, typed statuses and errors. It does not import UI libraries, SQLite or CLI packages.
- `internal/ports` is the persistence contract.
- `internal/storage/sqlite` implements the port. SQL, migrations, transactions, entity mapping and local scheduling persistence live here.
- `internal/config` owns CLI database path policy and the user-editable TUI configuration file. Product copy is a separate embedded resource at `tui/content/en.json`; copy is not written into user preferences.

## TUI module map

| File | Responsibility |
| --- | --- |
| `tui/model.go` | Pages, form state, shared snapshot and application lifecycle |
| `tui/update.go` | Event/update loop, keyboard routing and asynchronous messages |
| `tui/navigation.go` | Cursor, list selection, back navigation and shortcut dispatch |
| `tui/actions.go` | User intent routing and form submission through Application |
| `tui/canvas.go` | Terminal layout primitives and mouse hit targets |
| `tui/styles.go` | Semantic color tokens and theme presentation |
| `tui/view.go` | Application chrome, responsive layout and page dispatch |
| `tui/view_overview.go` | Workspace overview and existing repository/issue/project/graph pages |
| `tui/view_details.go` | Entity details, forms and in-app help |
| `tui/view_workspace.go` | Intro, profile-oriented Overview elements, notes, quotes, jobs, notifications and logs |
| `tui/content.go`, `tui/content/en.json` | Embedded user-facing prose and help copy |
| `tui/logging.go` | Local TUI logs with optional debug entries |
| `tui/scheduler.go` | In-app due-job timer and processing command |

## Persistence

```text
internal/storage/sqlite/
  store.go                 Store lifecycle and health check
  connection.go            DSN construction, SQLite pool policy
  query.go                 Row/query and transaction helpers
  values.go                Scalar conversion, timestamps and error classification
  migrations/
    migrate.go             Ordered embedded migration runner
    0001_initial.sql        Core repository/workflow/graph tables
    0002_workspace_experience.sql  Profile, pins, notes, quotes, jobs and notifications
  repositories.go          Repositories, projects, branches and repository settings
  issues.go                Issue lifecycle, Sub-issues and dependencies
  workflow.go              Pull requests and Completion records
  knowledge.go             Concepts, semantic relations and entity resolution
  graph.go                 Projection of canonical data into typed graph nodes/edges
  workspace.go             Notes, quotes, workspace profile/pins, jobs and notifications
  seed.go                  Atomic, idempotent demonstration workspace
```

The database is upgraded at startup by applying contiguous, versioned migrations. A migration already released is immutable; add `NNNN_description.sql` rather than editing an old migration. `go-sqlite3` requires CGO and a C compiler.

## Data flow

A user gesture should flow through `tui` or `internal/cli` → `internal/application` → `internal/ports.Store` → `internal/storage/sqlite`. UI callbacks may format feedback and choose a destination, but must not query SQL or encode workflow invariants. Schema/port changes require storage tests; use-case changes require Application tests; TUI changes require rendering/navigation tests; docs and help copy should be updated with visible behavior.

## Deliberate limits

The current release does not manage real Git repositories, commits, diffs, multi-branch checkouts or merge operations. Scheduled jobs are evaluated only while the TUI is running; IDIOS does not register OS scheduler tasks. The notification inbox is local storage, not a network push integration.
