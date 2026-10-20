# Contributing to IDIOS

Thanks for helping improve IDIOS. This repository contains the Go Engine and its CLI/TUI adapters;
the goal is to keep the application predictable, modular, and free of required network
services or AI. Before changing behavior, read [AGENTS.md](AGENTS.md),
[Architecture](docs/development/ARCHITECTURE.md), and the relevant feature guide.

## Toolchain

- Go 1.25+
- A C compiler on `PATH` (`gcc` or `clang`) for `github.com/mattn/go-sqlite3`
- `make` on Linux/macOS; Windows users can invoke `scripts/build.ps1` and `go` commands directly
- Optional: [PSX](https://github.com/m-mdy-m/psx) v3.0.0 for `psx check`

Runtime behavior and the test suite must not need network access. Module downloads are a setup step,
not something performed by IDIOS at runtime.

## Local quality gate

```sh
go mod download
make fmt-check
make test
make vet
make test-race
make build
make smoke
```

Or run the complete gate with `make ci`. `make psx-check` checks project shape if PSX is installed.
For Windows, use `./scripts/build.ps1`, `go test ./...`, `go vet ./...`, and `go test -race ./...`.

## Dependency direction

```text
cmd/idios ───────> internal/cli ──┐
                                  ├──> internal/application ──> internal/domain
cmd/idios-tui ───> tui ──┘                │
                                                   v
                                            internal/ports
                                                   ^
                                                   │
                                      internal/storage/sqlite
```

- **Domain** (`internal/domain`) owns entity types, identifiers, statuses, and domain errors. Keep
  it free of SQLite, output formatting, terminal interaction, and CLI-specific types.
- **Application** (`internal/application`) owns use cases and workflow decisions. It validates
  transitions, checks blockers, prevents dependency cycles, and orchestrates multi-step behavior.
- **Ports** (`internal/ports`) describe the persistence contract. New use cases should not reach into
  SQLite tables directly.
- **SQLite adapter** (`internal/storage/sqlite`) owns connection setup, SQL queries, migrations and
  persistence. It uses `database/sql` directly with `mattn/go-sqlite3`; avoid creating a second
  query/transaction wrapper or letting database handles leak into Application/Domain.
- **CLI** (`internal/cli`) owns command parsing and machine/human output routing, not business rules.
- **TUI** (`tui`) owns full-screen views, navigation, clickable controls, forms and graph presentation. It must call the
  Application Core and must not issue SQL, shell out to `idios`, or duplicate workflow validation.
- **Composition roots** (`cmd/idios`, `cmd/idios-tui`) wire storage, Application and the appropriate
  adapter. The CLI must not import TUI and the TUI must not import CLI.

## Add a feature

1. Define the entity or rule in Domain only if the model actually needs it.
2. Add a use case to Application and expose the smallest required method through the Store port.
3. Implement persistence in a focused file under `internal/storage/sqlite`. For schema changes, add
   a numbered SQL migration under `internal/storage/sqlite/migrations/`; never rely on deleting the
   user's database to update it.
4. Add Application tests for workflow behavior and Storage tests for transactions, persistence,
   constraint handling and rollback.
5. Expose the use case through CLI and/or TUI. Both interfaces should invoke the same Application
   method.
6. If the new entity or relationship belongs in the knowledge graph, extend graph projection and
   test node references, edge semantics, ordering and neighborhood behavior.
7. Update the command reference, TUI guide, workflow/architecture docs and JSON protocol if the public
   behavior or response shape changed.

## Workflow invariants

- Only an accepted PR can mark an Issue `done` and create its Completion Record.
- Do not infer blocking from a generic graph edge. Explicit `depends_on` relationships govern
  readiness; Concept links and other semantic edges are informational.
- A dependency must not create a cycle. Application validation gives readable errors, and the storage
  transaction must protect race-sensitive transitions.
- Keep all coupled writes in one SQLite transaction.
- Do not model logical work completion as a real Git commit/merge; V1 has no file snapshots or diffs.

## UI and accessibility

- Keep text concise and scannable; display references such as `ARLIZ#12` and `ARLIZ!4` consistently.
- Never hide blockers behind color alone: status chips must include readable words.
- Graph nodes need a type, a stable reference, and status when relevant. Relationship names must remain
  visible so `depends_on` cannot be mistaken for a non-blocking knowledge link.
- Respect `NO_COLOR` and `--no-color`: that mode must emit no escape sequences at all, so state has to be carried by text (`▸` for the current control, `BLOCKED` as a word). Test the Bubble Tea model and render output directly; do not require an interactive terminal in unit tests. The executable itself requires a TTY and uses the alternate screen.
- Use deterministic ordering in graph output and snapshots so tests and CLI/TUI views are predictable.

## Pull requests

Keep PRs focused. Include the behavior change, tests run, migration impact and documentation updates in
the PR description. User-facing and storage behavior changes should have regression tests. Never
include personal database files, generated binaries or secrets in a change.


The TUI uses Bubble Tea v2 and Lip Gloss v2 only inside `tui`; keep these dependencies out of Domain, Application, Ports and Storage.
