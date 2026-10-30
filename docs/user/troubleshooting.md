# Troubleshooting

## The browser page does not open

1. Run `idios server status`.
2. If the service is stopped, run `idios server start` or `idios web`.
3. Open `http://127.0.0.1:43170` directly.
4. If the port is occupied, close the other local service or change the IDIOS server port in Settings/configuration.

## The CLI is not found

The installer places `idios` in a user-writable directory. Add that directory to your `PATH`, then open a new terminal. On Windows, reopen PowerShell after installation.

## The Web page shows old frontend code

Rebuild and run the executable from the current checkout:

```sh
make run-built
```

On Windows, `make run-built` uses `scripts/restart-built.ps1`. It stops only an IDIOS process that owns the configured local port; it does not stop unrelated applications.

## A PDF does not render

Update IDIOS to the latest build. The bundled PDF.js worker and decoder assets are produced by the Web build. If the original file moved, reconnect the Shelf folder and open the book again.

## AI model list did not change

Open **Settings → AI → Models**, press **Refresh catalogue**, choose the provider you want, and confirm. IDIOS intentionally does not refresh all providers at once. Under Offline-only, remote providers cannot be refreshed.

## A reading note or highlight is missing

Check that IDIOS showed a saved confirmation. Select the passage again and save it explicitly. If a model only returned a suggestion, that suggestion is not a saved annotation until the application confirms the write.

## Where is my data?

See [Settings and data](settings-and-data.md) for the database and configuration paths. Do not delete the database to fix a display or build problem; make a backup first.
