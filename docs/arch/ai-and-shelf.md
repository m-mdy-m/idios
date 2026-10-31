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

Translation is a no-thinking task. Both the client and server omit positive reasoning requests for the `translation` task. The shared runtime strips unsupported reasoning parameters for every selected model; when a model advertises native Ollama thinking, it explicitly disables that capability for translation, while models that do not advertise it receive no `think` field. An unsupported `think` or `reasoning_effort` parameter must never turn translation into a provider failure.

Provider model discovery and model inference are separate operations. `Manager.RefreshProviderModels` refreshes one configured provider. The caller must provide a `provider_id`; cached models from unselected providers are not changed.

Offline-only blocks remote discovery and inference at the server boundary. Local-first allows explicit discovery of remote model metadata but blocks remote inference unless the cloud fallback policy is enabled. The UI must mirror these rules, but the server is authoritative.

## Evidence-led agent discussions

New IDIOS installations default to a six-agent capacity and a Shelf roster of Reader, Tutor, Steelman, Concept Connector, Evidence Auditor, and MIMI · Chair. Existing saved runtime limits are preserved, so an installation whose saved `max_agents` is still 5 must raise it to 6 under Settings → AI Runtime → Execution & privacy → Advanced resource limits before selecting all six. Book discussions use a sequential, staged round-table. In a fresh discussion, Round 1 Reader and specialist turns are independent: each sees the supplied passages and saved annotations, but not another specialist's same-round answer. MIMI speaks last and compares those opening positions. In Round 2 and later, specialists receive bounded completed peer history and must examine a real claim from a role-relevant participant. A resumed discussion starts with its supplied history available immediately. Parallel generation is disabled because it would prevent meaningful peer review.

The Shelf Agent Discussion UI mirrors this protocol: it explains the independent opening versus later peer-review rounds, offers a guided page-analysis prompt, shows the model/provider for each completed turn, and displays a run-level completion/failure/warning summary. “Runtime checks passed” means the response passed structural/evidence-marker validation; it is not a semantic truth score. Readers must still compare each claim with the displayed source text, and any warning keeps the contribution visibly marked for review.

- Built-in agents receive a mandatory role-specific deliverable in addition to their editable profile instructions. They must not impersonate peers, treat a peer statement as source evidence, or paraphrase agreement without adding a new qualification or reason.
- Source passages remain authoritative. Agent outputs are claims to check, not facts to inherit. Exact `[source:ID]` markers are validated against the request's supplied source IDs.
- The final speaking turn belongs to MIMI · Chair. MIMI compares the actual positions, weighs the supplied text rather than counting votes, preserves explicit negations and qualifications, and reports unresolved disagreement instead of forcing consensus.
- A continuation carries up to 40 completed turns, with a 5,000-byte per-turn and 48,000-byte aggregate UTF-8 budget. Oversized carry-over is bounded from the newest turn backward; truncation preserves source markers and adds a warning. Failed and waiting turns are not treated as peer evidence.
- Older turns are represented by bounded excerpts in a later model prompt. Those excerpts retain source markers; the original contribution remains visible in the task result. The source passage is reattached in full on every generation.

The discussion is deliberately staged. In a fresh discussion, Round 1 specialist turns are independent: Reader, Tutor, Steelman, Concept Connector, and other specialists see the original source context but not one another's same-round answers. MIMI may read those independent turns only when it is the final Chair turn. Later rounds expose the bounded history and let each role select a peer suited to its responsibility instead of mechanically replying to the immediately previous speaker. The Chair prioritizes the central reading and a substantive challenge (such as Steelman, Reviewer, or Evidence Auditor), not two arbitrary recent voices.

Role-specific agent skills and runtime role contracts require independent contribution, non-echo peer review, and careful preservation of the author's logical force. The built-in role contracts are deliverable-oriented: the Tutor explains one difficult relation, the Steelman reconstructs a fair argument, the Concept Connector makes only grounded cross-source links, the Ambiguity Scout rejects false ambiguity, the Evidence Auditor checks exact source support, and the Chair adjudicates rather than counts agreement. The validator separates hard evidence failures from recoverable quality defects. Empty or visibly unfinished responses, invented source IDs, and explicit chapter/section references absent from the supplied passages remain hard failures after one focused repair. Missing citations, word-budget overruns, missing peer attribution, verbatim echoes, and high lexical-overlap paraphrases are surfaced as quality warnings after one focused repair when the response is otherwise complete. Echo detection is advisory: lexical similarity is not a semantic verdict and never rejects a contribution by itself. An uncited textual claim remains visibly unverified and cannot be counted as independent evidence. The Reader must supply an exact page/source marker for a page-specific claim. A narrowly source-triggered logical-force guard also protects explicit negations and qualifications: when a passage says material is not peripheral while separately giving the reader freedom to choose a reading path, an output that infers the material is optional or unimportant is sent through a focused repair. This is a targeted safeguard, not a general semantic correctness score. The visible output budget is 85 words for specialists and 110 for the final Chair, giving compact models room to state a reason rather than only restate a conclusion. Repair prompts include the exact displayed names and short claims of completed peers. Chapter ranges such as `Chapters IV–VIII` are interpreted as inclusive ranges. Each selected agent retains a bounded per-turn timeout (at least four minutes for local providers). The enclosing discussion deadline is calculated from the selected agent count, requested rounds, and each selected provider's timeout, so a sequential multi-round discussion does not lose its later speakers to the unrelated default ten-minute workflow cap; the calculated deadline remains capped at one hour and does not change the user's global timeout setting. These checks do not prove semantic correctness: model outputs still require source-based evaluation, and every unverified claim is labelled as such rather than being presented as established fact.

The Shelf assistant treats a model action envelope as a proposal, not a completed write. A mixed request can produce one exact highlight action and one separate book-note action; the client sends them through the annotation and Shelf-note persistence APIs independently and reports the result of each. A reader-selected phrase takes precedence over a model-generated highlight quote. `tests/examples/rawls-page-21-reading-session.md` documents the end-to-end example, and `scripts/test-ai-agent-ollama.sh` downloads `qwen2.5:3b` and runs the real six-agent, two-round round-table test (Reader, Tutor, Steelman, Concept Connector, Evidence Auditor, and MIMI Chair) through the reusable `m-mdy-m/pipeline` workflow task. The Rawls page-21 integration also checks that the Reader, Evidence Auditor, and Chair cite the supplied page marker and that no agent turns reader preference into a claim that the chapters are optional or unimportant.

## Skills and structured output

Reading skills provide instructions and verification rules. Static checks can validate skill metadata and evaluation fixtures; they do not prove that every model will follow the instructions. Structured-output requests must still be parsed and validated by the caller before being trusted.
