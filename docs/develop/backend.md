# Go backend development

The Go backend owns business behavior, local HTTP serving, storage access, AI orchestration, scheduled work, and the CLI adapter.

## Main directories

```text
cmd/idios/                 executable entry and server commands
clients/cli/               command-line adapter
clients/server/            HTTP server, routes, endpoint handlers
internal/domain/            domain records and invariants
internal/application/       use cases and domain services
internal/ports/             storage interfaces
internal/storage/sqlite/    SQL queries and database migrations
internal/ai/                provider contracts and model runtime
internal/protocol/           request/response contracts
internal/validation/         shared input validation
```

## Where code belongs

- Put business rules in Domain/Application, not in an HTTP handler or React component.
- Put SQL only in `internal/storage/sqlite/`.
- Keep each application service dependent on narrow storage ports.
- Keep HTTP handlers thin: parse, validate the transport shape, call an application service, return a response.
- Keep the CLI as another adapter over Application.
- Keep provider-specific HTTP and payload translation inside the AI adapter/runtime layer.

## Add or change a handler

1. Find the owning application service.
2. Add or change the operation and its tests.
3. Add the handler to the corresponding `handlers_<area>.go` file.
4. Register its path in `clients/server/server.go`.
5. Test valid requests, invalid requests, storage failures, and privacy restrictions.
6. Update `arch/http-api.md` or the relevant reference page.

## Run Go checks

```sh
go test ./...
go test -race ./...
go vet ./...
gofmt -w cmd internal clients
```

Run `make fmt-check` after formatting. Do not call a handler change complete if it bypasses application validation or writes SQL directly.
