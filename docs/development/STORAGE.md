# Database and migrations

IDIOS stores workspace records in SQLite. The default database is `~/.idios/idios-engine.sqlite3`; set `IDIOS_DB` or use `--db PATH` when you need another file. The normal terminal interface does not add sample data automatically.

## Code locations

- `internal/ports/store.go` defines the storage operations the application uses.
- `internal/storage/sqlite/` implements those operations.
- `internal/storage/sqlite/migrations/` contains numbered SQL schema migrations.
- Tests for storage live beside the implementation files.

## Changing the schema

1. Add the next numbered SQL migration in `internal/storage/sqlite/migrations/`.
2. Register or include it using the existing migration mechanism.
3. Test a fresh database and, when relevant, a database created by the previous schema version.
4. Test failure and rollback behavior for multi-step changes.
5. Never tell users to delete their database to apply a schema update. Migrations should update an existing database safely.

Use the migration and store helpers that are already in the project. Do not open a second independent database path for one feature or let UI code write SQL directly.

## Tests and personal data

The Go tests should use temporary database files. Keep your normal workspace database out of tests and pull requests. If you need a test fixture, create it in a temporary directory and remove it when the test ends.

For a user backup, close IDIOS and stop its background service before copying the database file. The config file is separate from the database and may need its own backup.
