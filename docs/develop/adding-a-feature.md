# Add a feature

This page gives a small, repeatable path for adding one feature without coupling unrelated areas.

## 1. Decide who owns the behavior

Ask these questions:

- Is this a new business rule? It belongs in `internal/domain/` or `internal/application/`.
- Does it read or write data? Give the owning service a narrow port in `internal/ports/` and implement that port in SQLite storage.
- Is it an HTTP operation? Add a handler under `clients/server/handlers_<area>.go`.
- Is it a page or user workflow? Add it under `clients/web/src/features/<feature>/`.
- Is it reusable visual UI? Add it to `design-system/` only when more than one feature can use the same contract.

## 2. Keep the feature boundary small

A Web feature should own its pages, private components, state/hooks, API calls, styles, and tests. Export only the stable public surface from `index.ts`. Another feature must not import private files directly.

A Go application service should depend on the narrowest storage port it needs. It should not reach into SQL or import HTTP/Web packages.

## 3. Implement the operation

1. Define the input and output types.
2. Validate the input at the application boundary.
3. Implement the use case in the owning service.
4. Add the narrow persistence method if storage is needed.
5. Add the HTTP adapter and the Web UI if users need it.
6. Add CLI support only when the operation is useful from a terminal and has a suitable command contract.

## 4. Test edge cases

Cover at least:

- valid input and the expected saved result;
- missing or malformed input;
- unknown or deleted references;
- permission/privacy and offline behavior where relevant;
- persistence failure;
- duplicate or rapid repeated submissions;
- loading, empty, and error states in the UI.

For AI-generated actions, test that model text is never treated as a successful save until the application confirms persistence.

## 5. Update documentation

Update the matching guide, architecture page, route/command reference, and `CHANGELOG.md`. Remove outdated descriptions rather than appending a new paragraph that contradicts them.

## 6. Run checks

Start with the focused package and Web check, then run `make ci` or the relevant shared pipeline task. Report anything that was **NOT RUN**.
