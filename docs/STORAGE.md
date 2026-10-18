# SQLite storage guide

IDIOS uses `database/sql` with [`github.com/mattn/go-sqlite3`](https://github.com/mattn/go-sqlite3) as the registered `sqlite3` driver. SQLite is the canonical local store for V1. The adapter is deliberately small and domain-oriented; it is not a generic ORM or a home-grown SQL driver.

## Responsibilities by file

| File | Responsibility |
| --- | --- |
| `connection.go` | Opens the database, creates the parent directory, builds an escaped file-URI DSN, configures the pool, applies driver options to every connection, and retries the start-up open while another process still holds the journal-mode lock. |
| `store.go` | Owns `*sql.DB`, closes/pings it, and runs migrations at startup. |
| `query.go` | Small private helpers for row queries, writes, `database/sql` transactions, not-found errors, and activity-friendly transaction providers. |
| `values.go` | Converts SQL row values to domain-friendly values, formats timestamps, and maps `sqlite3.Error` result codes to domain errors. |
| `activity.go` | Persists activity records and exposes the activity timeline. |
| `repositories.go` | Repository settings, default `main` Branch, Project membership, and their storage queries. |
| `issues.go` | Issue lifecycle, Sub-issues, blocking dependencies, and readiness checks. |
| `workflow.go` | Required PR workflow and `CompletionRecord` history. |
| `knowledge.go` | Concepts, typed semantic relations, and entity-reference resolution. |
| `labels.go` | Global/Repository-scoped labels and Repository/Project milestones. |
| `graph.go` | Read-only projection of canonical tables into the graph view. |
| `legacy.go` | Imports a supported old IDIOS database using a separate read-only database handle. |
| `migrations/` | Embedded, ordered SQL schema changes and their transaction runner. |
| `database_test.go` | Connection, DSN, pragma, migration and constraint-classification coverage. |

## Why database/sql directly

`go-sqlite3` implements Go's `database/sql/driver` interface. IDIOS should rely on `*sql.DB`, `*sql.Tx`, `QueryContext`, and `ExecContext` rather than building a second custom database-driver surface with its own result types and transaction implementation. A small private `provider` remains only to share the query patterns used by entity-specific storage methods and to allow a helper to receive either the database or the active transaction.

Rows are mapped by column name in one helper, then converted by focused `repositoryFrom`, `issueFrom`, `conceptFrom`, and similar mapping functions. All SQL still lives in the SQLite adapter. Values are bound with `?`; never concatenate user input into a SQL predicate. SQL identifiers (for example, a PRAGMA name) are internal constants only.

## Connection policy

The file URI DSN is built with `net/url`, rather than appending `?options` to a raw filesystem path. This preserves paths that contain spaces, `#`, or `?`.

| Option | Purpose |
| --- | --- |
| `_foreign_keys=on` | Ensures relational constraints are enforced on every pooled connection. |
| `_busy_timeout=5000` | Gives another local writer time to finish before SQLite returns `SQLITE_BUSY`. |
| `_journal_mode=WAL` | Lets readers proceed while a writer is active for normal file databases. |
| `_synchronous=NORMAL` | A practical local-app balance when using WAL. |
| `_txlock=immediate` | Write transactions attempt to acquire the writer reservation at transaction start, so invariants can be re-checked before writes. |
| `mode=ro` | Used for legacy-source imports; the old database is not modified. |

File databases use the standard `database/sql` pool; WAL plus the busy timeout lets several
connections work concurrently. An in-memory database (`:memory:`) is pinned to a single
connection, because every additional pooled connection would otherwise open its own empty
database. The DSN options are per connection; do not set `PRAGMA foreign_keys` on one
arbitrary pooled connection and assume the setting applies to the others.

SQLite reports `SQLITE_BUSY` immediately — it does not wait on the busy handler — while another
connection switches the journal mode. Two IDIOS processes creating the same fresh database at
the same moment can therefore race, so `connection.go` retries a contended open for a fraction
of a second before giving up.

`mattn/go-sqlite3` uses CGO and requires `CGO_ENABLED=1` plus a C compiler. The default build bundles SQLite source; a system SQLite installation is not required unless the build is deliberately configured to use external `libsqlite3`.

## Transactions and errors

Use `Store.run(ctx, func(p provider) error { ... })` for a use case that must update several rows atomically. All statements inside the callback use the transaction-bound provider. The transaction is committed only if the callback succeeds; on an error it is rolled back. Do not call `s.p()` for writes inside that callback because those statements would execute on a separate pooled connection and escape the transaction.

`values.go` classifies `sqlite3.Error` using `ExtendedCode` (unique/primary-key, foreign-key, check and not-null constraints). Avoid string matching: error text may change with the bundled SQLite version or platform. Unclassified errors are returned with their original error chain intact.

## Schema migrations

Migration files live in `internal/storage/sqlite/migrations/` and follow the pattern `NNNN_short_description.sql`, for example `0001_initial.sql`. Versions must be positive, unique, and contiguous from 1. Files are embedded in the binary, sorted by version, and applied in version order.

Every migration runs in its own SQL transaction, with `PRAGMA user_version` updated in the same transaction. The runner re-reads `user_version` after acquiring the write transaction, so simultaneous Engine startups do not apply the same migration twice. If a migration fails, its schema changes and version increment roll back together. If `user_version` is newer than the latest embedded migration, IDIOS stops rather than attempting a downgrade.

**Never edit a migration already released.** Add `0002_add_issue_field.sql` (or the next available version) for future changes. Include appropriate defaults or explicit backfill statements so an existing user's records remain valid. Add tests against both a fresh database and an upgraded fixture when the change affects existing data.

## Legacy import

`ImportLegacy` opens the source file with URI `mode=ro`. It imports selected Concepts, Tasks-as-Issues, and eligible semantic relations into the current database; it does not run migrations or writes against the old database. Keep this path separate from the primary Engine Store so that source and destination cannot be confused. Import additions to the new database should remain repeatable where practical and should carry explicit provenance when the old model did not have a modern PR/Completion record.

## Storage tests

Storage tests should cover at least:

- escaped paths and read-only URI behavior;
- connection PRAGMAs and migration version;
- typed constraint-error mapping;
- transaction rollback and atomic workflow writes;
- concurrent start-up against one fresh database;
- cross-Repository dependency behavior and prevention of dependency cycles;
- legacy import preserving the old source file.

Remember that `?` is a legal filename character on POSIX but not on Windows: a probe that needs
a reserved character on disk must pick a set the host filesystem accepts, while the DSN test
can keep `?` and check the escaping itself.
