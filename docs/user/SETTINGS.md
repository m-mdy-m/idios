# Settings, themes and shortcuts

The TUI preferences file is normally `~/.idios/config.json`. The first TUI launch creates it from the packaged defaults. Set `IDIOS_CONFIG=/path/to/config.json` to use another location. Changes made in the Settings screen are saved immediately.

Product UI wording and help copy are packaged with the program in `tui/content/en.json`; they are source resources for maintainers, not user preferences and are not copied into the user's config file.

Repository, Issue and Pull Request starter templates are different: they are editable configuration, because the user benefits from shaping how new work is documented. See [Repository, Issue and Pull Request templates](ISSUE_AND_PR_TEMPLATES.md) for the complete guide.

## Themes and colors

Choose **Settings → Next theme** to cycle through the packaged themes (`midnight`, `paper`, `forest`). Choose **Toggle density** to switch between comfortable and compact spacing.

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
| `accent_background` | Active controls and focused inputs |
| `surface`, `surface_alt` | Control and secondary-surface backgrounds |
| `border` | Panels and separators |
| `success`, `success_background` | Completed/healthy states |
| `warning`, `warning_background` | Attention states |
| `danger`, `danger_background` | Blocking/error states |

You do not need to change Go files to change a theme. Hex values live in the packaged defaults and the user config override. `NO_COLOR`, `TERM=dumb`, or the TUI `--no-color` flag disables ANSI styling.

**Fonts:** the terminal application controls the font family and font size. IDIOS currently configures theme colors and spacing rather than installing or bundling fonts.

## Startup and profile

- `startup.show_intro`: show the Intro screen on launch.
- Overview profile fields: username, display name, bio and README are saved in the local SQLite database, not in `config.json`.
- Important pins: repositories, Issues and other supported graph/workspace items can be pinned from their detail views.

## Notifications and scheduled jobs

Use **Settings / Notifications** to enable or disable inbox notifications and set the polling interval. The polling interval cycles through 1, 5, 15 and 60 minutes. The **Scheduled jobs** page lets you create reminders on hourly, daily, weekly or 30-day monthly intervals, enable/pause them, and choose reminder types.

Important limitation: jobs are evaluated while the TUI is running. Closing IDIOS stops its in-app scheduler. These are not registered with Windows Task Scheduler, cron or a remote notification service. The notification inbox is local SQLite data, not a desktop push notification.

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
| Load demo data | `Ctrl+D` |
| Quit | `q` or `Ctrl+C` |

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

The settings are merged with defaults, so omitted shortcuts retain their defaults. F6 or Ctrl+Tab focuses visible controls; Up/Down moves the focus and Enter activates it. Tab/Shift+Tab changes sections. If two actions are assigned the same key, the first action in IDIOS's documented shortcut priority wins. Use a unique key per action for predictable behavior.
