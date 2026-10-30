# Service boundaries

The goal of the service structure is simple: changing one feature should not require editing unrelated features.

## Go application

- `internal/domain/` contains shared business types and domain rules.
- `internal/application/<area>.go` contains one service per business area.
- `internal/ports/` defines the storage contracts each service needs.
- `internal/application/service.go` composes the services and preserves a compatibility facade for existing callers.
- `clients/server/handlers_<area>.go` translates HTTP input/output for a domain area.
- `clients/server/server.go` owns listener lifecycle, route registration, middleware, and shared HTTP behavior.
- `internal/storage/sqlite/` owns SQL queries and migrations.

A handler should not implement business policy. A service should depend on the narrowest persistence port that it needs, not the entire storage implementation.

## Web application

```text
clients/web/src/
├── app/              # app startup, routes, providers
├── design-system/    # shared visual primitives and tokens
├── features/         # feature-owned pages and behavior
├── lib/              # shared technical helpers
├── styles/           # global reset and named style domains
└── types/            # shared wire/domain types
```

A feature should export its stable surface from `features/<name>/index.ts`. Do not import another feature's internal page, hook, CSS, or component directly.

## What is shared?

- Reusable buttons, tiles, inputs, dialogs, and layout primitives belong in `design-system/`.
- Shared HTTP transport and cross-cutting browser helpers belong in `lib/`.
- Business validation and state transitions belong in Go services.
- Provider-specific AI behavior belongs in an AI adapter, not a Shelf component.

## Why not a process per feature?

IDIOS currently uses one local executable and one database. Turning every UI area into a process would add networking, deployment, monitoring, versioning, and data-consistency costs without giving a single-user local app a clear benefit. Keep boundaries explicit inside the process first. Split a service only when there is a concrete operational requirement.

## Boundary checks

The Web build runs `check-feature-boundaries.mjs` and `check-service-boundaries.mjs`. A change that introduces a private cross-feature import or bypasses a declared service boundary should fail before Vite builds the app.
