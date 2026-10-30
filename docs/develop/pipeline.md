# Shared pipeline

IDIOS uses [`m-mdy-m/pipeline`](https://github.com/m-mdy-m/pipeline) for its
portable task runner and reusable GitHub Actions workflows. The task graph is
defined once in the root `pipeline.toml`. IDIOS does not ship a second runner,
clone the tool into `.tools/`, or maintain a project-specific wrapper script.

## Update policy

Local installs and reusable workflows follow the shared repository's `main`
branch. This is a moving reference: updates to the shared runner or reusable
workflow can affect the next run. IDIOS keeps its workflow contract small and
runs the task graph against the selected shared version.

## Run it locally

Requirements: Python 3.11+ and Git to install the shared CLI. IDIOS tasks also
require Go 1.25+, Node.js 22, and pnpm 9.15.4.

Install the shared runner:

```sh
python -m pip install "git+https://github.com/m-mdy-m/pipeline.git@main"
```

Use the runner's own commands; they read `pipeline.toml` from the current
repository root:

```sh
pipeline validate
pipeline list
pipeline run check
pipeline run build
pipeline run ci
```

There is no project-local runner shim, downloaded tool checkout, or Makefile
alias that forwards to a repository-specific wrapper. The shared CLI is the
entry point; the TOML file is the project configuration.

## Task graph

- `docs`: verify required documentation entry points, local links, and removed
  legacy reports.
- `format`: enforce `gofmt` formatting.
- `go-test` and `go-vet`: run the Go test suite and static checks.
- `web-lint`: install the Web dependencies and run ESLint.
- `check`: aggregate documentation, formatting, Go, and Web checks.
- `race`: run Go tests with the race detector after the quality checks.
- `build`: build the Web frontend and embed it in the single IDIOS executable.
- `smoke`: exercise the CLI, embedded Web server, and SQLite workflow.
- `ci`: execute the complete dependency graph in that order.

Commands are declared as argument arrays. The task runner does not implicitly
invoke a shell; shell pipelines and compound commands belong in a project
script or an explicit shell invocation.

## GitHub Actions

The local `.github/workflows/ci.yml` is a thin caller of the shared `ci.yml`
reusable workflow. It declares the toolchains needed by this mixed Go/Node.js
project and passes `pipeline-task: ci`. The reusable workflow checks out the
shared engine, sets up Python, and invokes
`python -m pipeline.providers.github task ci --json` itself. No project-local
runner script is needed.

The release caller delegates matrix execution, release-note preparation, asset
collection, checksum generation, and GitHub Release publication to the shared
`release.yml` workflow. IDIOS keeps target-specific verification and packaging
commands in `scripts/release/` because they need native CGO toolchains and
platform-specific installers.

The documentation caller uses the shared `docs-check.yml` workflow. The
secret-scan caller uses the shared `security-check.yml` workflow for Gitleaks.
CodeQL remains project-owned because it is a separate compiled-Go static
analysis job rather than a general shared pipeline task.

All shared reusable workflows are called with `@main`. See the
[pipeline repository](https://github.com/m-mdy-m/pipeline), its
[task configuration guide](https://github.com/m-mdy-m/pipeline/blob/main/docs/configuration.md),
and its [provider/workflow guide](https://github.com/m-mdy-m/pipeline/blob/main/docs/providers.md).
