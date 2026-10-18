# Extending IDIOS safely

The standard path for a feature is **Domain → Store port → SQLite adapter → Application use case →
CLI/TUI adapter → tests and docs**. This keeps features modular and prevents UI-specific behavior
from becoming part of the Engine.

## 1. Define the behavior

- Add a type or invariant to `internal/domain` only when the model needs one.
- Put workflow validation and coordination in `internal/application`.
- Keep `internal/ports/store.go` as the storage boundary, using typed arguments/results.
- Do not import UI, terminal, SQL or driver types into Domain/Application contracts.

## 2. Persist it

- Add SQL to the relevant focused file in `internal/storage/sqlite` (for example `issues.go`,
  `knowledge.go`, `workflow.go` or `labels.go`).
- If schema changes are needed, add a new `internal/storage/sqlite/migrations/NNNN_description.sql`
  migration. Shipped migrations are immutable.
- `connection.go` is the only place allowed to call `sql.Open`; use `database/sql` directly.
- `query.go` provides package-local row mapping, query and transaction helpers. Do not create a
  second driver/pool abstraction or pass `*sql.DB` into Application.
- Keep coupled writes in `s.run(...)`. Classify driver failures in `values.go` using typed
  `sqlite3.Error` result codes rather than matching error text.
- For read-only legacy import, use `openDatabase(path, true)` and never write to the source DB.

Read [STORAGE.md](STORAGE.md) for connection options, DSN construction, transactions and migration
invariants.

## 3. Add application tests

Application tests should prove the user-visible rule (blocked Issue cannot start, parent waits for
active children, one accepted PR creates one CompletionRecord). Storage tests should verify SQL
invariants, transaction rollback, foreign keys and migration behavior. Do not rely only on CLI/TUI
tests to cover core rules.

## 4. Expose it through adapters

- Add commands in `internal/cli/registry.go`, flags in the existing declarative flag structures,
  and machine/human output through `internal/render`.
- Add TUI menu actions/screens in `internal/tui`. The TUI can format and navigate; it cannot repeat
  validation, issue SQL or invoke the CLI process.
- If a GUI might use the feature, consider whether the versioned JSON protocol needs a new field or
  stable command shape and add regression tests.

## 5. Add it to the graph deliberately

Classify each relation before implementing it:

- **Structural**: ownership and parent/child invariants.
- **Workflow**: `depends_on` or `blocks`; changes readiness and requires cycle detection.
- **Semantic**: `uses_concept`, `related_to`, `implements`, etc.; informational unless explicitly
  defined otherwise.

Only explicit workflow dependencies may block work. Add nodes/edges in `graph.go`, but keep cycle
checking, blocker computation and traversal decisions in Application/Storage, not in the renderer.

## 6. Preserve future Branch compatibility without implementing Git

V1 creates `main` and records accepted work through CompletionRecords. Do not add fake Git hashes,
file snapshots or merge behavior to complete an Issue. When non-main branches are designed, first
define what a branch contains and how PRs compare/accept it; then add separate use cases and tests.
