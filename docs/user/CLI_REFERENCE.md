# CLI reference

The CLI and TUI use the same Application Engine and database. Every command can accept `--db PATH` to select a database; set `IDIOS_DB` to configure a default for the process. Add `--json` for machine-readable output and use `idios help COMMAND...` to see accepted options.

## Common commands

```sh
# General help and version
go run ./cmd/idios --help
go run ./cmd/idios version

# Seed a sample workspace (once per database)
go run ./cmd/idios seed demo

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

## Workflow limitation

An Issue becomes `done` only through accepting a Pull Request and writing its Completion record. IDIOS v1 does not create actual Git commits, merge source files or operate remote pull requests.
