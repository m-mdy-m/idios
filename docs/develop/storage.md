# SQLite storage and migrations

SQLite is the durable store for IDIOS workspace records. The storage package is the only layer that should contain SQL.

## Add a migration

1. Inspect the existing migration naming and runner in `internal/storage/sqlite/`.
2. Add a new numbered migration; do not edit a migration that has shipped.
3. Make the change safe for existing database files.
4. Implement or update the relevant store methods.
5. Add tests for the new schema and for reading existing data.
6. Run `go test ./internal/storage/sqlite ./internal/application` and then the full Go tests.

## Rules

- Never ask users to delete their database to apply a schema update.
- Keep SQL out of CLI, HTTP handlers, and Web feature code.
- Treat migrations as part of the public upgrade path.
- Preserve transaction boundaries for multi-record changes.
- Back up a user database before any manual migration experiment.

The default path and backup steps are documented in [Settings and data](../user/settings-and-data.md).
