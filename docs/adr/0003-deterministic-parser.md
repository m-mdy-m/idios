# 3. A deterministic parser instead of AI classification

Date: 2026-10-08

## Status

Accepted

## Context

The shell must feel like conversation (`> Why does the CPU need registers?`) without the
user learning commands, and the core must work with no AI (`AGENTS.md` §3).

## Decision

`shell/parser.py` is a pure function with a handful of explicit rules: directives (`:`),
`keyword:` prefixes, `link A to B`, short location phrases, a trailing `?`, and answer cues
used only when a question is open. Anything else is *asked about*, never guessed.

## Consequences

- Behaviour is predictable and unit-testable; no model, no network.
- Some phrasings need the explicit form (`note:`) or one menu choice.
- Wrong guesses are visible (the answer message names its question) and reversible (`:delete`).
