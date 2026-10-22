# Contributing to IDIOS

Thanks for considering a contribution. This guide explains the usual path from an idea to a reviewed change.

## Before you start

- Check [open issues](https://github.com/m-mdy-m/idios/issues) to see whether the problem is already being discussed.
- For a behavior change, describe the problem and the expected result before doing a large rewrite.
- For security issues, follow [SECURITY.md](SECURITY.md) rather than posting details publicly.
- Read the short [architecture guide](docs/development/ARCHITECTURE.md) if you are not sure where a change belongs.

## Set up the project

You need Go 1.25 or newer and a C compiler for SQLite. Node.js 22+ and pnpm 9 are needed to build or lint the browser interface. GNU Make is used by the shortcut commands in the Makefile.

```sh
git clone https://github.com/m-mdy-m/idios.git
cd idios
make build
```

## Make a change

1. Create a branch for one focused change.
2. Make the smallest change that solves the problem.
3. Add or update tests for changed behavior.
4. Update documentation when a user-visible command, setting, or workflow changes.
5. Do not commit personal SQLite databases, generated binaries, secrets, or machine-specific configuration.

Keep application rules in `internal/application/`. UI code should call the application rather than reimplementing the rule. For storage changes, add a numbered migration instead of asking users to delete their database. See [Adding a feature](docs/development/EXTENDING.md) and [Storage and migrations](docs/development/STORAGE.md).

## Run checks

Before opening a pull request, run the checks relevant to your change. The full project check is:

```sh
make ci
```

Useful individual checks include:

```sh
make fmt-check
make test
make vet
make test-race
make web-lint
make build
make smoke
```

On Windows, `make build` runs `scripts/build.ps1`. You can also run Go tests directly with `go test ./...` and `go vet ./...`.

If a check cannot run on your machine, mention that in the pull request rather than saying it passed.

## Open a pull request

Use a clear title and explain:

- what changed and why;
- how you tested it;
- any data migration or compatibility impact;
- any known limitation or follow-up work.

Keep the pull request focused and be open to review feedback. An IDIOS pull-request record is an in-app work item; contributions to this repository are submitted as normal GitHub pull requests.

## Community standards

Please follow the [Code of Conduct](CODE_OF_CONDUCT.md). For bugs and feature ideas, use [GitHub Issues](https://github.com/m-mdy-m/idios/issues).
