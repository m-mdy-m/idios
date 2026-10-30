# Use IDIOS from the terminal

The CLI is useful for quick actions, scripts, and automation. Running `idios` without arguments prints help.

## First commands

```sh
idios help
idios help issue create
idios version
idios protocol
```

## Create work

```sh
idios repo create --name "Learning Lab"
idios issue create --repo "Learning Lab" --title "Read the architecture guide"
idios issue list --repo "Learning Lab"
idios issue start --id "Learning Lab#1"
```

A repository issue reference has the form `Repository#number`. A pull-request reference has the form `Repository!number`.

## Record completion

```sh
idios pr create --issue "Learning Lab#1" --description "Read the guide and recorded notes"
idios pr accept --id "Learning Lab!1" --summary "Architecture guide reviewed"
```

Accepting an IDIOS Pull Request creates an in-app completion record. It does not create a Git commit or a GitHub Pull Request.

## Projects

```sh
idios project create --name "IDIOS onboarding"
idios project add-issue --project "IDIOS onboarding" --issue "Learning Lab#1"
idios project issues --project "IDIOS onboarding"
```

## Server commands

```sh
idios web                   # start the local Web workspace and open the browser
idios web --no-open         # start/check the workspace without opening a browser
idios serve --no-open       # run the server in the foreground
idios server status
idios server start
idios server stop
idios server install        # register per-user autostart
idios server uninstall
```

`idios serve install|uninstall|start|stop|status` are accepted aliases for the corresponding `idios server` commands.

## Global options

- `--db PATH` selects the workspace database for a command.
- `--json` prints machine-readable JSON.
- `--verbose` enables verbose diagnostics.

Examples:

```sh
idios --json repo list
idios --db ./scratch.sqlite3 issue list --json
```

The CLI does not currently expose every Web feature. Notes, Shelf, scheduled jobs, and other visual workflows are managed in the Web workspace. Use `idios help` for the exact commands available in your installed version.
