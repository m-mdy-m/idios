# Configuration reference

## Environment variables

| Variable | Purpose |
|---|---|
| `IDIOS_DB` | Default SQLite database path. |
| `IDIOS_CONFIG` | Configuration JSON path. |
| `IDIOS_DEMO_DB` | Separate database path for demo-only operations. |
| `IDIOS_LOG_LEVEL` | Enable/configure application log verbosity where supported. |

## Default paths

| File | Linux / macOS | Windows |
|---|---|---|
| Workspace database | `~/.idios/idios-engine.sqlite3` | `%USERPROFILE%\.idios\idios-engine.sqlite3` |
| Configuration | `~/.config/idios/config.json` or `$XDG_CONFIG_HOME/idios/config.json` | `%USERPROFILE%\.config\idios\config.json` |

`IDIOS_CONFIG` overrides the default config path. A legacy `~/.idios/config.json` is read when the new path is absent. JSON files under the sibling `config.d/` directory are merged in lexical filename order.

## Server defaults

- Host: `127.0.0.1`
- Port: `43170`
- Browser theme: dark by default for new Web settings.

The server's configured host/port determine the local URL. Do not bind to a public interface unless authentication and network exposure have been deliberately reviewed.

## AI provider privacy

Provider endpoint, API key, model, and privacy settings are managed under Settings → AI. Do not put API keys in committed JSON examples. For model catalogue refresh, see [provider refresh](provider-refresh.md).
