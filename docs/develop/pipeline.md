# Shared pipeline

IDIOS uses [`m-mdy-m/pipeline`](https://github.com/m-mdy-m/pipeline) for its local task runner and reusable GitHub Actions workflows. The project task graph lives in the root `pipeline.toml`; IDIOS does not maintain a second copy of the pipeline engine.

## Update policy

IDIOS follows the `main` branch of the shared pipeline repository. The local runner fetches the latest `main` before it executes, and reusable workflow callers use `@main`. If the network is unavailable but a previous clone exists, the local runner clearly reports that it is using the cached commit.

`main` is a moving reference: changes to the shared pipeline can affect the next local or hosted run. The shared repository's own tests and compatibility checks should therefore remain healthy, and IDIOS should run its full task graph after shared-runner changes.

## Run it locally

Requirements: Python 3.12+, Git, Go 1.25+, Node.js 22, and pnpm 9.15.4.

```sh
python scripts/run-pipeline.py validate
python scripts/run-pipeline.py list
python scripts/run-pipeline.py run check
python scripts/run-pipeline.py run build
python scripts/run-pipeline.py run ci
```

The Makefile provides aliases for the shared runner:

```sh
make pipeline-validate
make pipeline-list
make pipeline-check
```

The first invocation downloads the shared pipeline source into `.tools/pipeline/`. That directory is ignored by Git. The task graph itself remains in `pipeline.toml` and uses argument arrays, explicit dependencies, task-specific working directories where needed, and bounded timeouts.

## Task graph

- `docs`: verify required documentation entry points, local links, and removed legacy reports.
- `format`: enforce `gofmt` formatting.
- `go-test` and `go-vet`: run the Go test suite and static checks.
- `web-lint`: install the Web dependencies and run ESLint.
- `check`: aggregate documentation, formatting, Go, and Web checks.
- `race`: run Go tests with the race detector after the quality checks.
- `build`: build the Web frontend and embed it in the single IDIOS executable.
- `smoke`: exercise the CLI, embedded Web server, and SQLite workflow.
- `ci`: execute the complete dependency graph in that order.

Commands are intentionally declared as argument arrays. The task runner does not implicitly invoke a shell; shell pipelines and compound commands belong in a script or an explicit shell invocation.

## GitHub Actions

The local `.github/workflows/ci.yml` is a thin caller of the shared `ci.yml` reusable workflow. It prepares the pinned pnpm release, validates `pipeline.toml`, and delegates the full project task graph to `scripts/run-pipeline.py`.

The release caller delegates matrix execution, release-note preparation, asset collection, checksum generation, and GitHub Release publication to the shared `release.yml` workflow. IDIOS keeps the target-specific verification and packaging commands in `scripts/release/` because they need native CGO toolchains and platform-specific installers.

The documentation caller uses the shared `docs-check.yml` workflow for Markdown and workflow syntax validation. The secret-scan caller uses the shared `security-check.yml` workflow for Gitleaks. CodeQL remains a project-owned workflow because it is a separate compiled-Go static analysis job rather than a general shared pipeline task.

The shared workflows are called with `@main` as requested. This deliberately follows the central repository's current workflow definitions rather than pinning each call to a commit SHA. See the [pipeline repository](https://github.com/m-mdy-m/pipeline), its [task configuration guide](https://github.com/m-mdy-m/pipeline/blob/main/docs/configuration.md), and its [provider/workflow guide](https://github.com/m-mdy-m/pipeline/blob/main/docs/providers.md).
