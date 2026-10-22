# IDIOS

IDIOS helps you keep track of work in one place: repositories, issues, projects, notes, labels, and scheduled reminders. You can use it in a terminal or in a browser.

IDIOS stores its workspace in a SQLite database on your computer. It does **not** clone Git repositories, push code, or create real GitHub pull requests. An IDIOS pull request is a record of work and its review inside IDIOS.

## Install

**Windows (PowerShell):**

```powershell
irm https://github.com/m-mdy-m/idios/releases/latest/download/install.ps1 | iex
```

**Linux or macOS:**

```sh
curl -fsSL https://github.com/m-mdy-m/idios/releases/latest/download/install.sh | sh
```

These commands install the latest published release for your CPU. You do not need Go or Node.js to run a release build. To download an installer or portable archive yourself, visit [Releases](https://github.com/m-mdy-m/idios/releases/latest).

## Open IDIOS

```sh
idios       # open the terminal workspace
idios web   # open the browser workspace
idios help  # see commands
```

The browser workspace normally opens at `http://127.0.0.1:43170`. It runs on your own computer; the default address is not exposed to your network.

## What you can do

- Organize work in repositories, issues, sub-issues, and projects.
- Record blockers so an issue can depend on another issue.
- Track work with IDIOS pull-request and completion records.
- Save notes with multiple labels and connect them to other items.
- Schedule reminders and keep reusable concepts and links.
- Use the terminal UI, browser UI, or CLI commands.

## Your data

The default database is `~/.idios/idios-engine.sqlite3` (`%USERPROFILE%\.idios\idios-engine.sqlite3` on Windows). The file is created when IDIOS first opens the workspace. A new workspace starts empty.

## Guides

- [Install and start IDIOS](docs/user/GETTING_STARTED.md)
- [How the workspace works](docs/user/WORKSPACE_GUIDE.md)
- [CLI commands](docs/user/CLI_REFERENCE.md)
- [Settings and data files](docs/user/SETTINGS.md)
- [Build, run, and test from source](docs/development/README.md)
- [Contributing changes](CONTRIBUTING.md)
- [Security](SECURITY.md) · [Code of Conduct](CODE_OF_CONDUCT.md) · [Changelog](CHANGELOG.md)
