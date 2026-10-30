---
name: math-for-programmers
description: Connect formal mathematics to executable programs, algorithms, graphics, systems, and numerical computation without confusing a model with its implementation.
---
# Mathematics for Programmers — Formal Meaning Meets Finite Machines

## Required bridge
1. Identify the mathematical object and its formal definition before mapping it to code. Define variables, domains, units, and assumptions.
2. Explain the programmer-facing meaning: representation, operation, invariant, input/output, and why the abstraction is useful.
3. Give one tiny example with explicit values. Track the values through the operation, then generalize to the formal rule.
4. Distinguish the mathematical model from implementation details. A real-valued equation is not automatically an exact floating-point computation; asymptotic complexity is not a latency benchmark.

## Domain checks
- **Algorithms/data structures:** state input size, expected/worst/amortized case where relevant, and why time/space complexity follows.
- **Numerics:** account for finite precision, rounding, overflow/underflow, conditioning, cancellation, and representation limits when material.
- **Probability/statistics:** state the population/sample, assumptions, conditioning, independence, and uncertainty; avoid causal claims from correlation alone.
- **Graphics/linear algebra:** define coordinate spaces, vector/matrix dimensions, multiplication order, units, and transformations.
- **Systems/concurrency:** identify state, ordering, atomicity, invariants and race assumptions; mathematical equivalence does not guarantee the same execution behavior.

## Verification gate
Check dimensional consistency, boundary cases, numerical counterexamples and whether the code operation preserves the stated invariant. If giving code, ensure its types and precision assumptions match the explanation. If there is no faithful programming analogy, say so rather than forcing one.

## Output
Use the lightest useful path: intuition → formal rule → worked values → minimal code/pseudocode → limitations. Omit irrelevant stages. Label pseudocode and do not claim code was executed unless it was.
