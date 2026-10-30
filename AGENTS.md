# IDIOS engineering guide

## Product shape

IDIOS consists of a Go Engine, one CLI, and a local HTTP server embedding a static Web application.
The full-screen TUI and its standalone binaries have been retired. Legacy installer cleanup and log
migration may mention old filenames only to remove/migrate old installations; no TUI runtime is built.

## Go boundaries

- `internal/domain/` holds business types and invariants; it must remain independent of UI and storage.
- `internal/application/` owns use cases, transaction boundaries, workflow validation and orchestration.
- `internal/validation/` shares command validation; `internal/protocol/` publishes action schemas.
- `internal/storage/sqlite/` owns SQL and migrations. Do not issue SQL from CLI or Web handlers.
- `clients/cli/` and `clients/server/` are adapters over Application. The Web UI uses the HTTP API and
  must never implement a second source of truth.
- Preserve backward-compatible data migrations and test transaction-sensitive changes.

## Web feature architecture

- `clients/web/src/app` is the app composition/router boundary.
- `clients/web/src/features/<feature>/` owns feature pages, private UI components, state/hooks, API
  orchestration, feature-local tests and styles. Each feature has a public `index.ts`.
- Cross-feature dependencies must use only another feature's public entry point. Private internals are
  not API. Add a test/check when introducing an architectural rule.
- `clients/web/src/design-system/` owns composable UI primitives and CSS Modules. It must not import
  feature code. Domain-specific pieces stay inside their owning feature.
- `clients/web/src/lib/` and `src/types/` hold shared infrastructure and wire contracts, not page UI.
- New CSS belongs beside the component/feature in CSS Modules or named files under `src/styles/`; do
  not grow a monolithic global stylesheet. Avoid broad element selectors and cross-feature overrides.
- Prefer small, focused components and hooks with one responsibility, meaningful names, stable props and
  tests. Do not make empty wrappers around every HTML tag if they add no contract or reuse.

## Change workflow

1. Identify the owning feature/domain and its public contract before editing.
2. Make the smallest coherent change; do not refactor unrelated features in the same patch.
3. Test the changed use case, its error/loading/empty states and important regressions.
4. Run `make test`, `make web-build`, `make web-lint`, and `make smoke-release` when the environment
   provides their toolchains. Report any skipped check explicitly.
5. Update docs when a public command, route, configuration default, release artifact, or architecture
   boundary changes.

## Release shape

Build and release a single `idios` executable per target, with the Web bundle embedded. Node/pnpm are
  build dependencies, not runtime dependencies. Do not restore TUI targets or packages.
