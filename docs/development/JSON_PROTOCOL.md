# Machine-readable CLI contract

A future Python GUI should initially use the Engine CLI's JSON output instead of sharing SQLite tables or reimplementing business logic. The command line is an adapter over the same Go Application use cases used by the TUI.

## Protocol metadata

```sh
idios protocol --json
```

returns `protocol_version: 1`, `application_version`, the global JSON flag and the recommended Issue/PR reference formats. Protocol V1 is currently a command/output contract, not a long-running network service or JSON-RPC daemon.

## Calls

```sh
idios --db /path/to/idios.sqlite3 --json repo list
idios --db /path/to/idios.sqlite3 issue show --id CPU#1 --json
idios issue list --repo CPU --db /path/to/idios.sqlite3 --json
```

Success writes one JSON document to stdout. Errors write a JSON envelope to stderr and exit non-zero:

```json
{
  "protocol_version": 1,
  "error": {
    "code": "blocked",
    "message": "CPU#1 is waiting for ..."
  }
}
```

Known error codes include `not_found`, `conflict`, `invalid_input`, `blocked`, `invalid_transition`, `unsupported` and `internal_error`. A client must use the exit status and `error.code`, not parse human-readable error messages to make workflow decisions.

Global flags `--db PATH` and `--json` can appear before or after command arguments. Do not parse the human-oriented default output in GUI code.

## Python client sketch

```python
import json
import subprocess


def idios(*args, db_path: str):
    proc = subprocess.run(
        ["idios", "--db", db_path, "--json", *args],
        text=True,
        capture_output=True,
        check=False,
    )
    if proc.returncode:
        envelope = json.loads(proc.stderr)
        raise RuntimeError(f"{envelope['error']['code']}: {envelope['error']['message']}")
    return json.loads(proc.stdout)

repos = idios("repo", "list", db_path="/path/to/idios-engine.sqlite3")
```

For interactive or high-volume clients, a future version may add a persistent local IPC protocol. Do not introduce it until process-per-command overhead is measured and a versioned request/response format is defined.
