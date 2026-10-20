# Settings, themes and shortcuts

The TUI preferences file is normally `~/.idios/config.json`. The first TUI launch creates it from the packaged defaults. Set `IDIOS_CONFIG=/path/to/config.json` to use another location. Changes made in the Settings screen are saved immediately.

Product UI wording and help copy are packaged with the program in `tui/content/en.json`; they are source resources for maintainers, not user preferences and are not copied into the user's config file.

Repository, Issue and Pull Request starter templates are different: they are editable configuration, because the user benefits from shaping how new work is documented. See [Repository, Issue and Pull Request templates](ISSUE_AND_PR_TEMPLATES.md) for the complete guide.

## Themes and colors

Choose **Settings → Next theme** to cycle through the packaged themes (`midnight`, `ocean`, `ember`, `forest`, `paper`, `contrast`). Choose **Toggle density** to switch between comfortable and compact spacing.

To override individual tokens, edit the `appearance.colors` object in the config file. For example:

```json
{
  "appearance": {
    "theme": "forest",
    "density": "comfortable",
    "colors": {
      "accent": "#ff78a9",
      "accent_background": "#422038",
      "success": "#74e3ae",
      "danger": "#ff6767"
    }
  }
}
```

The user config is merged on top of packaged defaults, so a small file can contain only the fields you want to override. A token override applies on top of the selected theme until it is removed from `appearance.colors`.

Supported semantic color tokens:

| Token | Used for |
| --- | --- |
| `background` | Full-screen background |
| `foreground` | Main text and controls |
| `muted` | Supporting text |
| `accent` | Brand, interactive highlights and focus |
| `accent_background` | Current tab and primary buttons |
| `on_accent` | Text drawn on `accent_background` (optional; defaults to `foreground`) |
| `surface`, `surface_alt` | Control and secondary-surface backgrounds |
| `border` | Panels and separators |
| `success`, `success_background` | Completed/healthy states |
| `warning`, `warning_background` | Attention states |
| `danger`, `danger_background` | Blocking/error states |

The packaged themes are always loaded from the application, so edits to them inside `config.json` are ignored; put personal color changes in `appearance.colors`, or add a theme with a new name under `themes`. You do not need to change Go files to change a theme. Hex values live in the packaged defaults and the user config override. `NO_COLOR`, `TERM=dumb`, or the TUI `--no-color` flag disables ANSI styling.

**Fonts:** the terminal application controls the font family and font size. IDIOS currently configures theme colors and spacing rather than installing or bundling fonts.

## Startup and profile

- `startup.show_intro`: show the Intro screen on launch.
- Overview profile fields: username, display name, bio and README are saved in the local SQLite database, not in `config.json`.
- Important pins: repositories, Issues and other supported graph/workspace items can be pinned from their detail views.

## Notifications and scheduled jobs

Use **Settings / Notifications** to enable or disable inbox notifications and set the polling interval. The polling interval cycles through 1, 5, 15 and 60 minutes. The **Scheduled jobs** page lets you create, edit, pause/resume, run now or delete reminders. Supported schedules are intervals (`30m`, `1h`, `2d`, `1w`), a one-time local date/time (`YYYY-MM-DD HH:MM`), daily (`09:00`), weekly (`Mon 09:00`) and monthly (`15 09:00`). Monthly dates beyond a month’s length use that month’s final day. A one-time reminder completes after its first run; edit it to schedule it again.

Important limitation: jobs are evaluated while the TUI is running. Due jobs from while IDIOS was closed are processed when it starts again, but closing IDIOS stops the in-app scheduler. These are not registered with Windows Task Scheduler, cron or a remote notification service. Scheduled-job messages are recorded in the local SQLite notification inbox; they do not trigger native desktop popups.

### Random quote reminders

IDIOS seeds the local Quotes library with a curated starter collection of short quotations from software engineering, programming, computing, mathematics and statistics. Each curated item stores its author, original work/publication, category and an HTTPS reference link; open **Quotes** to inspect the attribution. Existing personal quotes are preserved. Seeding is idempotent and will not create another copy when the same quote text is already present.

Open **Settings → Notifications** (or **Settings → Random quote notifications**) to:

- enable or disable random quote reminders independently from scheduled-job notifications;
- cycle the random interval range through **1–5**, **15–60**, **30–120**, **60–240** or **180–720 minutes**;
- enable/disable native desktop delivery and send a test notification;
- see the next scheduled reminder time.

New installations default to reminders enabled, desktop delivery enabled, and a random interval between **60 and 240 minutes**. Consecutive reminders avoid choosing the same quote when the library contains more than one. Each reminder is also recorded in the local notification inbox. On Linux, native delivery uses `notify-send` from `libnotify`; install that command and use a desktop session with notifications enabled. macOS uses `osascript` notifications. Windows uses a PowerShell `NotifyIcon` balloon. If native delivery is unavailable or blocked by system settings, the app reports that failure and keeps the reminder in its inbox when the database write succeeds.

Quote reminders use the same lifecycle limitation as scheduled jobs: **IDIOS must remain open** for the randomized timer to fire. They are not registered as operating-system background tasks and do not wake a closed application. The next due time is persisted in the configured IDIOS config file (normally `~/.idios/config.json`); while IDIOS is closed, no desktop alert is sent. For a custom range beyond the built-in presets, add an override like this to `config.json`:

```json
{
  "notifications": {
    "quote_reminders": {
      "enabled": true,
      "desktop_enabled": true,
      "min_interval_minutes": 45,
      "max_interval_minutes": 180
    }
  }
}
```

## Logs and debug mode

Open **Logs** from Settings. Important info and error events are always recorded; verbose `DEBUG` events are written only while Debug is enabled. The default file is `~/.idios/logs/idios-tui.log` (or the `logs` folder beside a custom `IDIOS_CONFIG`). The log page shows recent lines. Sensitive user content should not be copied into logs or bug reports without review.

## Keyboard shortcuts

The default bindings are:

| Action | Default |
| --- | --- |
| New item | `Ctrl+N` |
| Search | `/` |
| Help | `?` |
| Overview / Repositories / Projects | `1` / `2` / `3` |
| Issues / Pull requests | `4` / `5` |
| Knowledge graph / Concepts | `g` / `7` |
| Settings | `8` |
| Profile / edit profile | `Ctrl+P` / `Ctrl+Shift+P` |
| Theme / density | `Ctrl+T` / `Ctrl+Shift+D` |
| Debug / notifications / Intro | `Ctrl+Shift+L` / `Ctrl+Shift+I` / `Ctrl+Shift+O` |
| Change reminder poll interval | `Ctrl+Shift+E` |
| New Note / Quote / Job | `Ctrl+Alt+N` / `Ctrl+Alt+Q` / `Ctrl+Alt+J` |
| Mark notifications read | `Ctrl+Alt+M` |
| Notes / Quotes | `Ctrl+Shift+N` / `Ctrl+Shift+Q` |
| Scheduled jobs | `Ctrl+J` |
| Notifications | `Ctrl+Shift+M` |
| Logs | `Ctrl+L` |
| Edit profile | `Ctrl+P` |
| Refresh | `r` |
| Command palette | `Ctrl+K` |
| Bulk Issues / quotes / scheduled jobs | `Ctrl+Alt+I` / `Ctrl+Alt+U` / `Ctrl+Alt+B` |
| Quit menu | `q` or `Ctrl+Q`; `Ctrl+C` remains an emergency graceful exit |

Change these in the `keyboard` object of `config.json`, for example:

```json
{
  "keyboard": {
    "overview": "0",
    "graph": "G",
    "notes": "ctrl+alt+n"
  }
}
```

The settings are merged with defaults, so omitted shortcuts retain their defaults. The top navigation favors Overview, Repositories, Issues and Jobs. Use the **MORE** rail or searchable **Command Palette** (`Ctrl+K`) to reach supplementary sections. The palette remains available on narrow terminals, but will not interrupt a form or a running batch. Bulk creation accepts one non-empty line per Issue title, quote or scheduled-job name; shared fields apply to the whole batch. IDIOS previews the count before writing, reports each item separately, and can retry failed items without resubmitting the successful ones. Each batch is limited to 100 items. The quit menu is available with `q`, `Ctrl+Q` or the **Exit IDIOS** action where shown; `Ctrl+C` remains an emergency graceful-exit path. Exiting during a batch waits for the current write, then skips unstarted items safely.

Demo data is no longer an in-app action: launch `idios-tui-demo` to use the separate sample database. F6 or Ctrl+Tab focuses visible controls; Up/Down moves the focus and Enter activates it. Tab/Shift+Tab changes sections. If two actions are assigned the same key, the first action in IDIOS's documented shortcut priority wins. Use a unique key per action for predictable behavior.
