# IDIOS architecture

IDIOS is a local-first workspace with one shared Go application core and multiple adapters. The terminal, command-line, and browser interfaces must reach the same use cases; UI components may format and collect input, but they must not own business rules or issue SQL.

## Layer map

```text
cmd/                             Composition root / executable entry points
  └─ clients/cli                 CLI adapter
  └─ clients/tui                 Terminal UI adapter
  └─ clients/server              Loopback HTTP API + browser host
       └─ clients/web             React + TypeScript presentation layer

internal/protocol                Shared command catalog / input contract
internal/validation              Pure validation rules and normalization
internal/application             Use cases, workflow checks, orchestration
internal/domain                  Entities, state, transitions, invariants
internal/ports                   Storage and external-service interfaces
internal/storage/sqlite          SQLite implementation and migrations
internal/diagnostics              Local logs and file-level diagnostics
internal/config                  Local settings, directories, preferences
```

The direction of dependencies is deliberate:

```text
CLI / TUI / HTTP ──> protocol (where applicable) ──> validation
       │                                                │
       └────────────────────> application <─────────────┘
                                  │
                         domain + ports
                                  │
                          SQLite adapter
```

The domain and validation packages must not import UI packages or SQLite. The application service depends on `ports.Store`, not on the concrete SQLite store. The server adapts HTTP into use cases and exposes local diagnostics; it does not become a second business-logic layer. The browser sends typed API calls and displays responses; it does not decide whether a transition is legal.

## Ownership and boundaries

| Concern | Owner | Not owned by |
|---|---|---|
| Entity state and allowed transitions | `internal/domain` | React components or SQL handlers |
| Use cases, relationships, orchestration | `internal/application` | HTTP handlers or page files |
| Command metadata and shared form contract | `internal/protocol` | separate copies in each frontend |
| Reusable validation and normalization | `internal/validation` | per-page ad-hoc checks only |
| Persistence, constraints, migrations | `internal/storage/sqlite` behind `internal/ports` | UI or protocol |
| HTTP routing, request boundaries, local health | `clients/server` | domain workflows |
| Visual primitives and design tokens | `clients/web/src/components/ui` and `index.css` | individual pages inventing their own styles |
| Shared graph canvas/inspector/layout | `clients/web/src/components/graph` and `lib/graph` | `GraphPage` |
| Logs, filtering and process snapshot display | `internal/diagnostics`, `/api/monitoring`, `LogsPage` | feature modules logging ad-hoc files |

## Validation flow

For protocol commands, validation is the first executable boundary after resolving the command ID:

1. Resolve the command from the known catalog. Unknown commands fail immediately.
2. Reject unknown input keys (sorted in the error for reproducibility).
3. Normalize and validate each supplied value: type, size, UTF-8/null bytes, enum choices, dates, times, color and number/boolean formats.
4. Report missing/empty required fields with human-readable labels.
5. Confirm the application service is available.
6. Run the use case. The application validates invariants again before storage, so callers that bypass the protocol cannot bypass domain-facing checks.
7. Storage constraints and transactions provide the final persistence boundary.

Typed HTTP JSON bodies are size-limited, must contain exactly one JSON value and reject unknown fields. Dynamic select values are type-checked centrally and then verified against real entities in the application/store. Validation errors should name the field and expected format; never log secrets or full private text values.

## Reusable web design system

- `components/ui/primitives.tsx` defines consistent `Button`, `Card`, `Badge`, `Field`, headings, and error/empty/loading states.
- `components/ui/Modal.tsx` is a content-agnostic modal shell. Pages pass content and actions as children instead of implementing custom backdrops, escape handling, focus semantics, or body-scroll locking.
- `index.css` owns design tokens (surface, border, text, accent, semantic colors, spacing and typography). Components use tokens; they should not define new one-off hex colors unless representing entity-specific data.
- Feature-specific components belong under `components/<feature>/`. A page should compose components and coordinate API state, not contain reusable graphics, form infrastructure, or a second copy of a workflow.
- Use a shared component when two or more views implement the same interaction. Keep feature components focused and typed; avoid giant “do everything” components.

## Knowledge graph contract

The graph is a projection of relationships persisted by the Engine, not a separate source of truth. `internal/storage/sqlite/graph.go` builds nodes/edges from repositories, branches, projects, issues, pull requests, completions, concepts, labels, milestones, and explicitly graph-visible linked notes. Profile metadata, quotes, scheduled jobs and activity history are not graph nodes merely because they are visible elsewhere in the workspace. Add them only when the product defines a meaningful relationship and the graph builder can state what that edge means.

`GraphPage` owns filtering and API loading. `GraphCanvas` owns zoom, pan, pointer dragging and local layout preferences. `GraphInspector` owns selected-node details and direct links. The initial placement is deterministic; user placement is saved in browser-local storage, not in the shared database. `depends_on` is the only relationship that blocks issue progress. Ownership, project membership, labels, milestones and semantic links are informational unless an application rule explicitly says otherwise.

## Operational diagnostics

`internal/diagnostics` owns the single user-local log file and safe serialization to it. The server request middleware records status and latency; `/api/monitoring` reports a local-only snapshot (database ping, process uptime, goroutine count, memory, HTTP totals and log-file metadata). `/api/logs` supports bounded reading, and `DELETE /api/logs` truncates the local history. The clear operation does not delete workspace data and its own successful request is not written back into the just-cleared file. No telemetry is sent to a remote service.

## License

This source tree already carries the **Mozilla Public License 2.0 (MPL-2.0)** in `LICENSE`. Keep that license consistent across source headers and release metadata. MPL-2.0 is file-level copyleft: modifications to MPL-covered files distributed to others must remain available under MPL terms, while larger applications can combine those files with separately licensed files. This is a pragmatic fit for an extensible desktop/local-first tool. Revisit the choice only if the project intentionally changes its source-distribution or contribution model; this note is engineering guidance, not legal advice.

## Change checklist

When changing behavior:

1. Update a domain/use-case contract first, or explain why no business behavior changes.
2. Apply reusable validation before state lookup or mutation where possible.
3. Update the store interface and SQLite migration in the same change if persistence changes.
4. Add tests for valid, invalid, boundary, and relationship cases.
5. Expose the use case through protocol/API; do not re-implement it in a page.
6. Build feature UI from shared primitives and focused components.
7. Update relationship docs and user-facing help when semantics change.
8. Run Go tests/vet/race checks, web lint/build, and migration/packaged smoke tests.

See [Adding a feature](EXTENDING.md), [Entity relationships](ENTITY_RELATIONSHIPS.md), [Storage and migrations](STORAGE.md), and [Web API notes](WEB_API.md).
