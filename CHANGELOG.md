# Changelog

## [Unreleased]

### Fixed

- Bound each sequential Shelf discussion agent turn, including its single correction attempt, to 120 seconds so an unresponsive model cannot keep the remaining agents waiting indefinitely.
- Show an accurate waiting-for-model status while a discussion agent is generating a response.

## [3.2.2] - 2026-10-31

### Added

- Add verified source-to-target word alignments to sentence-aligned Shelf translations, enabling precise hover linking between PDF words and their translated equivalents.
- Show live elapsed time and completed-part progress while page translation is running.

### Changed

- Send typical PDF pages in one larger translation request and process unusually long pages in bounded parallel batches, preserving source order, paragraph separators, and resumable chunk caching.
- Use a low-variance translation request with an output budget sized to the passage, while keeping book terminology and nearby-page context available.

### Fixed

- Keep Mimi's page thought bubble visible and interactive while the translation companion is open.
- Restore exact Mimi margin highlights on browsers without the CSS Custom Highlight API, using range-derived overlay rectangles instead of silently dropping marks.
- Republish Mimi's page marks when Focus Mode changes so returning to normal reading restores the current page's notes.

## [3.2.1] - 2026-10-31

### Changed

- Replace the harsh notification alarm effects and spoken warnings with a short, locally synthesized, gentle chime for inbox notifications and plan reminders.
- Keep the browser notification sound preference and volume control, but cap the generated audio master level to avoid sudden loud peaks.
- Update Notification Settings to describe the softer sound and preview the actual chime used by incoming notifications.

### Fixed

- Send test quote reminders with the supported `info` notification level instead of the unsupported `quote` level, which previously caused `POST /api/protocol/execute` to return HTTP 400.
- Add a protocol regression test that sends a test quote reminder and verifies the stored notification severity.

## [3.2.0] - 2026-10-31

### Added

- Added mandatory role-specific deliverables for built-in Shelf discussion agents, with explicit identity, originality, source-verification, and non-echo requirements.
- Added regression coverage for chapter-range source validation, bounded discussion-history carry-over, citation preservation, and distinct agent role prompts.

### Changed

- Make MIMI the single primary Shelf identity and final discussion Chair; prefer a larger verified local model for its moderation role while preserving explicit user model choices.
- Strengthen the sequential-agent skill with mandatory role-specific contribution, non-echo review after Chair turns, and explicit preservation of negation, scope, and argument structure.
- Preserve up to 40 completed discussion turns across continuations, using shared UTF-8 byte limits between the Web client and Go runtime instead of dropping history after eight turns.
- Keep source markers when discussion history must be compacted, and disclose omitted or shortened carry-over to subsequent agents.
- Require an available MIMI · Chair for Shelf round-table starts and continuations; final synthesis must compare evidence rather than count repeated agreement.
- Strengthen Mimi's page-reading contract to preserve explicit negations, qualifications, and the distinction between a page's main argument and secondary acknowledgements.

### Fixed

- Reject verbatim peer echoes, unnamed peer responses, and final Chair summaries that omit available participant positions; require one focused repair and keep repeat failures out of subsequent peer evidence.
- Correctly recognize inclusive chapter ranges such as `Chapters IV–VIII` when validating chapter references, preventing valid references to range endpoints from being rejected.
- Prevent failed agent turns from being passed forward as if they were completed peer arguments.

## [3.1.2] - 2026-10-31

### Added

- Add an in-reader, translation-first companion for PDF pages and selected passages. The full page translation is the default view; Word, Grammar, and IELTS are available from a compact tool menu and run as focused tasks.
- Add a compact settings popover for target language, AI model, and per-request timeout, plus a resizable, collapsible reader companion docked beside the PDF page.
- Retain the reader's resized companion dimensions when it is closed and reopened.
- Dock the resizable reading companion to the left side of the PDF reader and synchronize hover highlights between source text and sentence-aligned translation segments.
- Render translation Markdown safely in the reader, including paragraphs, headings, emphasis, inline code, and lists.
- Add source-grounded grammar and IELTS analysis on demand, without making either analysis part of the page-translation request.

### Changed

- Use book subject, nearby PDF pages, saved notes, highlights, and the existing glossary only as supporting context for terminology and meaning.
- Return a target-language-only translation field with exact sentence-aligned source/translation segments for PDF hover synchronization; keep vocabulary, grammar, and IELTS analysis in separate focused requests.
- Translate long pages in larger sequential chunks to reduce provider round trips, display partial results, and keep completed chunks in the Reader session so a retry resumes at the first unfinished chunk.
- Allow translation requests up to the Runtime's bounded 10-minute ceiling; the provider HTTP client no longer silently truncates a longer per-request deadline at its ordinary 120-second default.

### Fixed

- Keep the PDF page open when translation opens; show the companion as a dismissible, resizable left-side panel rather than navigating away from the book.
- Keep the sidebar expand control available while navigation is collapsed, so the same control can collapse and restore the sidebar without opening Settings.
- Render Search as a centered icon in the collapsed sidebar instead of squeezing its text label into the narrow navigation rail.

All notable changes to IDIOS are recorded here. This file follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and releases follow [Semantic Versioning](https://semver.org/).

## [3.1.1] - 2026-10-31

### Changed

- Use the shared pipeline's provider-neutral `PIPELINE_ARTIFACT_TEST_DIR` contract in Linux, macOS, and Windows release artifact tests instead of an IDIOS-specific environment variable.

### Fixed

- Release artifacts now use the canonical release tag provided by the shared pipeline, including branch dry-runs.
- Fix the Windows browser-launch test shim so repeated invocations append separate newline-delimited records rather than overwriting the previous call. This makes the second-click shortcut smoke test reliably verify reuse of the existing workspace.

## [3.1.0] - 2026-10-30

### Added

- Added a native Linux desktop launcher using a .desktop entry and the IDIOS SVG icon.
- Added a Windows desktop and Start Menu shortcut using a hidden Windows Script Host launcher.
- Added a native macOS IDIOS.app bundle with an app icon and a bundled IDIOS server executable.
- Added release artifact tests that download and execute each platform package on its native runner before release publication.

### Changed

- Installers now create platform-native launchers: desktop/menu shortcuts on Linux and Windows, and IDIOS.app on macOS.
- Linux DEB/RPM packages and portable archives now include their desktop entry and icon.
- Windows ZIP and Setup packages now include the VBS launcher and application icon.
- macOS TAR, PKG, and DMG packages now include the app bundle.

### Fixed

- Repeated clicks on serve --open now reuse an already-running local server and open its workspace instead of failing to bind the occupied port.

## [3.0.2] — 2026-10-30

IDIOS 3.0.2 is a maintenance release focused on making native release builds
reproducible across the supported Linux, macOS, and Windows targets.

### Fixed

- Provision Go 1.25.0 explicitly in the shared release workflow so macOS
  runners and all other targets have the toolchain required by go.mod.
- Provision Node.js 22 and pnpm 9.15.4 for the embedded Web frontend build
  instead of relying on tools already present on hosted runners.
- Configure Windows AMD64 with the MSYS2 UCRT64 GCC toolchain and Windows
  ARM64 with the CLANGARM64 Clang toolchain.
- Stop running a full MSYS2 runtime upgrade from inside the active release
  shell; compiler packages are installed by the reusable workflow before
  the test and build steps.
- Preserve CC and CGO_ENABLED for subsequent release steps and report
  actionable errors when a Windows compiler environment is not active.

## [3.0.1] — 2026-10-30

IDIOS 3.0.1 is a maintenance release focused on shared pipeline integration,
CI reliability, and repository security checks. It removes the project-local
pipeline wrapper and makes the shared task runner a first-class reusable
workflow capability.

### Changed

- Run the root `pipeline.toml` task graph directly through the shared
  `m-mdy-m/pipeline` GitHub provider adapter. The reusable workflow owns
  checkout and Python setup; IDIOS no longer needs a wrapper script or a
  private copy of the pipeline runner.
- Configure the mixed Go and Node.js toolchains through reusable CI inputs,
  including the pinned pnpm version used by the Web workspace.
- Configure Markdown linting around the repository's actual documentation:
  keep structural checks, avoid a hard line-length rule for prose and tables,
  allow repeated section headings across changelog versions, and exclude skill
  prompt files that intentionally use their own format.
- Keep React's `set-state-in-effect` diagnostic visible as a warning while
  legacy async page-loader effects are migrated; the frontend lint gate now
  completes with zero errors and reports remaining warnings.
- Keep CodeQL and Gitleaks as active security checks while preserving the
  shared reusable workflow boundary.

### Fixed

- Removed the broken `scripts/run-pipeline.py` entry point, which invoked a
  non-existent `scripts/pipeline.py` path in the shared repository.
- Corrected Markdown validation failures from repository templates, links,
  headings, fenced blocks, and changelog references.
- Replaced secret-looking test fixture literals with runtime-generated
  non-secret values so Gitleaks can scan the repository without a broad
  allowlist or disabling the check.
- Composed local notification preference keys from non-secret segments to
  avoid a Gitleaks generic-api-key false positive without adding an allowlist.
- Updated the AI status test to expect the current local-first default and
  SQLite migration tests to assert the current schema version, 13.
- Corrected frontend regex/prompt escaping and unused-variable lint errors.
- Improved shared Gitleaks failure diagnostics to report the rule, file, and
  line while keeping secret values redacted.

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
- Pipeline task configuration for the shared [`m-mdy-m/pipeline`](https://github.com/m-mdy-m/pipeline) task runner and reusable CI workflows.

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

[3.2.2]: https://github.com/m-mdy-m/idios/compare/v3.2.1...v3.2.2
[3.2.1]: https://github.com/m-mdy-m/idios/compare/v3.2.0...v3.2.1
[3.2.0]: https://github.com/m-mdy-m/idios/compare/v3.1.2...v3.2.0
[3.1.2]: https://github.com/m-mdy-m/idios/compare/v3.1.1...v3.1.2
[3.1.1]: https://github.com/m-mdy-m/idios/compare/v3.1.0...v3.1.1
[3.0.2]: https://github.com/m-mdy-m/idios/compare/v3.0.1...v3.0.2
[3.0.1]: https://github.com/m-mdy-m/idios/releases/tag/v3.0.1
[3.0.0]: https://github.com/m-mdy-m/idios/compare/v3.0.0...v3.0.1
[2.0.0]: https://github.com/m-mdy-m/idios/compare/v2.0.0...v3.0.0
