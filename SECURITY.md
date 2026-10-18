# Security Policy

## Supported versions

During the Go Engine rebuild, security fixes target the currently supported Go release line.
Upgrade to the latest tagged release before reporting a potential issue. The previous Python IDIOS
application has a separate historical schema and is not upgraded in place by the Go Engine.

## Security boundaries

IDIOS is a local-first CLI/TUI application with no network listener and no required outbound
connections at runtime. The primary boundaries are the local user account, the SQLite database,
legacy databases passed to the importer, and the build dependencies.

- The default database is `~/.idios/idios-engine.sqlite3`. Keep the directory private to your user.
- The legacy importer opens its source database read-only; do not point the active Engine database
  path at the old Python application's database.
- SQLite foreign keys are enabled on every connection, and multi-step workflow changes are
  transactional.
- `github.com/mattn/go-sqlite3` uses CGO. Keep `go.mod`/`go.sum` changes reviewed and run `make
  check` and `make test-race` after dependency updates.
- CLI data goes to stdout; opt-in diagnostics go to stderr. Do not include secrets in Issue/PR text,
  Repository settings, logs or bug reports. Never upload a personal SQLite database to an issue.

## Reporting a vulnerability

Please do not report security vulnerabilities through public GitHub issues. Use GitHub's private
advisory flow: <https://github.com/m-mdy-m/idios/security/advisories/new>. Include the affected
release, reproduction steps and impact, with secrets and personal data removed.
