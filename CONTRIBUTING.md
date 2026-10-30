# Contributing to IDIOS

Thank you for considering a contribution. This guide explains the first steps. The detailed source layout and test process are in the [developer guide](docs/develop/README.md).

## Before changing code

- Search the existing [issues](https://github.com/m-mdy-m/idios/issues) for the same problem.
- Keep one change focused on one behavior or feature.
- For security problems, follow [SECURITY.md](SECURITY.md) and do not publish exploit details in a public issue.
- Read the [architecture overview](docs/arch/overview.md) and [service boundaries](docs/arch/service-boundaries.md) to find the correct owner.

## Set up the source

Requirements: Go 1.25+, a C compiler, Node.js 22+, pnpm 9, GNU Make, Python 3.12+, and Git.

```sh
git clone https://github.com/m-mdy-m/idios.git
cd idios
make build
```

## Make a change

1. Create a branch for the change.
2. Identify the feature or domain that owns the behavior.
3. Change the smallest number of modules that can own the new behavior cleanly.
4. Add or update tests for the normal path, errors, and important edge cases.
5. Update user or developer docs and `CHANGELOG.md` when behavior changes.
6. Do not commit databases, credentials, generated binaries, or machine-specific settings.

Business rules belong in Go domain/application services. SQL belongs in `internal/storage/sqlite/`. The Web client calls the local API and must not create a second source of truth. See [Adding a feature](docs/develop/adding-a-feature.md), [Web development](docs/develop/web.md), and [Storage](docs/develop/storage.md).

## Run checks

```sh
make ci
```

Or install the shared task runner and execute the repository's task graph:

```sh
python -m pip install "git+https://github.com/m-mdy-m/pipeline.git@main"
pipeline validate
pipeline run ci
```

If a check cannot run, state that it was **NOT RUN** and explain why. Do not claim that a syntax check is a full build.

## Open a pull request

Explain what changed, why it changed, how it was tested, and whether the change affects stored data, commands, routes, privacy, or release artifacts. A Pull Request record inside IDIOS is not a GitHub Pull Request; contributions to this repository use normal GitHub pull requests.

Please follow the [Code of Conduct](CODE_OF_CONDUCT.md).
