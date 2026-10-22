# Client applications

The client code turns IDIOS operations into commands and user interfaces. The shared workflow rules and database access live under `internal/`.

- `cli/` — command parsing and text/JSON output.
- `tui/` — full-screen terminal interface.
- `server/` — browser server and HTTP API adapter.
- `web/` — React + TypeScript browser interface.

The usual call path is `clients -> internal/application -> internal/ports -> internal/storage/sqlite`. Keep database access and workflow decisions out of client code so the interfaces behave consistently.
