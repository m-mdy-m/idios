# Settings and data

You can adjust appearance, keyboard shortcuts, notifications, browser behavior, and templates from IDIOS settings where supported. Most people do not need to edit a file by hand.

## Workspace database

The database contains repositories, issues, projects, notes, jobs, and other workspace records. The default file is:

- **Linux/macOS:** `~/.idios/idios-engine.sqlite3`
- **Windows:** `%USERPROFILE%\.idios\idios-engine.sqlite3`

To use a different database, set `IDIOS_DB` before starting IDIOS, or pass `--db PATH` to supported commands. The TUI also accepts `--db PATH`.

## Configuration file

The configuration file stores preferences and templates. The default path is:

- **Linux/macOS:** `~/.config/idios/config.json`
- **Windows:** `%USERPROFILE%\.config\idios\config.json`

Set `IDIOS_CONFIG` to use another file. On systems that use `XDG_CONFIG_HOME`, IDIOS also respects that directory. An older `~/.idios/config.json` is still used when the newer file does not exist and the older file does.

The first launch creates a default configuration. You can leave it as-is and make changes through the interface.

## Browser service

The browser service listens on `127.0.0.1:43170` by default. Its host, port, browser-opening preference, and refresh interval are stored under the `server` section of the configuration. Restart the service after changing its listening address or port.

To check or control the service:

```sh
idios server status
idios server stop
idios server start
```

## Back up your workspace

1. Close the terminal interface and stop the background service with `idios server stop`.
2. Copy `idios-engine.sqlite3` to a safe location.
3. Copy `config.json` too if you want to preserve themes, shortcuts, and templates.
4. Keep the backup somewhere private; it may contain notes and work details.

Do not delete the database to fix a build or update problem. IDIOS applies database migrations when it opens the database. A program reinstall and a workspace-data reset are different things.

## Web themes and custom palettes

Open **Settings → Theme** to choose System, Dark, Light, Midnight, Forest, Violet, Rose, or OLED. To create a palette, use **Custom theme studio**: choose a name, select its Dark/Light base, adjust the color tokens, then choose **Add & preview theme**. This adds the theme to the current settings draft and applies it immediately. Click **Save changes** to persist the palette in your IDIOS configuration. Existing custom themes can be edited or removed from the same panel.

Custom theme names are converted to a stable lowercase slug for the configuration key. Each theme stores its base appearance, background, surface, text, muted text, accent, borders, success, warning, and danger colors. IDIOS validates these colors before saving. The custom palette is a web preference; it does not change TUI colors.

The Overview includes a quote spotlight that cycles through saved quotes with a smooth transition. Use the pause and next buttons to control it, copy the displayed quote, or open the complete quote library.
