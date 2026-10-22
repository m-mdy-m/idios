# Command-line reference

The CLI is useful for scripts or for actions you would rather run from a terminal. Most interactive workspace features are easier to use in the terminal or browser interface.

## Start here

```sh
idios help                   # list available commands
idios help issue create      # show flags for one command
idios version                # print the installed version
idios protocol               # print the CLI/API contract version
```

Running `idios` with no arguments opens the terminal workspace. Running `idios web` opens the browser workspace.

## Common commands

```sh
idios repo create --name "Demo"
idios repo list

idios issue create --repo Demo --title "Read the project guide"
idios issue list --repo Demo
idios issue show --id Demo#1
idios issue start --id Demo#1

idios project create --name "Release preparation"
idios project add-issue --project "Release preparation" --issue Demo#1

idios pr create --issue Demo#1 --description "Documented the first-run workflow"
idios pr accept --id Demo!1 --summary "Guide is ready"
```

The example identifiers (`Demo#1`, `Demo!1`) mean repository `Demo`, issue 1, and pull request 1. Use the identifiers returned by IDIOS in your own workspace.

An issue becomes done when its IDIOS pull request is accepted. That operation only records completion inside IDIOS; it does not merge Git code or create a GitHub pull request.

## Other command groups

Use `idios help` to see the full list. Depending on the group, commands also cover concepts, graph relationships, labels, milestones, dependencies, activity, repository settings, and importing an older IDIOS database.

Notes and scheduled jobs are managed through the terminal and browser interfaces; they do not currently have standalone CLI command groups.

## JSON output and database selection

Add `--json` to request machine-readable output:

```sh
idios --json repo list
idios issue list --json
```

Use `--db PATH` to select a database for a command:

```sh
idios --db ./scratch.sqlite3 repo list
```

The `IDIOS_DB` environment variable can also set the default database path. See [Settings and data](SETTINGS.md).
