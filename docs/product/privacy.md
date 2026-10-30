# Privacy and local data

IDIOS is designed to keep the workspace on your computer.

## Local workspace data

- Repositories, Issues, Projects, Notes, Quotes, Plans, and other workspace records are stored in the local SQLite database.
- The Web server binds to `127.0.0.1` by default. This makes it available to programs on your computer, not to other devices on your network.
- Shelf connects folders that you choose. The original PDF files remain in those folders; IDIOS stores book metadata, progress, and annotations in its database.

The [Settings and data guide](../user/settings-and-data.md) explains default paths and backups.

## AI providers

A local provider such as Ollama can run on your computer. A hosted provider may receive the prompt and context that IDIOS sends to it. Review the provider's own privacy policy before using it with private material.

The AI privacy modes have different behavior:

- **Offline-only:** remote provider discovery and remote inference are blocked.
- **Local-first:** local inference is preferred. Remote catalogue discovery requires an explicit provider selection, and remote inference requires the cloud fallback option.
- **Cloud-first:** remote providers may be used when selected and configured.

Refreshing a model catalogue is provider-scoped. IDIOS asks only the provider you choose; it does not refresh every configured provider at once.

## Make a backup

Stop IDIOS before copying the database file, or use the application's supported backup/export operation when available. Keep a backup in a private location. Do not include API keys or private database files in a public bug report.
