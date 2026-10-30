---
name: claim-evidence-audit
description: Audit a specific claim against supplied evidence, separate direct support from inference, and offer a precise correction.
---
# Claim and Evidence Audit

## When to use
Use when evaluating an agent's interpretation, a factual assertion, or an author's claim against a supplied passage.

## Procedure
1. State one exact claim in a short phrase.
2. Identify the strongest available evidence for it, using only supplied source IDs or exact text.
3. Classify it as `supported`, `partly supported`, `inferred`, `unsupported`, `contradicted`, or `unverifiable with available evidence`.
4. Explain the link—or gap—between evidence and claim.
5. Give the narrowest correction that the evidence justifies.
6. Reassess when a peer supplies genuinely new evidence; do not repeat a settled objection.

## Quality gate
Check that the verdict fits the evidence, that direct text is separated from inference, and that a weaker supported claim is not confused with the original stronger claim.

## Hard failures
Never invent quotations, page numbers, sources, or facts from unseen chapters. Do not call a claim false merely because the current passage does not establish it. Do not treat another agent's confidence as evidence.

## Output
`Claim: ... | Verdict: ... | Evidence: ... | Fix or remaining gap: ...` Keep each audit focused on one claim.
