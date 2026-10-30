# AI and Shelf architecture

Shelf is a feature that uses local file access, the Go application, persistent annotations, and optional AI assistance. These parts are separate so reading behavior does not depend on a model being available.

## Shelf reading path

1. The Web client selects a book and its local source files.
2. The local server validates the book reference and serves the requested PDF bytes.
3. PDF.js renders pages in the browser.
4. The reader stores progress and annotations through the HTTP API.
5. The Go application validates and persists those records in SQLite.

PDF files stay in their connected source folders. Book notes, questions, reading sessions, and PDF annotations are IDIOS records.

## Annotation rules

A highlight is a saved annotation. A note may be attached to the selected passage. The UI must only show a saved state after the application confirms persistence. AI output may propose an annotation, but the model response itself is not a write.

## AI runtime

- `internal/ai/` owns provider contracts, runtime management, discovery, generation, streaming, and provider adapters.
- `clients/server/ai_api.go` exposes the HTTP adapter for AI settings and requests.
- `clients/server/shelf_ai.go` coordinates Shelf-specific AI requests.
- `clients/web/src/features/ai-settings/` owns provider/model/agent settings.
- `clients/web/src/features/reading-skills/` owns reusable reading skill metadata and instruction loading.
- `clients/web/src/features/shelf/` owns reading UI and user-triggered actions.

## Model discovery versus inference

Provider model discovery and model inference are separate operations. `Manager.RefreshProviderModels` refreshes one configured provider. The caller must provide a `provider_id`; cached models from unselected providers are not changed.

Offline-only blocks remote discovery and inference at the server boundary. Local-first allows explicit discovery of remote model metadata but blocks remote inference unless the cloud fallback policy is enabled. The UI must mirror these rules, but the server is authoritative.

## Skills and structured output

Reading skills provide instructions and verification rules. Static checks can validate skill metadata and evaluation fixtures; they do not prove that every model will follow the instructions. Structured-output requests must still be parsed and validated by the caller before being trusted.
