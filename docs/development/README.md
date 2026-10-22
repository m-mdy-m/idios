# Build and test IDIOS

This guide is for people who want to change IDIOS or run it from the source code. If you only want to use IDIOS, install a release instead; see [Install and start](../user/GETTING_STARTED.md).

## Requirements

- Go **1.25 or newer**.
- A C compiler supported by your Go setup. The SQLite driver uses CGO.
- Node.js **22 or newer** and pnpm **9.x** to build or lint the browser interface.
- Git and GNU Make for the convenient project commands.

You do not need Node.js to run a release build after it has been compiled.

## Get the source

```sh
git clone https://github.com/m-mdy-m/idios.git
cd idios
```

## Build

```sh
make build
```

The build command installs the web dependencies, compiles the TypeScript/React UI, embeds it in the Go executable, and writes binaries to `dist/`:

- `dist/idios` — CLI and terminal workspace (`dist/idios.exe` on Windows).
- `dist/idios-tui` — standalone terminal workspace.
- `dist/idios-tui-demo` — terminal workspace using a separate sample database.

On Windows, `make build` calls `scripts/build.ps1`. You can call that script directly from PowerShell if needed:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts/build.ps1 -Version dev
```

## Run the interfaces

### Terminal workspace

```sh
go run ./cmd/idios
```

This uses your normal IDIOS database. If you want to see the interface with sample data instead, use:

```sh
make tui-demo
```

The demo uses a different database and does not add sample records to the normal workspace.

### Browser workspace during development

The browser UI uses Vite for hot reload and the Go process for the API. Run these in two terminals from the repository root.

Terminal 1:

```sh
go run ./cmd/idios serve --no-open
```

Terminal 2:

```sh
cd clients/web
pnpm install
pnpm dev
```

Open the address printed by Vite, normally `http://127.0.0.1:5173`. Vite forwards `/api` requests to the Go service at `http://127.0.0.1:43170`.

For a production-style browser build, run `make build`, then start `dist/idios web`. That binary contains the compiled browser UI; no separate Node.js server is needed.

## Tests and checks

Run the checks that match your change:

```sh
make fmt-check    # check Go formatting
make test         # run Go tests
make vet          # run go vet
make test-race    # run tests with the race detector
make web-lint     # lint the browser UI
make build        # compile the app and browser UI
make smoke        # build, then test the packaged entry points
```

To run the full project check, use `make ci`. It runs the formatting check, Go tests, vet, browser lint, race tests, build, and smoke tests.

You can also run Go checks directly:

```sh
go test ./...
go vet ./...
go test -race ./...
```

Tests use temporary databases. Do not point tests at your personal workspace database.

## Install a build for your own user

`make install-user` builds IDIOS and installs its executables in a user-writable location. It is useful for trying a build as an installed command. To remove the binaries installed by this target, run `make uninstall`. These commands do not remove your workspace database.

## More developer information

- [Architecture](ARCHITECTURE.md)
- [Entity relationships](ENTITY_RELATIONSHIPS.md)
- [Adding a feature](EXTENDING.md)
- [Storage and migrations](STORAGE.md)
- [Web API notes](WEB_API.md)
- [Contributing](../../CONTRIBUTING.md)
