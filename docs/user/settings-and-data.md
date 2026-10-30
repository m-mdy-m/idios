# Settings, files, and backups

Most preferences can be changed from the Web interface. You normally do not need to edit a configuration file by hand.

## Database

The default workspace database is:

- Linux / macOS: `~/.idios/idios-engine.sqlite3`
- Windows: `%USERPROFILE%\.idios\idios-engine.sqlite3`

Set `IDIOS_DB` to choose a different default path. Supported CLI commands also accept `--db PATH`.

The CLI and Web workspace use the same database when they use the same path.

## Configuration file

The default configuration path is:

- Linux / macOS: `~/.config/idios/config.json` (or `$XDG_CONFIG_HOME/idios/config.json` when set)
- Windows: `%USERPROFILE%\.config\idios\config.json`

Set `IDIOS_CONFIG` to use another configuration file. Older `~/.idios/config.json` settings are read as a compatibility fallback when the new file is absent. Optional JSON fragments in a sibling `config.d/` directory are merged in filename order.

The first launch creates default settings. The packaged Web theme defaults to a dark appearance.

## Server address

The server listens on `127.0.0.1:43170` by default. It binds to the loopback interface, so it is intended for access from the same computer. Change the host or port only when you understand the network exposure that results.

## Back up your workspace

1. Stop IDIOS before copying the database:

   ```sh
   idios server stop
   ```
2. Copy `idios-engine.sqlite3` to a private backup location.
3. Copy `config.json` as well if you want to preserve preferences, themes, provider settings, and templates.
4. Start IDIOS again with `idios server start` or `idios web`.

Do not delete the database as a troubleshooting shortcut. Reinstalling the executable and deleting workspace data are different operations.

## Remove data

Use the **Remove Items** feature for supported record deletion. Review the confirmation carefully. Removing an item is not the same as uninstalling the IDIOS executable.
