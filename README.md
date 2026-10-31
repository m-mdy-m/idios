<p align="center">
  <img src="clients/web/public/brand/idios-wordmark.svg" alt="IDIOS" width="260">
</p>

<p align="center">A local-first workspace for learning, planning, and development.</p>

IDIOS brings your projects, repositories, issues, reading library, notes, plans, and AI tools into one private workspace. It provides a command-line interface and a Web workspace that can be installed as a PWA; optional Android and iOS shells connect to an HTTPS IDIOS deployment.

> IDIOS is an application, not an operating-system kernel. “Work OS” describes the workspace it provides.

## Start using IDIOS

### Install a released version

- **Linux / macOS:**

  ```sh
  curl -fsSL https://raw.githubusercontent.com/m-mdy-m/idios/main/scripts/install.sh | sh
  ```

- **Windows PowerShell:**

  ```powershell
  irm https://raw.githubusercontent.com/m-mdy-m/idios/main/scripts/install.ps1 | iex
  ```

The installer downloads the latest release, checks its SHA-256 checksum, and installs the single `idios` executable. It also creates a native launcher: a desktop/menu shortcut on Linux and Windows, and an `IDIOS.app` launcher on macOS. You can also download an installer manually from [GitHub Releases](https://github.com/m-mdy-m/idios/releases).

### Open the workspace

```sh
idios serve --open
```

IDIOS starts its local service when needed and opens the browser. The default address is `http://127.0.0.1:43170`. You can also double-click the installed IDIOS launcher instead of opening a terminal.

To run the service without opening a browser:

```sh
idios serve --no-open
```

To check the service:

```sh
idios server status
```

### Try the CLI

```sh
idios help
idios version
idios repo create --name "Learning Lab"
idios repo list
```

## Mobile and PWA

The Web client includes an installable PWA shell. Android and iOS Capacitor build targets are available for device-specific packaging. Native builds require `IDIOS_MOBILE_SERVER_URL` to point to an HTTPS IDIOS deployment protected by your authentication/reverse-proxy layer. The current Go service remains loopback-only and is not embedded into the mobile shell. See [Mobile and PWA](docs/user/mobile.md) before building or distributing a mobile package.

## What can I do with IDIOS?

| Area | What it is for |
| --- | --- |
| Repositories, Issues, Pull Requests | Organize work and record progress. These are IDIOS records, not Git operations. |
| Projects | Group related Issues across Repositories. |
| Shelf | Connect a local book folder, read PDFs, and keep reading notes and highlights. |
| Notes and Quotes | Keep reusable thoughts and excerpts. |
| Concepts and Graph | Connect ideas and inspect relationships. |
| Daily Plans | Turn a day into a checklist and track completion. |
| Jobs and Actions | Schedule supported local actions and run workspace operations. |
| Activity and Notifications | Review workspace history and reminders. |
| AI Runtime | Connect local or remote model providers and configure reading agents. |

## Learn IDIOS

Start with the [documentation index](docs/README.md). New users should read:

1. [Install and open IDIOS](docs/user/getting-started.md)
2. [Understand the workspace](docs/user/workspace.md)
3. [Use Shelf and read a PDF](docs/user/shelf.md)
4. [Connect AI providers](docs/user/ai.md)

## Build from source

Requirements: Go 1.25 or newer, Node.js 22 or newer, pnpm 9, a C compiler for SQLite, and GNU Make. On Windows, use PowerShell and a working CGO C toolchain.

```sh
git clone https://github.com/m-mdy-m/idios.git
cd idios
make build
```

The executable is written to `dist/`. See the [developer guide](docs/develop/README.md) for the complete setup and test commands.

## Data and privacy

IDIOS stores workspace data in a local SQLite database. The web server binds to the loopback interface by default; it is not exposed to your network unless you deliberately change the server configuration. AI provider connections are configured by you. The [AI guide](docs/user/ai.md) explains provider selection and privacy modes.

## Project status

IDIOS is actively developed. Some modules are more mature than others. Treat an operation as complete only when IDIOS confirms that it was saved; AI-generated text is not proof that a change was persisted.

- [Documentation](docs/README.md)
- [Changelog](CHANGELOG.md)
- [Contributing](CONTRIBUTING.md)
- [Security](SECURITY.md)
- [Code of Conduct](CODE_OF_CONDUCT.md)
- [License](LICENSE)
