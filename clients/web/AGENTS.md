# IDIOS web implementation guide

This folder owns the Web presentation layer. The repository's root `AGENTS.md` documents the Go
application and domain boundaries. Web-only work must not change domain semantics or persistence.

## Architecture

- React + TypeScript + Vite + Tailwind CSS v4.
- `src/features/<feature>/` owns a route-level feature; each feature exposes its public API from
  `index.ts`. Cross-feature code must import that public entry point rather than deep-importing
  another feature's internal files.
- `src/design-system/` owns small, composable visual primitives and their CSS Modules.
- `src/app/layout/` owns the application shell and navigation; `src/lib/api.ts` owns the
  HTTP client; `src/types/idios.ts` owns the wire contract; `src/store/` may cache UI state but is
  not a second source of truth.
- Feature-only styles live beside the feature. Shared base styles and legacy feature styles are
  split into named files under `src/styles/`; do not add new page styles to a single global file.
- Every domain mutation calls the Go API and uses the existing `internal/application.Service`.
  No Node.js server is introduced; the production static bundle is embedded in the `idios` binary.

## Change isolation

- A feature may depend on `design-system`, `lib`, `types`, and documented public feature APIs.
- Shared primitives must not import feature code.
- Put stateful workflows in feature hooks/models, not monolithic page components.
- Add regression tests for a changed workflow and maintain existing loading, empty, error, keyboard,
  focus-visible, responsive, offline, and reduced-motion behavior.
- Keep new components small and single-purpose. Do not create one-file wrappers with no contract or
  behavior just to wrap a raw HTML tag; use `Box`, `Stack`, `Inline`, `Text`, `Tile`, and focused
  domain components where their abstraction removes duplication or defines a reusable contract.

## Commands

Run from the repository root: `make web-install`, `make web-build`, and `make web-lint`.

## Design

Dark is the default. Use the low-gloss ceramic/tile palette, compact squared edges, restrained
borders and motion, clear hierarchy, visible focus, responsive layouts, and accessible loading,
empty, and error states. Respect `prefers-reduced-motion`. Do not introduce runtime images or CDNs;
installed assets must work offline.
