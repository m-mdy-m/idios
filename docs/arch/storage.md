# Storage and data ownership

IDIOS uses SQLite for durable workspace records. The database is shared by the CLI and the local Web server when both use the same database path.

## Ownership

- `internal/domain/` defines the business records and relationships.
- `internal/ports/` defines what application services need from storage.
- `internal/storage/sqlite/` implements those ports with SQL and migrations.
- Application services validate and coordinate writes before storage is called.

The CLI and HTTP handlers must not issue SQL directly.

## Migrations

Database schema changes are added as new, numbered migrations in the SQLite storage package. Do not edit a migration that has already shipped. Do not tell users to delete their database to apply a schema change; the application runs migrations when it opens the store.

A migration should be:

1. Small and focused on one schema change.
2. Safe for existing rows and supported old databases.
3. Covered by storage or application tests.
4. Reversible when the migration framework and change allow it; otherwise explain the compatibility decision.

## Backups

Stop the local service before copying the database file. Keep the database and configuration backup private; they may contain notes, provider settings, and personal work records.

See [Settings and data](../user/settings-and-data.md) for file locations.
