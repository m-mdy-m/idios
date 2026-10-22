# Adding a feature

Use this checklist whenever a change affects workspace behavior rather than only text or styling.

## Module ownership

- **A screen or interaction:** `clients/web/src/pages/` composes focused feature components; the TUI/CLI adapter lives in its corresponding `clients/` package.
- **A shared UI primitive:** `clients/web/src/components/ui/` (buttons, cards, badges, fields, modal shell, loading/empty/error states). Keep color/spacing/type tokens in `clients/web/src/index.css` and reuse them.
- **A feature visual:** `clients/web/src/components/<feature>/`; keep SVG/pointer mechanics and inspection panels out of page components.
- **A protocol command or shared form:** `internal/protocol/`. Keep command metadata, field names, option sources, and handler contract in one place.
- **Input validation:** `internal/validation/` for reusable pure rules and normalization; `internal/application/` repeats business invariants for every caller.
- **A workflow/use case:** `internal/application/`; add one method and reuse it across TUI, CLI, and HTTP/protocol paths.
- **A shared model or invariant:** `internal/domain/`.
- **Persistence:** add a method to `internal/ports/`, implement it in `internal/storage/sqlite/`, and add a safe, forward-only migration where needed.
- **HTTP API boundary:** `clients/server/`; parse bounded request bodies, reject malformed/unknown fields, call the application, and return a stable error status/body.
- **Logs/health:** use `internal/diagnostics` and the local monitoring endpoints; don't create feature-specific log files or silently ship telemetry.

## Validation order

1. Resolve the requested command/route and body schema.
2. Reject unknown keys, malformed JSON, unexpected trailing data, unsupported enums and wrong data types.
3. Normalize strings and typed values; enforce size limits, UTF-8/null-byte rules, date/time/color/URL formats and required fields.
4. Validate that referenced entities exist, are in the correct scope, and are in an allowed state.
5. Validate cross-entity invariants, cycles and blocking dependencies.
6. Perform the mutation atomically in storage where partial state would be incorrect.
7. Return actionable errors; log operation/outcome/latency without recording secrets or complete private note/quote bodies.

A form-level required marker is only a usability feature. It is never the security or correctness boundary.

## Reuse the UI instead of duplicating it

- Use `Button` for all standard button variants (`primary`, `secondary`, `ghost`, `danger`) and one loading/disabled behavior.
- Use `Modal` as the generic modal shell and put page-specific form/detail/confirmation content inside it. Do not create a fresh backdrop or implement Escape/body-scroll behavior per page.
- Use `Card`, `Badge`, `Field`, `PageHeading`, `EmptyState`, `LoadingState` and `ErrorNotice` before writing another near-identical primitive.
- Put entity-specific UI (issue creation/edit, PR details, label selector, note editor, graph inspector) into a focused feature component. Repositories and the Issues page should call the same issue-creation command instead of maintaining parallel creation logic.
- Prefer design tokens and semantic color names over literal page-specific colors. Data-defined label colors are the deliberate exception and must be validated.

## Suggested implementation sequence

1. Write the behavior and acceptance examples, including invalid and boundary cases.
2. Add a domain/application contract with clear owners and transitions.
3. Add central validation and tests before touching persistence.
4. Extend ports + SQLite + migration transactionally, if needed.
5. Expose it via the shared protocol and HTTP adapter.
6. Connect all intended UI adapters and compose reusable components.
7. Update [Entity relationships](ENTITY_RELATIONSHIPS.md), the user guide and API notes.
8. Run formatting, Go tests/vet/race, browser lint/build, and migration/packaged smoke tests.

## Workflow rules to preserve

- Only accepting an IDIOS pull request completes an issue and creates its completion record.
- `depends_on` is the blocking relation; ordinary semantic links do not block progress.
- Prevent parent/dependency cycles and verify cross-repository constraints.
- Keep related persistence writes in the same transaction when partial results are incorrect.
- An IDIOS pull request is not a real Git commit or hosted PR merge.
