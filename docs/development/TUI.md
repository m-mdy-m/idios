# TUI implementation

The TUI is a full-screen Bubble Tea v2 application with Lip Gloss v2 styling, keyboard navigation and clickable hit targets. It is a presentation adapter, not a shell around the CLI. It calls `internal/application.Service` and never issues SQL.

## Run and build

```sh
go run ./cmd/idios-tui
# or
make build
./dist/idios-tui
```

Go 1.25+ and a working C toolchain are required. The default database is `~/.idios/idios-engine.sqlite3`; preferences are in `~/.idios/config.json`. `IDIOS_DB`, `--db PATH`, `IDIOS_CONFIG` and `--no-color` can be used to override defaults where supported.

## Responsibilities by file

- `model.go`: typed page/form state and the shared data snapshot.
- `update.go`: event loop, keyboard shortcuts, mouse events, snapshot updates and scheduled-job ticks.
- `navigation.go`: back-stack behavior, cursor/list selection and key-to-action mapping.
- `actions.go`: one place to route semantic UI actions and submit forms.
- `canvas.go`: layout primitives, responsive button wrapping and hitbox generation.
- `styles.go`: theme tokens resolved from `config.AppConfig`; renderers must not add hex colors directly.
- `view.go`: persistent nav, responsive chrome and page dispatch.
- `view_overview.go`, `view_details.go`, `view_workspace.go`: focused screen renderers.
- `scheduler.go`, `logging.go`: scheduler tick and local diagnostics.
- `content/en.json`: embedded editable-at-source UI prose, not end-user preferences.

## State and effects

`model` is the event-loop state holder. Reads and writes should be exposed as typed Application service operations. Use `tea.Cmd` for blocking calls, and send typed messages back to `Update` instead of mutating the model from a goroutine. Click targets must only be created for controls actually rendered in the visible viewport. Forms preserve entered values after validation errors.

`loadSnapshotCmd` refreshes workspace collections. Be conservative when adding new collections because a full reload currently refreshes all of them. Keep detail-page selection keyed by stable reference rather than slice index.

## Theme and copy policy

The active palette is `appearance.theme` merged with `appearance.colors` in the user config. Add semantic tokens to `internal/config/defaults.json`; reference token names from `styles.go`. Do not add literal hex colors in a view. `appearance.density` controls spacing. Terminal font family/size remain the user's terminal setting.

Human-facing help, intro, and reusable static labels belong in `tui/content/en.json`, parsed through `tui/content.go`. The `overrides` object is keyed by the English source phrase; changing a value there changes the shipped UI copy without adding it to the user preferences. Formatted display templates (for example, issue counts) also resolve their template through this map, so keep their `%d` / `%s` placeholders compatible with the Go formatting arguments. This resource is embedded in the executable and is not copied to `~/.idios/config.json`. Dynamic values, domain errors and data-driven names still originate from the model/application; do not use copy overrides to alter identifiers, keys or storage values. Update the user guide when changing defaults or visible behavior.

## Repository, Issue and Pull Request templates

The TUI reads template configuration through `internal/config.AppConfig`; it does not parse `config.json` independently. The packaged source of truth is `internal/config/defaults.json`, with the public keys `REPOSITORY_TEMPLATE`, `ISSUE_TEMPLATE`, and `PULL_REQUEST_TEMPLATE`. `LoadAppConfig` recursively overlays the user file on those defaults, so a custom Issue template can be added or one template can be overridden without copying every bundled theme, shortcut, or template.

- `REPOSITORY_TEMPLATE.description` seeds the Repository creation form's multiline overview. On submission, `{{repository}}` is replaced with the chosen name and the resulting text is stored as the Repository description.
- `ISSUE_TEMPLATE.default` selects the initial template key. `ISSUE_TEMPLATE.templates` maps stable lowercase keys to `{name, title_prefix, body}`. The first Issue field is a template selector; pressing Enter validates the key, loads the body and applies the title prefix. The final application call still goes through the normal Issue use case, so templates do not bypass parent/child or workflow rules.
- `PULL_REQUEST_TEMPLATE` provides `title_prefix` and `body`. The body seeds a multiline field and is saved as the explanation supplied to the ordinary Pull Request use case.

The UI form editor is implemented in `update.go`, `helpers.go`, and `view_details.go`. Keep multiline field values as one string with newline separators; the editor caret is a rune offset, not a byte offset. Paste normalizes CRLF/CR to LF; single-line fields convert pasted line breaks to spaces. `Enter` inserts a newline inside multiline fields, while `Ctrl+Enter` / `Ctrl+S` submits. Template selection and submission must remain keyboard-operable. When adding a new template capability, test config merge behavior in `internal/config/app_test.go` and form behavior in `tui/app_test.go`.

Templates are local text scaffolds. Do not describe them as creating GitHub's `.github` files, Git commits, branches, or remote Pull Requests; IDIOS currently stores the resulting prose in its local SQLite records. User instructions and JSON examples live in `docs/user/ISSUE_AND_PR_TEMPLATES.md`.

## Keyboard and interaction

Shortcut bindings are user-configurable in the `keyboard` object. `Update` dispatches a configured action before applying normal navigation keys. Avoid ambiguous duplicate bindings; the action order in `shortcutAction` defines precedence. Every primary destination should remain usable without a mouse.

## Local scheduling and logging

` scheduler.go` processes due jobs while the TUI event loop is alive. It must not imply that work continues after application exit. Logs are written below the config directory (`logs/idios-tui.log`) with restrictive permissions. INFO/ERROR entries are always recorded; DEBUG entries should be guarded by `config.Debug`.

## Tests

Add view tests for visible labels/empty states, keyboard tests for shortcuts, action tests for navigation and forms, and storage/Application tests for domain behavior. When a screen changes, verify that the Overview remains usable at narrow terminal widths and that the CLI remains independent of Bubble Tea.
