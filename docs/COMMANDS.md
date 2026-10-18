# IDIOS command reference

Everything the Engine CLI accepts, grouped by what it is for. Run `idios help` for the
one-screen version, or `idios help COMMAND...` (for example `idios help issue create`) for
one command's flags. `idios <group> --help` works too.

- [Running it](#running-it)
- [Global flags](#global-flags)
- [References](#references)
- [Repositories](#repositories) · [Projects](#projects) · [Issues](#issues) · [Pull requests](#pull-requests)
- [Completions and branches](#completions-and-branches) · [Concepts and relations](#concepts-and-relations)
- [Labels](#labels) · [Milestones](#milestones) · [Graph](#graph) · [Activity](#activity)
- [Legacy import](#legacy-import)
- [Environment variables](#environment-variables) · [Cheat sheet](#cheat-sheet)

## Running it

```sh
idios [--db PATH] [--json] [--verbose] COMMAND [FLAGS]
idios help COMMAND...
idios protocol
idios version
```

The default database is `~/.idios/idios-engine.sqlite3`. Global flags may appear before or
after the command. Every successful mutation prints one short confirmation; failures print
one readable line plus a usage hint (and never a stack trace).

The workflow rules — what can start, what counts as done — are in
[WORKFLOW.md](WORKFLOW.md). This page is only the command surface.

### Terminal UI

```sh
idios-tui
idios-tui --db ./work.sqlite3
idios-tui --no-color
make tui # run from the source checkout
```

The TUI is an independent, full-screen Bubble Tea presentation adapter with clickable tabs, a workspace sidebar, forms and a typed Graph Explorer. It calls the same Application Core as CLI. Bubble Tea and Lip Gloss are presentation-only dependencies. The `--db` flag selects the Engine database, while `--no-color` disables color for monochrome terminals or accessibility setups: in that mode the screen carries no escape sequences and marks the current control with `▸`. `NO_COLOR` and `TERM=dumb` are respected automatically. Unlike the CLI, the TUI requires an interactive terminal and does not support redirected stdin/stdout. Keyboard navigation and the Graph Explorer are documented in
[TUI.md](TUI.md).

## Global flags

| Flag | Meaning |
| --- | --- |
| `--db PATH` | Engine database file (default `~/.idios/idios-engine.sqlite3`, `IDIOS_DB` overrides) |
| `--json` | One JSON document on stdout; errors become a JSON envelope on stderr |
| `--no-json` | Return to human output for the rest of the argument list |
| `--verbose` | Write monitoring logs to stderr (same as `--log-level debug`) |
| `--log-level LEVEL` | `debug`, `info`, `warn`, `error` |

Logs always go to stderr and only when asked for, so scripts can rely on stdout.

## References

A few commands accept either a stable ID (`repo_01H…`) or a short, human reference:

| Form | Example | Used by |
| --- | --- | --- |
| `Repository#number` | `CPU#12` | Issues |
| `Repository!number` | `CPU!3` | Pull requests |
| `concept:Name`, `label:global/lang`, `branch:Repo/main`, `milestone:repository/CPU/Release`, `completion:Repo/main/1` | | `graph --from`, relations |

Repository, Project, Concept, Label and Milestone names are also accepted where unambiguous;
a name that matches two things is rejected with a message telling you to use the scoped form.

## Repositories

Repositories own Issues, Pull Requests and Branches. Creating one also creates its default
`main` Branch.

```sh
idios repo create --name CPU --description 'CPU and hardware learning'
idios repo list
idios repo show --repo CPU
idios repo settings get --repo CPU
idios repo settings set --repo CPU --key color --value blue
```

## Projects

A Project groups Issues and can span Repositories. Ownership of an Issue stays with its
Repository.

```sh
idios project create --name 'Assembly track' --description 'Everything about x86'
idios project list
idios project add-issue --project 'Assembly track' --issue CPU#2
idios project issues --project 'Assembly track'
```

## Issues

The unit of work. `--parent` makes a Sub-issue (same Repository only); `depends` adds a
blocking prerequisite that may cross Repositories.

```sh
idios issue create --repo CPU --title 'Learn basic electricity' --priority p2 --due 2026-02-01
idios issue create --repo CPU --title 'Learn registers' --parent CPU#1
idios issue list --repo CPU --status in_progress
idios issue show --id CPU#1
idios issue start --id CPU#1
idios issue cancel --id CPU#1
idios issue depends --issue ARLIZ#1 --on CPU#1
idios issue undepends --issue ARLIZ#1 --on CPU#1
idios issue deps --id ARLIZ#1
idios issue concept --issue CPU#2 --concept 'CPU Register'
idios issue label --issue CPU#2 --label p2
idios issue milestone --issue CPU#2 --milestone 'milestone:repository/CPU/Chapter 2'
```

`--status` is one of `open`, `in_progress`, `done`, `cancelled`. An Issue cannot be set to
`done` by hand; only an accepted Pull Request does that.

## Pull requests

The review record for finishing an Issue. The description is required and holds the work
explanation and result.

```sh
idios pr create --issue CPU#1 --description 'What I did, what I learned, and the result'
idios pr list --repo CPU --status open
idios pr show --id CPU!1
idios pr accept --id CPU!1 --summary 'Electricity fundamentals reviewed'
idios pr close --id CPU!1
```

`--status` is one of `open`, `accepted`, `closed`. Accepting the PR is the only path that
completes the Issue; it writes the CompletionRecord on `main` in the same transaction.

## Completions and branches

```sh
idios completion list
idios completion list --repo CPU
idios branch list --repo CPU
```

Completion history is the log of accepted work per Branch. V1 exposes `main` only.

## Concepts and relations

Concepts are global knowledge nodes. Relations are typed *semantic* edges — they never block
work. Use `issue depends` for blocking.

```sh
idios concept create --name 'CPU Register' --description 'Storage inside the CPU'
idios concept list
idios relation add --from-type concept --from 'CPU' --to-type concept --to 'Register' --type contains
```

`--type` defaults to `related_to`. Allowed: `related_to`, `uses`, `contains`, `part_of`,
`explains`, `contrasts_with`, `uses_concept`, `references`, `supports`, `contradicts`,
`implements`, `implemented_by`. `issue concept` is the shortcut for a `uses_concept` edge.

## Labels

Labels are global or scoped to one Repository. When you assign by name, a local label wins
over a same-name global one.

```sh
idios label create --name p2 --description 'High priority'
idios label create --name lang --scope repository --repo CPU --color blue
idios label list
idios label list --scope CPU
idios label assign --issue CPU#2 --label p2
```

## Milestones

A Milestone is scoped to a Repository or a Project. Assigning a Project milestone first
requires the Issue to be in that Project.

```sh
idios milestone create --owner CPU --title 'Chapter 2' --due 2026-03-01
idios milestone create --scope project --owner 'Assembly track' --title 'Foundations'
idios milestone list --scope repository --owner CPU
idios milestone assign --issue CPU#2 --milestone 'Chapter 2'
```

`--scope` is `repository` (default) or `project`.

## Graph

```sh
idios graph
idios graph --from issue:CPU#1 --depth 2
```

Without `--from` it prints the whole graph; with it, the neighbourhood within `--depth`
edges (default 1). Nodes and edges are sorted so output and JSON stay stable.

## Activity

```sh
idios activity list
idios activity list --type issue --id i_01H… --limit 20
```

Recent workflow events. Not an event-sourcing log and not the source of truth for state.

## Legacy import

Reads an existing IDIOS 0.4.x SQLite file **read-only** and imports its Concepts and Tasks
(Tasks become Issues; historical done Tasks get a labelled synthetic PR). The source file is
never modified.

```sh
idios legacy-import --from ./old-idios.sqlite3
```

Details and caveats: [MIGRATION.md](MIGRATION.md).

## Environment variables

| Variable | Meaning |
| --- | --- |
| `IDIOS_DB` | Default database path when `--db` is not given |
| `IDIOS_LOG_LEVEL` | Enable stderr logging (`debug`, `info`, `warn`, `error`, or `off`) |

## Cheat sheet

```sh
# A small cross-Repository run
idios repo create --name CPU
idios repo create --name ARLIZ
idios issue create --repo CPU --title 'Learn basic electricity'
idios issue create --repo ARLIZ --title 'Write chapter three'
idios issue depends --issue ARLIZ#1 --on CPU#1
idios issue start --id ARLIZ#1                 # blocked until CPU#1 is done
idios issue start --id CPU#1
idios pr create --issue CPU#1 --description 'Reviewed and wrote up the result.'
idios pr accept --id CPU!1 --summary 'Electricity basics reviewed'
idios issue start --id ARLIZ#1                 # now allowed
idios completion list
idios graph --from issue:ARLIZ#1 --depth 3
```


## Development commands

The Makefile provides a repeatable quality gate for contributors. These are build-tool commands,
not part of the `idios` runtime CLI.

| Command | Purpose |
| --- | --- |
| `make build` | Build `dist/idios` and `dist/idios-tui`. |
| `make test` | Run all Go tests. |
| `make test-race` | Run tests with the race detector. |
| `make test-cover` | Run tests and write `coverage.out`. |
| `make vet` | Run `go vet ./...`. |
| `make fmt` / `make fmt-check` | Format Go files / verify formatting without edits. |
| `make check` | Formatting check, tests and vet. |
| `make smoke` | Build and run the cross-Repository workflow smoke test. |
| `make ci` | Full local gate: checks, race tests, build and smoke test. |
| `make psx-check` / `make psx-fix` | Check/fix repository shape using PSX v3. |
| `make tidy` | Reconcile `go.mod` and `go.sum`. |
| `make clean` | Remove generated binaries and coverage data. |

The storage layout, migration rules and connection options are documented in [STORAGE.md](STORAGE.md);
terminal navigation is documented in [TUI.md](TUI.md).
