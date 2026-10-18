# Security policy

## Supported versions

The Go Engine is under active development. Security fixes are applied to the current supported
release line; users should upgrade to the latest tagged release before reporting an issue. The
legacy Python application has a separate historical release line and schema.

## Threat model

IDIOS is a local-first, single-user CLI/TUI application. It does not start a network listener or
call external services during normal operation. Its primary security boundaries are the local user
account, the SQLite database file, imported legacy data, and third-party build dependencies.

- **Local database**: the default Engine database is `~/.idios/idios-engine.sqlite3`. Its directory
  and file permissions should prevent unintended access by other local accounts. SQLite foreign keys
  are enabled for every connection and multi-step workflow writes run transactionally.
- **Legacy import**: the old database is opened read-only. Do not point the Engine at the legacy file
  as its active database; the schemas are different. See [MIGRATION.md](MIGRATION.md).
- **Dependencies**: the runtime SQLite driver is `github.com/mattn/go-sqlite3`, built with CGO. Keep
  `go.mod` and `go.sum` reviewed and run the repository's Go tests after dependency changes.
- **CLI JSON output**: stdout is intended for user/script data. Debug logging is opt-in and goes to
  stderr. Avoid writing secrets or database content into diagnostics.
- **Untrusted repositories**: if importing or inspecting an unknown database, use a copy first;
  schema migrations are applied to the active Engine database during open.

## Reporting a vulnerability

Do not open a public issue for an unpatched security vulnerability. Use GitHub's private advisory
flow: <https://github.com/m-mdy-m/idios/security/advisories/new>. Include the affected version,
reproduction steps, impact and (if known) a suggested fix. Please do not include private database
files, access tokens or other secrets in the report.

## Practical hardening

- Keep the `~/.idios` directory private to your user account.
- Back up the database before testing a migration or importing a legacy database.
- Review `go.mod`/`go.sum` changes and run `make check` plus `make test-race`.
- Do not place credentials in Repository descriptions, Issue bodies, PR descriptions or activity notes.
