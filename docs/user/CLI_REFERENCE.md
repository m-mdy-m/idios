# CLI reference

The CLI and both TUI variants use the same Application Engine but have different default database paths. CLI commands and the clean TUI use `IDIOS_DB`; the demo TUI uses `IDIOS_DEMO_DB` and seeds only its separate database by default. Every CLI command and both TUI binaries can accept `--db PATH` to explicitly select a database. Add `--json` for machine-readable output and use `idios help COMMAND...` to see accepted options.

## Common commands

```sh
# General help and version
go run ./cmd/idios --help
go run ./cmd/idios version

# Intentionally seed the currently selected CLI database (use a separate path)
go run ./cmd/idios --db ./idios-demo.sqlite3 seed demo

# Prefer the isolated demo TUI for a ready-to-explore sample workspace
go run ./cmd/idios-tui-demo

# Create and inspect repositories
go run ./cmd/idios repo create --name CPU --description "CPU learning notes"
go run ./cmd/idios repo list
go run ./cmd/idios repo show --repo CPU

# Work with Issues
go run ./cmd/idios issue create --repo CPU --title "Learn paging"
go run ./cmd/idios issue list --repo CPU
go run ./cmd/idios issue start --id CPU#1

# Record the outcome through a PR
go run ./cmd/idios pr create --issue CPU#1 --description "What changed and what I learned"
go run ./cmd/idios pr edit --id CPU!1 --description "Updated progress"
go run ./cmd/idios pr accept --id CPU!1 --summary "Paging fundamentals reviewed"

# Inspect connected relationships
go run ./cmd/idios graph
```

The exact flags for each command are the source of truth; run `go run ./cmd/idios help issue create` or `go run ./cmd/idios help pr accept` for detailed forms.

The `ISSUE_TEMPLATE` and `PULL_REQUEST_TEMPLATE` config sections currently prefill interactive TUI forms. CLI creation stays explicit for reproducible scripts: provide the title and description yourself. See [the template guide](ISSUE_AND_PR_TEMPLATES.md) for customization.

## References

- Issue: `Repository#12` (example: `CPU#1`)
- Pull Request: `Repository!3` (example: `CPU!1`)
- A Project can contain Issues from multiple repositories.

## Scripted use

```sh
go run ./cmd/idios --json repo list
go run ./cmd/idios --json graph
```

The `--json` output is useful for scripts and integration clients. For protocol and schema details, see [the developer protocol notes](../development/JSON_PROTOCOL.md).

The demo TUI supports `--check` for non-interactive setup verification. It applies schema migrations, ensures seed data exists and exits with an error if the database cannot be opened. The clean TUI `--check` only initializes/verifies the schema and does not insert sample records.

## Workflow limitation

An Issue becomes `done` only through accepting a Pull Request and writing its Completion record. IDIOS v1 does not create actual Git commits, merge source files or operate remote pull requests.
