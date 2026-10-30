# Client adapters

- `cli/` — terminal commands and text/JSON output.
- `server/` — the local HTTP API and embedded Web asset handler.
- `web/` — the React + TypeScript interface, built to static assets and embedded in the Go executable.

The Go `internal/application` layer owns use cases and business workflows. The CLI and HTTP server call that layer. The browser uses the HTTP API and never accesses SQLite directly. IDIOS has no full-screen terminal UI; terminal use is through the CLI.
