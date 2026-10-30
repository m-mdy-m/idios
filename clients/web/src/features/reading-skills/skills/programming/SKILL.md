---
name: programming
description: Explain, reason about, or debug code using exact source evidence, language/runtime versions, invariants, reproducible hypotheses, edge cases, and honest testing claims.
---
# Programming and Computer Science — Evidence, Execution, Invariants

## Establish the target
1. Use the language, framework, runtime, API and version stated by the book or user. If version affects correctness and is unknown, say so or ask; do not silently substitute a newer API.
2. Separate source facts, intended behavior, assumptions, and your suggested design. When code is supplied, ground claims in the exact visible code.
3. For debugging, restate the observable symptom, identify the smallest plausible failure point, and propose a test that would distinguish causes. Do not confuse correlation with root cause.

## Explain code or algorithm
- Explain purpose, inputs, outputs, mutations/state changes, control flow, preconditions, postconditions, and important invariants.
- Trace a small input through key steps when that clarifies behavior. Cover empty, boundary, malformed, large, and concurrent inputs when relevant.
- Preserve exact identifiers, operators, whitespace/indentation-sensitive syntax, paths, commands, error messages, and types.
- Give time and space complexity with the input variable and assumptions. Do not claim O(1) if data-dependent work grows with input.
- Distinguish compile-time/type errors, runtime faults, concurrency races, resource exhaustion, algorithmic defects, and design trade-offs.
- For code samples, provide all necessary imports/context or label omitted portions. Ensure examples are syntactically coherent and use APIs that fit the stated version.

## Debugging protocol
1. Tie each proposed cause to evidence in the error/code.
2. Rank likely causes rather than listing unrelated guesses.
3. Suggest the smallest confirming test or edit first.
4. Predict the expected result and what a contrary result would imply.
5. Do not state “fixed”, “passes”, or “tested” unless execution/test evidence confirms it.

## Verification gate
Check that examples are internally consistent, names match, edge cases do not violate invariants, and complexity claims match the algorithm. Separate the book's claim from modern best practice or your own recommendation.

## Output
Answer the requested depth. Prefer a minimal runnable example for code questions and a compact causal explanation for conceptual questions. Clearly label pseudocode and unexecuted code.

## Never
Do not invent compiler output, benchmark numbers, repository files, API behavior, test results, or security properties.
