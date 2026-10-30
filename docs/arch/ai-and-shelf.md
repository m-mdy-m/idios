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

## Evidence-led agent discussions

Book discussions use a sequential round-table. Each selected agent receives the original supplied passages plus the completed peer turns that precede it; parallel generation is disabled because it would prevent meaningful peer review.

- Built-in agents receive a mandatory role-specific deliverable in addition to their editable profile instructions. They must not impersonate peers, treat a peer statement as source evidence, or paraphrase agreement without adding a new qualification or reason.
- Source passages remain authoritative. Agent outputs are claims to check, not facts to inherit. Exact `[source:ID]` markers are validated against the request's supplied source IDs.
- The final speaking turn belongs to Mira · Chair. Mira compares the actual positions, weighs the supplied text rather than counting votes, preserves explicit negations and qualifications, and reports unresolved disagreement instead of forcing consensus.
- A continuation carries up to 40 completed turns, with a 5,000-byte per-turn and 48,000-byte aggregate UTF-8 budget. Oversized carry-over is bounded from the newest turn backward; truncation preserves source markers and adds a warning. Failed and waiting turns are not treated as peer evidence.
- Older turns are represented by bounded excerpts in a later model prompt. Those excerpts retain source markers; the original contribution remains visible in the task result. The source passage is reattached in full on every generation.

Role-specific agent skills and runtime role contracts require independent contribution, non-echo peer review, and careful preservation of the author's logical force. The deterministic validator checks response completeness, word budgets, source-marker allowlists, explicit chapter/section references, named peer attribution, verbatim echoes, and final-Chair coverage of actual positions. Chapter ranges such as `Chapters IV–VIII` are interpreted as inclusive ranges, so an endpoint cited from that range is not incorrectly rejected. These checks do not prove semantic correctness: model outputs still require source-based evaluation, and a failed Chair turn remains an explicit partial result rather than being presented as a successful synthesis.

## Skills and structured output

Reading skills provide instructions and verification rules. Static checks can validate skill metadata and evaluation fixtures; they do not prove that every model will follow the instructions. Structured-output requests must still be parsed and validated by the caller before being trusted.
