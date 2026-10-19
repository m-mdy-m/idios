# IDIOS Engineering Guide

IDIOS is a local-first personal work and knowledge graph. The current Engine is written in Go. It
models Repositories, Issues/Sub-issues, Projects, Concepts, typed relations, PR review records,
CompletionRecords, Labels, Milestones and a visible graph. V1 is a logical workflow system, not Git.

## Non-negotiable architecture

```text
cmd/idios ───────────────> internal/cli ───────────┐
                                                   ├──> internal/application ──> internal/domain
cmd/idios-tui ───────┐                             │
                     ├──> internal/tuilaunch ─> tui│
cmd/idios-tui-demo ──┘                             v
                                            internal/ports
                                                   ^
                                                   │
                                        internal/storage/sqlite
```

- `domain` is pure Go: no SQL, terminal I/O, CLI flags, logging or UI libraries.
- `application` owns workflow rules and coordinates use cases.
- `ports` defines the persistence boundary.
- `storage/sqlite` owns SQL, connection settings, migration and transactions. `connection.go` is the
  only location allowed to call `sql.Open`; use `database/sql` with `mattn/go-sqlite3` directly.
- `cli` parses commands and routes output; `tuilaunch` selects the clean or isolated demo SQLite database, and `tui` renders/navigates. Neither adapter duplicates
  workflow validation or writes SQL. CLI and TUI must never import one another.
- `render` owns CLI human/JSON output. `logger` owns optional diagnostics on stderr.

Prefer focused files grouped by domain area. Do not add a custom driver, generic repository
framework, event-sourcing platform, DI framework, service daemon or networking layer unless a
real requirement justifies it.

## Workflow invariants

1. New Repositories begin with the default Branch `main`.
2. A PR is a review and acceptance record. Accepting it is not a Git merge and writes no files.
3. An Issue reaches `done` only when its PR is accepted; the CompletionRecord, Issue transition, PR
   acceptance and associated activity events must commit atomically.
4. Only explicit `depends_on` relations block work. Concept/semantic relations do not.
5. Cross-Repository prerequisites are allowed; parent/Sub-issue relationships stay in one
   Repository. Dependency and parent cycles are rejected.
6. A parent Issue cannot start or be accepted while active Sub-issues remain unfinished.
7. Application validation should fail early with a readable error; the storage transaction must
   re-check important mutable state to protect against concurrent callers.
8. V1 supports `main` only. Do not fake Git hashes, file snapshots, merge commits, branch switching
   or non-main merge behavior.

## Storage and migrations

- Use `s.run(ctx, ...)` for coupled writes that must be atomic.
- Keep SQL in the appropriate file under `internal/storage/sqlite`.
- A schema change adds a new `migrations/NNNN_description.sql` file. Shipped migrations are
  immutable. Never instruct a user to delete their database to update the schema.
- Classify `go-sqlite3` errors by typed result codes, not error-string matching.
- Enable and test SQLite foreign keys, busy timeout, path escaping, transaction rollback and
  read-only legacy import.
- The legacy importer must never modify its source database.

## TUI and graph

- TUI is a presentation adapter over Application; graph rendering only displays the graph returned
  by Application/Storage. It must not infer relations or apply blocker policy.
- Keep persistent navigation and common button actions visible, empty states helpful and references
  consistent (`CPU#4` for Issues, `CPU!2` for PRs).
- Respect `NO_COLOR`, `TERM=dumb`, and `--no-color`. The full-screen TUI intentionally requires an interactive TTY; tests should exercise the Bubble Tea model directly without starting a terminal session.
- Do not convey `BLOCKED` by color alone. Always print the word.
- Graph nodes must show type/reference and status when meaningful. Edge labels remain visible so
  `depends_on` cannot be confused with semantic links.
- Sort graph nodes/edges deterministically and clip long labels to the panel width.

## Adding a feature

1. Define or update Domain types/invariants.
2. Add a small typed method to `ports.Store`.
3. Implement persistence and a migration if needed.
4. Add the Application use case and behavior tests.
5. Expose it through CLI and/or TUI using the same Application method.
6. Update graph projection if relevant and test node/edge semantics and depth.
7. Update `docs/user/CLI_REFERENCE.md`, `docs/development/TUI.md`, `docs/development/WORKFLOW.md`, `docs/development/STORAGE.md` and the JSON
   protocol if behavior/contracts change.

## Required local checks

```sh
make fmt-check
make test
make vet
make test-race
make build
make smoke
make psx-check
```

Do not require network connectivity or AI services at runtime or in tests. Dependencies are fetched
as a setup step; tests should use temporary local databases.
