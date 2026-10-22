# Install and start IDIOS

This guide is for someone who has been told to install IDIOS and has not used it before.

## 1. Install a release

Download a ready-to-run build from the [latest release page](https://github.com/m-mdy-m/idios/releases/latest). Choose a file for your operating system and CPU:

- **Windows:** use the `setup.exe` installer, or download the `.zip` for a portable copy.
- **Linux:** use the installer below, a `.deb` package on Debian/Ubuntu, an `.rpm` package on Fedora/RHEL, or the portable `.tar.gz` archive.
- **macOS:** use the `.pkg` installer, the `.dmg`, or the portable `.tar.gz` archive.

The release page contains builds for 64-bit Intel/AMD (`amd64`) and ARM (`arm64`) computers. You do not need to install Go, Node.js, or a database server to run a release build.

### Quick install on Windows

Open PowerShell and run:

```powershell
irm https://github.com/m-mdy-m/idios/releases/latest/download/install.ps1 | iex
```

The installer adds IDIOS to your user PATH. Close and reopen PowerShell if the `idios` command is not found immediately.

### Quick install on Linux or macOS

Open a terminal and run:

```sh
curl -fsSL https://github.com/m-mdy-m/idios/releases/latest/download/install.sh | sh
```

The default install location is `~/.local/bin`. If the terminal cannot find `idios`, open a new terminal or add that directory to your `PATH`.

## 2. Open the workspace

Run:

```sh
idios
```

This opens the full-screen terminal interface. Use it to browse or create repositories, issues, projects, notes, and other workspace items. Press `?` or `h` in the terminal interface to open its help.

A new database starts empty. IDIOS does not fill your normal workspace with example records.

## 3. Open the browser interface

Run:

```sh
idios web
```

IDIOS starts its background service if necessary and opens the browser. The default address is `http://127.0.0.1:43170`.

Useful server commands:

```sh
idios server status   # check whether the service is running
idios server stop     # stop the service
idios server install  # start the service and run it automatically at login
idios server uninstall # remove the automatic-start setting
```

The service listens on your own computer by default. It does not need to be exposed to your home or office network.

## 4. Learn the basics

1. Create a repository in IDIOS. This is a place to group work; it does not create a Git checkout.
2. Add an issue and describe the work.
3. Start the issue when you begin working on it.
4. Create an IDIOS pull-request record to describe the work, then accept it when the work is complete.
5. Use projects to group issues from different repositories, and notes to save useful context.

Read the [Workspace guide](WORKSPACE_GUIDE.md) for what these items mean and what IDIOS does not do.

## 5. Where your data lives

The default workspace database is:

- **Linux/macOS:** `~/.idios/idios-engine.sqlite3`
- **Windows:** `%USERPROFILE%\.idios\idios-engine.sqlite3`

The database holds your workspace items. Your settings are stored separately; see [Settings and data](SETTINGS.md).

## If something does not work

- **`idios` is not recognized / command not found:** reopen your terminal. If needed, check that the install directory is in `PATH`.
- **The terminal UI exits immediately:** it needs an interactive terminal. Run `idios help` for CLI usage.
- **The browser does not open:** run `idios web --no-open` to print the address, then open it yourself. Check `idios server status`.
- **You want to use a different database:** set `IDIOS_DB` or pass `--db PATH` to a supported command.
- **You want to report a bug:** open an issue at [GitHub Issues](https://github.com/m-mdy-m/idios/issues) and include the steps and error message. Do not attach your database if it contains private information.
