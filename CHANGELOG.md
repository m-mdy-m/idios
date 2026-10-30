# Changelog

All notable changes to IDIOS are recorded here. This file follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and releases follow [Semantic Versioning](https://semver.org/).

## [3.0.0] — 2026-10-30

IDIOS 3.0.0 is a breaking release. It removes the full-screen TUI and its demo executable, leaving CLI and Web as the supported interaction surfaces. It also rebuilds the Web architecture, learning workflows, AI provider discovery, documentation, and shared automation.

### Added

- A feature-oriented Web architecture with a public entry point for each feature, shared Design System primitives, feature-owned styles, and checks that prevent imports across private feature internals.
- Domain-focused Go application services and narrow persistence ports for repositories, projects, issues, pull requests, concepts, graph, notes, quotes, planning, jobs, notifications, activity, settings, and Shelf.
- Domain-specific HTTP handler files; the server entry point now concentrates on routing, middleware, and lifecycle rather than holding every endpoint implementation.
- A dark, tile-based workspace shell, reusable ceramic-style surface primitives, graphite and cool-neutral visual tokens, and a dedicated IDIOS brand kit with marks, wordmarks, favicons, and application icons.
- Shelf library filters for reading status, topic, file type, sort order, and display mode, with visible removable filter chips.
- A provider picker for model-catalogue refresh in Settings → AI. A refresh now names one provider explicitly; the server rejects missing or invalid provider selections.
- Categorized, searchable reading-agent selection, compact discussion personas, reusable reading skills, contextual translation, Tutor/Mimi controls, and structured model response formats.
- An extensible AI provider-adapter registry, generic model metadata, standard structured-output request formats, local Ollama setup, reading-agent profiles, and bundled reading skills.
- More focused in-reader discussions with searchable and categorized participants, compact discussion personas, and additional grounded-reading skills.
- Reading analytics and Focus sessions with saved session history, reading progress, and Activity integration.
- Pipeline task configuration and a local runner bootstrap that uses the shared [`m-mdy-m/pipeline`](https://github.com/m-mdy-m/pipeline) project from the shared project’s `main` branch.

### Changed

- Retired the terminal UI and its demo executable. IDIOS now exposes two interaction surfaces: CLI and the local Web workspace.
- Simplified the command entry point: running `idios` without arguments prints CLI help; `idios web` opens the local workspace; `idios serve` runs the server directly.
- Updated installers, updater behavior, packaging, and release artifacts to install and update one `idios` executable instead of CLI and TUI binaries.
- Rebuilt the Web shell and Overview around compact navigation, grouped modules, focused content, dark defaults, and a consistent tile-based layout.
- Reworked Shelf library, book details, and PDF reading screens into a quieter, more compact reading experience. The redundant “Read in place / PDF Reader” block was removed from the book overview; the direct PDF reading action remains available.
- Split Shelf study-assistant types, action parsing, storage, context preparation, and response rendering into separate modules.
- Changed model discovery from a global refresh into a provider-scoped operation. Refreshing Ollama, 9Router, OpenRouter, or another configured provider no longer contacts the others or replaces their cached catalogues.
- Stopped opening the Shelf reader from automatically refreshing every provider. The reader uses cached model data and only refreshes an explicitly resolved provider.
- Kept IDIOS as a modular monolith: feature services have clear ownership and contracts but run in the same local process and share the application's storage lifecycle.
- Replaced the documentation tree with `docs/product`, `docs/user`, `docs/arch`, `docs/develop`, and `docs/reference`; rewrote the root README and the documentation index for readers who are new to IDIOS.
- Moved GitHub CI and release orchestration to reusable workflows on the shared pipeline repository’s `main` branch; kept project-owned scripts for target-specific builds, native installers, and smoke tests.
- Set the default CLI and updater version to `3.0.0`.

### Fixed

- Made the service-boundary check resolve the repository root from its own script location, including when pnpm launches it from `clients/web` on Windows.
- Made a selected-passage highlight use the annotation persistence API directly instead of depending on an AI action envelope.
- Preserved a user's note when an AI response contains useful text but does not match the expected action format; save status and failures are reported by the application.
- Enforced provider selection and Offline-only restrictions at the server boundary, not only in the browser picker.
- Limited Ollama model probes to the selected Ollama provider rather than probing models from every provider.
- Improved PDF transport and rendering reliability: validate PDF bytes, support range requests and a JSON fallback, bundle PDF.js decoder assets, serve worker assets with explicit MIME types, retry transient failures, and avoid aborting a shared fetch during React StrictMode cleanup.
- Improved reading-agent output repair, source-marker checks, truncation detection, thinking-mode handling, participant context, and response budgets for local models.
- Updated the updater and platform installers to read the `SHA256SUMS` manifest generated by the shared release workflow.
- Kept the Reading Tutor inside the active browser fullscreen element, prevented Mimi popups from shifting the avatar layout, and retained a compact Focus Mode control in the PDF reader.
- Fixed TypeScript reading-session imports and stale callback context in translation and annotation actions.

### Removed

- The TUI application, TUI demo target, TUI navigation metadata, and TUI build/release assets.
- The old large, duplicated documentation sections and temporary redesign/provider-refresh report files.
- The unused PDF Reader promotional block from the book detail page.

## [2.0.0] — 2026-10-22

- Added the local Web workspace, Go HTTP server, shared CLI/Web application layer, feature catalog, and SQLite-backed workspace.
- Added dated Plans, task checklists, reminder delivery, notification controls, and Activity integration.
- Added the simulated Repository workflow: Issues, IDIOS Pull Requests, immutable completion records, virtual files, and project grouping.
- Added the knowledge graph, concepts, labels, milestones, notes, quotes, notifications, activity history, settings, and diagnostics.
- Added local server controls, browser autostart integration, and a single local database shared by CLI and Web.
- Added the initial Shelf reading library and PDF Reader with progress, highlights, notes, search, zoom, and PDF.js rendering.

[3.0.0]: https://github.com/m-mdy-m/idios/releases/tag/v3.0.0
[2.0.0]: https://github.com/m-mdy-m/idios/compare/v2.0.0...v3.0.0
