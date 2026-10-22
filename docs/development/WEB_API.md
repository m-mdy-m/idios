# Browser API notes

The browser interface is a React + TypeScript application under `clients/web/`. It talks to the Go server through `/api`; the server routes actions to the shared application service. The default server address is `http://127.0.0.1:43170`.

## Change request or response data

1. Find the relevant use case in `internal/application/`.
2. Check how `clients/server/` exposes that operation.
3. Update the typed request/response definitions and API helper in `clients/web/src/` if the payload changes.
4. Update the screen that uses the operation.
5. Add or update Go tests for behavior and browser checks for the user-visible path.

The frontend must not access SQLite or independently decide whether a workflow transition is allowed. Put the rule in the Go application service and expose it through the API.

## Run the browser UI while developing

From the repository root, start `go run ./cmd/idios serve --no-open` in one terminal. In another terminal, run `pnpm install` and `pnpm dev` from `clients/web/`. Open the URL printed by Vite, normally `http://127.0.0.1:5173`.

`make build` creates the production frontend and embeds it in the `idios` executable. Release users do not need Node.js or a separate web server.
