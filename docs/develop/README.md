# Developer guide

This guide starts at the beginning. It is for someone who has just downloaded or cloned IDIOS and has never built the project before.

## 1. What you are building

IDIOS is one Go executable. The executable contains the CLI, the local HTTP server, and the built Web interface. You do not need a separate server process or a database server.

The Web source is built with Node.js and pnpm. Go builds the final executable. SQLite is included through the Go SQLite driver, which needs a C compiler because it uses CGO.

## 2. Install the tools

Install these tools before building:

| Tool | Required version | Why it is needed |
|---|---|---|
| Git | Current version | Downloads the source code. |
| Go | 1.25 or newer | Builds the CLI and local server. |
| Node.js | 22 or newer | Builds the Web interface. |
| pnpm | 9.15.4 | Installs the Web packages. |
| C compiler | GCC or Clang | Required by SQLite/CGO. |
| GNU Make | Current version | Runs the project commands. |
| Python | 3.12 or newer | Runs the shared pipeline helper. |

Official installation pages:

- [Git](https://git-scm.com/downloads)
- [Go](https://go.dev/dl/)
- [Node.js](https://nodejs.org/en/download)
- [pnpm](https://pnpm.io/installation)
- [MSYS2 for Windows](https://www.msys2.org/)
- [Python](https://www.python.org/downloads/)

### Windows C compiler

IDIOS needs GCC available in PowerShell's `PATH` when CGO is enabled. One option is MSYS2 UCRT64:

1. Install MSYS2 using the official installer.
2. Open **MSYS2 UCRT64** from the Start menu.
3. Run:

   ```sh
   pacman -Syu
   pacman -S --needed base-devel mingw-w64-ucrt-x86_64-gcc
   ```

4. Add `C:\msys64\ucrt64\bin` to the Windows user `PATH`.
5. Open a new PowerShell window and check `gcc --version`.

On Linux, install GCC and Make through your distribution's package manager. On macOS, install Xcode Command Line Tools with `xcode-select --install`.

### Enable pnpm

Node.js includes Corepack in supported distributions. Run:

```sh
corepack enable
corepack prepare pnpm@9.15.4 --activate
```

Open a new terminal if the `pnpm` command is not found.

## 3. Check your installation

Open a new terminal and run:

```sh
git --version
go version
node --version
pnpm --version
gcc --version
make --version
python --version
```

Expected minimum versions are listed in the table above. On macOS, `clang --version` can be used instead of `gcc --version`.

## 4. Download the source

```sh
git clone https://github.com/m-mdy-m/idios.git
cd idios
```

All commands in this guide are run from the `idios` repository folder unless a step says to enter another directory.

## 5. Build IDIOS

Run:

```sh
make build
```

The first build installs the Web dependencies, checks the Web source, builds the Web files, and then compiles the Go executable.

The result is:

```text
dist/
  idios       # Linux and macOS
  idios.exe   # Windows
```

If the command fails, read the first error in the output. Later errors are often consequences of that first failure. See [Troubleshooting](../user/troubleshooting.md) before changing build scripts.

## 6. Start IDIOS

Build and start the exact executable from the current source:

```sh
make run-built
```

Or start the server directly from Go without building the final executable:

```sh
go run ./cmd/idios serve --no-open
```

Open this address in your browser:

```text
http://127.0.0.1:43170
```

Stop the foreground server with `Ctrl+C`. For the installed background service, use `idios server stop`.

## 7. Run checks

Start with the checks related to the code you changed:

```sh
make fmt-check
make test
make vet
make web-lint
```

Run the full local gate before opening a pull request:

```sh
make ci
```

The full gate includes Go tests, race tests, Web checks/build, and a local smoke test. See [Testing](testing.md) for focused commands and what each check proves.

## 8. Use the shared pipeline

IDIOS uses the separate [`m-mdy-m/pipeline`](https://github.com/m-mdy-m/pipeline) repository for the local task runner and reusable GitHub workflows.

With Python 3.12+ and Git installed, run:

```sh
python scripts/run-pipeline.py validate
python scripts/run-pipeline.py list
python scripts/run-pipeline.py run ci
```

The first command downloads the current `main` branch of the shared pipeline into `.tools/pipeline/`. The runner fetches `main` again before later executions. If the network is unavailable, an existing clone is reused and its cached commit is printed as a warning. This folder is ignored by Git. The task definitions remain in IDIOS's root `pipeline.toml`.

## 9. Find the code you need

| Path | What lives there |
|---|---|
| `cmd/idios/` | Program entry point and top-level commands. |
| `clients/cli/` | Terminal commands. |
| `clients/server/` | Local HTTP server and HTTP handlers. |
| `clients/web/` | React, TypeScript, and Vite source. |
| `internal/domain/` | Shared business records and rules. |
| `internal/application/` | Operations and services for each area. |
| `internal/ports/` | Interfaces used by application services. |
| `internal/storage/sqlite/` | SQLite queries and schema migrations. |
| `internal/ai/` | AI providers, model discovery, and generation. |
| `internal/features/` | Navigation and feature metadata. |
| `scripts/` | Build, install, verification, and release helpers. |
| `packaging/` | Linux, macOS, and Windows package definitions. |
| `pipeline.toml` | Local task list for the shared pipeline runner. |
| `docs/` | User, architecture, developer, and reference guides. |

## 10. Make a small change first

1. Find the page or command that shows the behavior.
2. Find the owning Web feature or Go application service.
3. Read the nearby tests before changing code.
4. Change one responsibility at a time.
5. Add a test for the expected behavior and at least one failure case.
6. Run the focused test, then the full checks that apply.
7. Update the guide and `CHANGELOG.md` if users will notice the change.

Read [Adding a feature](adding-a-feature.md) before introducing a new module or service.

## Read next

- [Build and run](build-and-run.md) — compile and start the application.
- [Testing](testing.md) — run checks and understand what they verify.
- [Adding a feature](adding-a-feature.md) — make a small, isolated change.
- [Backend development](backend.md) — work with Go services and HTTP handlers.
- [Web development](web.md) — work with React, TypeScript, and styles.
- [Storage](storage.md) — add SQLite migrations safely.
- [Pipeline](pipeline.md) — use the shared local and GitHub pipeline.
- [Releasing](releasing.md) — prepare a version and publish its packages.
