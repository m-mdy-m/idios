# Shelf Translation Architecture

The translation feature is organized around four boundaries:

- **Source preparation:** PDF extraction and paragraph/chunk boundaries produce the exact source payload. Context is never appended to the translatable payload.
- **Context and terminology:** `context.ts` builds a bounded book/page context and a stable glossary. Context is explicitly interpretation-only.
- **Translation integrity:** `source-integrity.ts` verifies that returned source-alignment segments are exact, ordered excerpts of the requested passage before they can be associated with it. This does not claim semantic correctness; semantic quality still requires review and benchmark evaluation.
- **Job lifecycle:** `job-manager.ts` owns app-scoped job state. UI visibility is deliberately independent from job cancellation. Only an explicit Stop action calls `cancel`.

## Non-negotiable invariants

1. A result is keyed to the exact book, page, request and source text that created it.
2. Context, notes and glossary entries may disambiguate terminology but must never become translation input.
3. A source-alignment failure is an error, not a reason to display an unrelated cached result.
4. Closing the companion hides the view; it does not cancel a running job.
5. Translation cancellation is explicit and idempotent.
6. Persian and Arabic content must declare RTL direction; embedded English terms must isolate their LTR direction.
7. A fluent result is not automatically a faithful result. Use reviewed domain-specific fixtures to measure omissions, additions, terminology consistency and negation fidelity.

## Quality evaluation

Maintain reviewed passages from philosophy, science and technical writing. Review against proposition coverage, negation/qualifier preservation, terminology consistency, paragraph structure and unsupported additions. Do not infer quality from provider/model names or fluency alone.

## Quality benchmark

The sibling `benchmarks.ts` module contains deterministic source/reference candidates for:

- Rawls-style political philosophy, with core terms such as *considered judgments*, *substantive theory*, and *peripheral*.
- Technical prose containing transactional all-or-nothing guarantees and failure conditions.
- General prose with concession and uncertainty qualifiers.

The Persian references are proposed baselines and are explicitly marked `maintainer-review-required`; they must not be described as peer-reviewed gold translations until a subject-aware reviewer approves them.

The rubric evaluates proposition fidelity, negation/qualification, completeness, terminology consistency, readability and unsupported additions. Deterministic tests can check source alignment, paragraph preservation and required markers. They cannot prove semantic equivalence for arbitrary model output; that still requires human review of sampled translations.
