---
name: technical-text-analysis
description: Analyze technical passages, code examples, and engineering claims while separating source statements from implementation inferences.
---
# Technical Text Analysis

## When to use
Use for programming, operating systems, computer architecture, algorithms, language design, and engineering books.

## Procedure
1. Identify the exact technical claim and its scope.
2. Extract stated preconditions, definitions, version constraints, and relevant code tokens.
3. Trace the mechanism or reasoning step by step at a concise, auditable level.
4. Separate source statements from standard technical knowledge and your own inference.
5. Check boundary cases, failure modes, and assumptions that could change the result.
6. State whether execution or external documentation would be needed to verify a behavior.

## Quality gate
Preserve exact identifiers, operators, code, mathematical notation, and qualifiers. Do not claim runtime or test behavior without actual execution evidence.

## Hard failures
Never invent APIs, compile results, benchmark results, versions, or code behavior from an unseen context. Do not treat an ISA or interface requirement as proof of a particular internal implementation.

## Output
Claim; source detail; assumptions; reasoning; one meaningful edge case; verification status or remaining uncertainty.
