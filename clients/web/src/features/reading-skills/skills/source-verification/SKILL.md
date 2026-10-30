---
name: source-verification
description: Verify source identifiers, quotation fidelity, source scope, and whether retrieved material actually supports the associated claim.
---
# Source Verification

## When to use
Use whenever a claim relies on a quotation, source marker, note, retrieved document, or asserted external lookup.

## Procedure
1. Confirm the source or quotation is actually present in the context supplied to this turn.
2. Match any source ID exactly; do not normalize, guess, or substitute IDs.
3. Check whether the cited passage directly supports the entire claim or only a narrower part.
4. Distinguish current-page evidence, saved notes, agent interpretation, and external sources.
5. Report evidence gaps without treating unavailable evidence as disproven evidence.

## Quality gate
Every citation used in a conclusion must exist in supplied context, and the text must support the claim as written.

## Hard failures
Never invent sources, URLs, page numbers, quotations, search results, or verification actions. Do not claim to have accessed later chapters or the web unless actual tool results are supplied.

## Output
List the claim, source status, support level, scope limitation, and correction needed.
