# IDIOS Architecture

IDIOS is a local-first personal work and knowledge graph. The Engine owns domain rules and
persistence; CLI and TUI are independent adapters. The current release is a workflow model,
not a Git implementation: a PR is a review/acceptance record and accepting it writes a
`CompletionRecord` to the Repository's logical `main` history.

## Architectural boundaries

```text
cmd/idios ──────> internal/cli ──────┐
                                     ├──> internal/application ──> internal/domain
cmd/idios-tui ──> internal/tui ──────┘              │
                                                    v
                                             internal/ports
                                                    ^
                                                    │
                                         internal/storage/sqlite
                                                    │
                                         database/sql + go-sqlite3
```

- `cmd/idios` is the CLI composition root. It parses no workflow logic; it wires the store,
  Application service, and CLI adapter.
- `cmd/idios-tui` wires the same store and Application to the TUI. The CLI and TUI do not import
  one another.
- `internal/cli` owns command/flag parsing, help and machine/human response routing.
- `internal/tui` owns the full-screen Bubble Tea event loop, clickable controls, navigation, forms, layout, status presentation and graph rendering. It only calls Application use cases and never runs SQL or shells out to the CLI. Bubble Tea and Lip Gloss remain confined to this adapter.
- `internal/render` owns CLI response formatting. `internal/logger` owns optional diagnostics
  sent to stderr.
- `internal/application` coordinates use cases, validates user inputs and protects workflow
  invariants. Adapters must not duplicate these rules.
- `internal/domain` defines entities, status types, stable IDs and domain errors. It must not
  import SQLite, CLI or UI libraries.
- `internal/ports` is the persistence boundary between Application and adapters.
- `internal/storage/sqlite` implements `ports.Store`; it owns SQL, connection configuration,
  migrations, transactions and row/value conversion.

## Storage layout

```text
internal/storage/sqlite/
  store.go                 Store lifetime and health check
  connection.go            DSN construction, pool settings and read-only connection setup
  query.go                 Row scanning, query provider and transaction helper
  values.go                SQLite error mapping, scalar conversion and timestamp handling
  migrations/
    migrate.go             Ordered, embedded, transactional migrations
    0001_initial.sql        Initial schema
  repositories.go          Repositories, Projects, Branches and settings
  issues.go                Issue records, hierarchy and blocking dependencies
  workflow.go              Pull Requests, acceptance and CompletionRecords
  graph.go                 Graph projection and neighborhood queries
  knowledge.go             Concepts and semantic relations
  labels.go                Labels and milestones
  activity.go              Activity history
  legacy.go                Read-only import from the previous IDIOS SQLite schema
```

`connection.go` is the only place that creates a `*sql.DB`. The adapter uses Go's standard
`database/sql` API and [`github.com/mattn/go-sqlite3`](https://github.com/mattn/go-sqlite3).
There is no second custom driver/connection-pool abstraction. `query.go` is a small package-local
helper, not a public data-access framework; feature SQL stays grouped by domain area.

The DSN applies settings to every pooled connection: foreign keys and busy timeout are enabled;
file databases use WAL and normal synchronous mode; writes use an immediate transaction lock so
race-sensitive transitions can re-check state while holding the write lock. Paths are encoded as
file URIs instead of concatenating raw user paths with query parameters. A read-only mode is used
for legacy import.

## Core workflow invariants

1. A Repository starts with one default Branch named `main`.
2. V1 does not create non-main Branches or merge content. The model reserves base/source Branch
   references for a later multi-branch workflow, but no Git file snapshot or merge is implied.
3. An Issue can become `done` only when its Pull Request is accepted.
4. Accepting a PR, creating the CompletionRecord, marking its Issue done and writing related
   activity events happen atomically.
5. `depends_on` is the only graph edge type that blocks work. Concept and semantic links do not
   implicitly block an Issue.
6. Cross-Repository prerequisites are allowed. Parent/Sub-issue relationships remain within one
   Repository. Dependency and parent cycles are rejected.
7. Parent Issues cannot start or be accepted while active Sub-issues remain unfinished.
8. Application-level validation gives useful early errors; the storage transaction rechecks
   important mutable conditions to remain safe under concurrent callers.

## Graph model

Graph output is a projection over existing entities and typed edges; the renderer is not the
source of truth. The TUI and CLI use the same Application graph query. The TUI groups typed nodes
and explicit relationships into a responsive relationship map and selectable node list. It always displays the relationship type. In
particular, `depends_on` is visibly identified as blocking so it cannot be confused with a
non-blocking Concept edge.

## Extensibility

New feature flow:

1. Add a domain type/rule only when the model needs it.
2. Add the smallest method to `ports.Store`.
3. Add persistence in a focused `internal/storage/sqlite/*.go` file. Schema changes require a new
   numbered SQL migration; do not edit an existing migration after it has shipped.
4. Add an Application use case and tests.
5. Expose it via CLI, TUI, or both by calling that same use case.
6. Extend the graph projection if needed, and update CLI/TUI/workflow/storage docs and protocol
   tests when user-visible behavior changes.

Do not make Application or Domain depend on a GUI framework or a specific transport. A future
Python GUI should initially use the versioned machine-readable CLI contract rather than open the
SQLite file directly. A future long-running JSON-RPC/IPC adapter can reuse Application as well.

## Local-first and future sync

Stable entity IDs, timestamps, typed relations and explicit records help future export/import or
sync, but are not a complete multi-device consistency model. Before adding sync, define conflict
resolution, tombstones, causal ordering, identity/ownership and schema-compatibility rules. No
network service is required by V1 runtime behavior or tests.
