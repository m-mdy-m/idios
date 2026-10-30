# Shared pipeline

IDIOS uses [`m-mdy-m/pipeline`](https://github.com/m-mdy-m/pipeline) for the reusable GitHub CI task and for the local task runner. This avoids maintaining a second copy of the task engine in IDIOS.

## Version pin

The helper downloads pipeline commit `a649e8cac30598a302ec7a699ce9f76bd4243f19`. The local wrapper and GitHub caller use the same reviewed revision. Update the pin only after reviewing the pipeline change and running the local tasks.

## Run it locally

Requirements: Python 3.12+ and Git.

```sh
python scripts/run-pipeline.py validate
python scripts/run-pipeline.py list
python scripts/run-pipeline.py run check
python scripts/run-pipeline.py run build
python scripts/run-pipeline.py run ci
```

The Makefile has shorter aliases for the common operations:

```sh
make pipeline-validate
make pipeline-list
make pipeline-check
```

The first command downloads the pinned pipeline source into `.tools/pipeline/`. That directory is ignored by Git. IDIOS does not copy or fork the pipeline engine into its source tree.

The task list lives in the root `pipeline.toml`. It describes tasks as argument arrays with dependencies. The runner executes commands without an implicit shell. Both local development and the GitHub CI caller use this same task graph, so documentation checks, Go checks, Web checks, build, race tests, and smoke tests do not drift into separate command lists.

## Task names

- `check`: documentation links, Go formatting, Go tests, vet, and Web lint.
- `build`: build the embedded CLI/Web executable after checks.
- `race`: run Go tests with the race detector.
- `smoke`: run the built executable through the platform smoke test.
- `ci`: run the full local verification sequence.

Use `python scripts/run-pipeline.py run ci --json` to get a machine-readable result. The wrapper passes the current repository as the task workspace.

## GitHub Actions

`.github/workflows/ci.yml` is intentionally a small caller for the reusable CI workflow in the shared pipeline repository. The caller pins a commit rather than `@main`.

GitHub requires a workflow file in the consuming repository to trigger a reusable workflow. That file is only a caller: the shared pipeline workflow validates the configuration and invokes the same `pipeline.toml` task graph used locally. Project-specific build commands stay in IDIOS Make targets and small scripts.

The release caller also delegates to the shared pipeline release workflow. IDIOS supplies small project-specific scripts for native CGO toolchains, packaging, and smoke checks so Linux DEB/RPM, macOS PKG/DMG, and Windows ZIP/setup assets remain available. CodeQL and secret scanning remain separate security workflows because the shared repository does not provide those scanners.

For pipeline behavior and inputs, see the [pipeline repository](https://github.com/m-mdy-m/pipeline) and its [workflow guide](https://github.com/m-mdy-m/pipeline/blob/main/docs/WORKFLOWS.md).
