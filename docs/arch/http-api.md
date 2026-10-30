# HTTP API overview

The HTTP server is an adapter over the Go application. The browser client talks to this API; it does not read or write the SQLite database directly.

## API areas

The API is grouped around workspace domains: repositories, projects, issues, pull requests, graph/concepts, notes/quotes, Shelf, plans/planning, jobs, activity, notifications, settings, diagnostics, AI, and updater operations.

The exact route handlers live in `clients/server/handlers_*.go`, `clients/server/ai_api.go`, and related files. Route registration and shared middleware live in `clients/server/server.go`.

## Protocol behavior

Mutating requests are validated at the protocol and application boundaries. Unknown fields, invalid values, malformed references, unsupported choices, and over-sized input should be rejected before persistence.

The CLI and Web adapters call the same application services, so a state transition should not be implemented only in one client.

## When adding an endpoint

1. Identify the owning domain service and its persistence port.
2. Implement or reuse the application operation.
3. Add an HTTP handler in the owning handler file.
4. Register the route in `server.go`.
5. Add handler and application tests for success and failure cases.
6. Update the route reference and any user guide affected by the operation.
