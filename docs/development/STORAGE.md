# SQLite storage

IDIOS uses SQLite through `database/sql` and `github.com/mattn/go-sqlite3`. The Store is the only package that owns SQL. Application and adapters depend on the typed `internal/ports.Store` contract and never receive `*sql.DB`.

## Schema and migrations

Migrations live under `internal/storage/sqlite/migrations/` and are embedded in the executable. The runner accepts contiguous four-digit filenames and applies each migration transactionally, updating `PRAGMA user_version`. Current migrations:

- `0001_initial.sql` — repositories, logical branches, Issues, dependencies, PRs/completions, Projects, Concepts/relations, labels/milestones, repository settings and activity.
- `0002_workspace_experience.sql` — app-level settings, notes, quotes, scheduled jobs/job runs, notifications, workspace profile and Important pins.

A shipped migration is immutable. Add the next sequential migration for any schema change. The runner refuses a database created by a newer application rather than silently downgrading it.

## Tables and ownership

| Data | Canonical storage |
| --- | --- |
| Repository and branch ownership | `repositories`, `branches` |
| Issue lifecycle and hierarchy | `issues`, `issue_dependencies` |
| PR acceptance and logical history | `pull_requests`, `completions` |
| Cross-repository project grouping | `projects`, `project_issues` |
| Concepts and semantic knowledge edges | `concepts`, `relations` |
| Labels and milestones | `labels`, `issue_labels`, `milestones`, `issue_milestones` |
| Audit-like timeline | `activity_events` |
| Quick capture and quote rotation | `notes`, `quotes`, `app_settings` |
| Profile and Important view | `workspace_profile`, `workspace_pins` |
| Reminders and local inbox | `scheduled_jobs`, `job_runs`, `notifications` |

Graph nodes and edges are projected from canonical records; the graph is not a second source of truth. Graph-visible Notes become nodes only when `graph_visible=1`, and their links are represented as `note_about` edges.

## Transactions and constraints

Use `s.run(...)` for related mutations that must commit or roll back together. SQLite foreign keys are enabled for every connection. Use bound parameters (`?`) for every user-provided value. Do not build predicates by concatenating user input. Unique/foreign-key/check failures are classified from typed SQLite error codes.

The demo seed is one transaction and writes a marker (`app_settings.seed.demo.v1`) at its end. If a write fails, the transaction rolls back and can be tried again. Re-running a completed seed is a no-op; it does not repeatedly append demo records.

## Connection policy

- `_foreign_keys=on`: enforce referential integrity on every pooled connection.
- `_busy_timeout=5000`: wait for normal concurrent local writes.
- `_journal_mode=WAL`: enable WAL for file-backed stores.
- `_synchronous=NORMAL`: WAL durability/performance balance.
- `_txlock=immediate`: reserve write transaction ownership up front.
- `mode=ro`: use read-only mode for legacy database import.

The DSN is constructed with `net/url` to preserve reserved filename characters. A `:memory:` database uses one pooled connection so the database is not accidentally split across independent in-memory connections.

## Testing storage changes

Use a fresh file under `t.TempDir()` and `OpenStore`; migration, connection and foreign-key policy are then exercised as a real database. Test idempotency, rollback, duplicate/conflicting records, graph projections and timing boundaries for scheduled jobs. Avoid modifying the user's default database from tests.
