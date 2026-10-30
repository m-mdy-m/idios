# Testing and verification

A check is complete only when it was run in the current checkout and passed. Reading code or checking syntax is not the same as building or running the program.

## Quick checks

```sh
make fmt-check
make test
make vet
make web-lint
```

## Full local gate

```sh
make ci
```

The Makefile runs formatting, Go tests, vet, Web lint/build, race tests, and smoke checks through the project targets. Some targets may rebuild the Web client more than once.

## Focused tests

```sh
go test ./internal/ai ./clients/server
cd clients/web
pnpm run check:architecture
pnpm run check:study-actions
pnpm run check:reading-skills
pnpm run build
pnpm run lint
```

Use the smallest focused package first, then run the full gate when a change crosses package boundaries.

## Windows

Use PowerShell and run `make` from the repository root. The Makefile uses PowerShell for build and formatting scripts. `make smoke` uses Git Bash; install [Git for Windows](https://git-scm.com/download/win) if that command is missing.

## Reporting results

For a pull request or release note, list the exact commands run. Use **NOT RUN** for checks that could not run because a toolchain, dependency, platform, or service was unavailable. Do not describe a static check as a full build.
