# Getting started

This guide assumes you have never used IDIOS before.

## 1. Install IDIOS

Download the installer for your operating system from [GitHub Releases](https://github.com/m-mdy-m/idios/releases).

You can also use the small installer scripts:

### Linux or macOS

```sh
curl -fsSL https://raw.githubusercontent.com/m-mdy-m/idios/main/scripts/install.sh | sh
```

### Windows PowerShell

```powershell
irm https://raw.githubusercontent.com/m-mdy-m/idios/main/scripts/install.ps1 | iex
```

The installer checks the downloaded archive against its SHA-256 checksum before replacing the executable. It also installs a native launcher: a desktop/menu shortcut on Linux and Windows, and `IDIOS.app` on macOS.

## 2. Open IDIOS

Open a terminal and run:

```sh
idios serve --open
```

The local service starts and your browser opens at `http://127.0.0.1:43170` by default. You can also double-click the IDIOS desktop shortcut (or `IDIOS.app` on macOS) to do the same without a terminal.

To print the address without opening a browser:

```sh
idios serve --open --no-open
```

## 3. Create your first Repository

In IDIOS, a Repository is a workspace folder for related Issues and activity. It is not a Git clone.

1. Open **Repositories**.
2. Create a Repository, for example `Learning Lab`.
3. Open it and create an Issue such as `Read the IDIOS guide`.
4. Start the Issue when you begin working.
5. Create an IDIOS Pull Request to record what you did. Accept it when the work is complete.

You can also try the same flow from the terminal:

```sh
idios repo create --name "Learning Lab"
idios issue create --repo "Learning Lab" --title "Read the IDIOS guide"
idios issue list --repo "Learning Lab"
```

## 4. Explore the workspace

- **Projects** group Issues from multiple IDIOS Repositories.
- **Shelf** connects a folder of books on your computer.
- **Notes** stores your own writing.
- **Concepts** and **Graph** connect ideas.
- **Daily Plans** turn a date into a checklist.
- **Settings → AI** configures model providers and reading agents.

## 5. Stop the service

```sh
idios server status
idios server stop
```

To start it again:

```sh
idios server start
```

For more detail, see [Workspace](workspace.md), [Shelf](shelf.md), and [Settings and data](settings-and-data.md).
