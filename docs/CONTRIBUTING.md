# Contributing to IDIOS

Start with [AGENTS.md](../AGENTS.md), then read the matching feature guide in this directory.
IDIOS deliberately keeps the Engine small and modular. A feature belongs in the layer that owns
its behavior; please do not move workflow logic into the interface that happens to trigger it.

## Requirements

- Go 1.25 or later.
- A C compiler on `PATH` (`gcc` or `clang`) because `github.com/mattn/go-sqlite3` uses CGO.
  SQLite is bundled by the module; a separate system SQLite library is not required.
- `make` for the recommended workflow on Linux/macOS. On Windows, use `scripts/build.ps1` and run
  Go commands directly if GNU Make is not available.
- Optional: PSX v3 for the repository shape policy in `psx.yml`.

## Before opening a PR

```sh
go mod download
make fmt-check
make test
make vet
make test-race
make build
make smoke
make psx-check  # optional locally; CI installs PSX and requires it
```

For a quick local gate use `make check`. `make ci` runs the full local gate, including a build and
end-to-end smoke test. On Windows without Make, run `./scripts/build.ps1`, `go test ./...`,
`go test -race ./...`, and `go vet ./...`.

## Dependency direction

```text
cmd/idios ──────> internal/cli ──────┐
                                     ├──> internal/application ──> internal/domain
cmd/idios-tui ──> internal/tui ──────┘              │
                                                    v
                                             internal/ports
                                                    ^
                                                    │
                                         internal/storage/sqlite
```

- **Domain** (`internal/domain`) owns entities, statuses, IDs and domain errors. No SQL, terminal
  input/output, CLI flags or presentation library imports.
- **Application** (`internal/application`) owns workflow decisions and coordinates use cases. It
  validates status transitions, computes blockers and rejects invalid/cyclic relationships.
- **Ports** (`internal/ports`) define the persistence contract. Do not let use cases depend on SQL
  tables or SQLite-specific types.
- **Storage** (`internal/storage/sqlite`) owns connections, migrations, transactions and queries.
  `connection.go` is the only place that creates a `*sql.DB`; `query.go` offers small local query
  helpers. Do not add another driver wrapper, pool, ORM or generic Repository framework without a
  demonstrated need.
- **CLI** (`internal/cli`) owns parsing/help/response routing; it must not contain domain rules.
- **TUI** (`internal/tui`) owns the full-screen event loop, clickable controls, navigation, forms, layout and graph rendering; it calls the same
  Application methods as CLI. It must not issue SQL, shell out to `idios`, or import `internal/cli`.
- **Render/logging**: CLI output formatting belongs in `internal/render`; diagnostic logs belong in
  `internal/logger` and must go to stderr.
- **Composition roots** (`cmd/idios`, `cmd/idios-tui`) wire adapters to the Engine.

## Adding a feature

1. Define the Domain entity, value type, status or invariant if required.
2. Add the smallest typed method needed to `ports.Store`.
3. Add an implementation in the appropriate focused file under `internal/storage/sqlite`. If the
   schema changes, add a new `migrations/NNNN_description.sql` file; never delete or recreate a
   user's database to migrate it.
4. Keep coupled writes in one transaction. Recheck race-sensitive state inside the transaction.
5. Add Application tests for behavior, plus Storage tests for transactions, constraints, lookups and
   rollback. Test migrations on a new database and, where relevant, an older schema.
6. Expose the use case through CLI and/or TUI. Both adapters must call the same Application method.
7. Extend graph projection only if the entity/relation belongs in graph navigation. Test edge types,
   references, ordering and neighborhood depth.
8. Update `docs/COMMANDS.md`, `docs/TUI.md`, `docs/WORKFLOW.md`, `docs/STORAGE.md` and
   `docs/JSON_PROTOCOL.md` when the public behavior or contract changes.

## Workflow invariants

- Only an accepted PR can mark an Issue `done` and create its CompletionRecord.
- Do not infer blockers from a generic graph relationship. Only explicit `depends_on` edges block.
- Dependencies and parent/Sub-issue structures must remain acyclic.
- The parent Issue and its active Sub-issues follow the documented lifecycle.
- PR acceptance, CompletionRecord creation, Issue completion and related activity events must be
  atomic.
- V1 completion history is logical work history, not a Git commit or a file merge.

## TUI and graph quality

- Keep menus scannable and make navigation keys visible on every section.
- Respect `NO_COLOR`, `TERM=dumb`, and `--no-color` for palette choices. The full-screen TUI requires a real interactive TTY; unit tests should exercise the Bubble Tea model/rendering directly and must not launch an interactive terminal session.
- Never communicate `BLOCKED` by color alone. Keep the word visible.
- A node needs a type, reference and status where relevant. Relationship names remain visible.
- Sort graph nodes and edges deterministically. Clip long labels to the panel width instead of
  corrupting borders or wrapping relationship labels unexpectedly.
- Do not add graph traversal/business rules to the renderer; it only displays the graph returned by
  Application.

## Adding a CLI command

Add one command definition in `internal/cli/registry.go` and the associated Application use case.
The generated help is guarded by tests; update tests whenever public flags or output fields change.


The TUI uses Bubble Tea v2 and Lip Gloss v2 only inside `internal/tui`; keep these dependencies out of Domain, Application, Ports and Storage.
