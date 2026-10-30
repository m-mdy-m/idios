# Custom REST adapter reference

This sample demonstrates how an IDIOS fork can register a user-owned inference API as a provider adapter. It is deliberately not linked into the stock executable by default.

## Enable it in your own build

Add a blank import to the import block in `cmd/idios/server.go`:

```go
import _ "idios/examples/ai-adapter/customrest"
```

Then rebuild IDIOS. The adapter registers as `custom-rest-v1`, and the ID appears in **Settings → AI Runtime → Providers → Custom adapter extension**. It will only appear in builds that include the import.

## Protocol expected by this example

The configured base URL is a prefix, so `http://127.0.0.1:9000/v1` resolves to the endpoints below. Requests use JSON; when an API key is configured, IDIOS sends `Authorization: Bearer <key>`.

### `GET /models`

```json
{
  "models": [
    {
      "id": "personal-agent-q4",
      "name": "Personal OS Agent",
      "architecture": "GPT",
      "family": "nanogpt-custom",
      "parameter_size": "160M",
      "quantization_level": "Q4_K_M",
      "format": "custom",
      "size_bytes": 180000000,
      "context_size": 4096,
      "capabilities": {
        "text_generation": true,
        "streaming": true,
        "structured_output": true
      },
      "metadata": {
        "training_run": "personal-v1"
      }
    }
  ]
}
```

`id` is required. Unknown model-specific fields can live in `metadata`. Only advertise capabilities the server actually implements.

### `POST /generate`

Request:

```json
{
  "model": "personal-agent-q4",
  "messages": [{ "role": "user", "content": "Hello" }],
  "temperature": 0.2,
  "max_tokens": 256,
  "response_format": {
    "type": "json_schema",
    "name": "answer",
    "schema": { "type": "object", "properties": { "ok": { "type": "boolean" } } }
  }
}
```

Response:

```json
{ "model": "personal-agent-q4", "content": "{\"ok\":true}" }
```

### `GET /health`

Return any successful 2xx response when the inference service is healthy. A small JSON body such as `{"status":"ok"}` is fine.

### Optional `POST /generate/stream`

Enable this with `adapter_config: { "supports_streaming": true }`. The endpoint must return Server-Sent Events, one JSON object per `data:` line:

```text
data: {"delta":"Hello"}

data: {"delta":" world"}

data: {"done":true}
```

`{"content":"..."}` is also accepted for a chunk; `[DONE]` ends the stream. Do not advertise the `streaming` model capability if the service does not stream incrementally. With streaming disabled, IDIOS emits the completed generation as one final chunk.

## Adapter configuration

The optional `adapter_config` object supports:

- `models_path` (default `/models`)
- `generate_path` (default `/generate`)
- `health_path` (default `/health`)
- `stream_path` (default `/generate/stream`)
- `supports_streaming` (default `false`)

Copy this package into your fork or create a separate extension package when your provider has a different protocol. A custom adapter runs inside the IDIOS server process with the same OS permissions, so only compile trusted adapter code. Use **Offline-only** mode for policy enforcement when you do not want remote provider requests.
