# CLI reference

The CLI is an adapter over the same Go application used by the Web server.

## Global usage

```text
idios [--db PATH] [--json] [--verbose] COMMAND [FLAGS]
idios help COMMAND...
```

Global options:

- `--db PATH`: select the SQLite workspace database.
- `--json`: output JSON for scripts.
- `--verbose`: enable verbose logs.

## Common commands

```sh
idios version
idios protocol
idios repo create --name "Example"
idios repo list
idios issue create --repo Example --title "First task"
idios issue list --repo Example
idios issue start --id Example#1
idios pr create --issue Example#1 --description "Work completed"
idios pr accept --id Example!1 --summary "Task finished"
idios project create --name "Learning project"
idios project add-issue --project "Learning project" --issue Example#1
```

## Reference syntax

- Issue: `Repository#number` (example `Example#1`).
- IDIOS Pull Request: `Repository!number` (example `Example!1`).

## Command groups

Use `idios help` for the list supported by the current executable. Current groups include repository, project, issue, pull request, concept, relation, graph, branch, completion, label, milestone, activity, color, seed, version, and protocol operations.

Use `idios help issue create` to see the exact flags for one operation. Notes, Shelf, scheduled jobs, and other visual workflows are managed through the Web UI.

## Web service commands

```sh
idios web [--no-open]
idios serve [--addr 127.0.0.1:43170] [--db PATH] [--open] [--no-open]
idios server start|stop|status|install|uninstall
```

The server binds to loopback by default. See [Settings and data](../user/settings-and-data.md).
