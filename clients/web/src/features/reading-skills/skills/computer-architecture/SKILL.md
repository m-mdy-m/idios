---
name: computer-architecture
description: Explain computer architecture and digital design from the book's exact model, tracing data/control/state and checking bit widths, timing, memory behavior, and ISA boundaries.
---
# Computer Architecture and Digital Design

## Lock down the level
Determine whether the passage concerns logic gates, digital circuits, datapaths/control, ISA, microarchitecture, memory hierarchy, operating systems, or distributed systems. Do not mix these abstraction levels. An ISA requirement is not the same as a specific microarchitectural implementation.

## Required method
1. Identify the system, components, signals/state, clock/event assumptions, inputs, outputs, and invariant being discussed.
2. For a datapath/control explanation, trace one concrete operation through the stages: source state → selected control/signals → transformation → destination state. Use only the stages supported by the supplied design.
3. For digital logic, preserve Boolean operators, truth-table inputs/outputs, active-high/low conventions, edge/level triggering, bit widths, signedness, and timing assumptions.
4. For processor topics, distinguish ISA-visible behavior from implementation choices such as pipelines, caches, branch predictors, reorder buffers, and physical registers.
5. For memory topics, identify address space, cache line/block, hit/miss, latency/bandwidth, coherence/consistency, and virtual versus physical addresses only when relevant. Do not conflate them.
6. For RISC-V or another ISA, use the exact extension/version named by the source. Do not silently apply newer instructions or toolchain assumptions.
7. If giving a numerical example, compute widths, offsets, masks, signed conversions, and units explicitly. State simplified assumptions such as no hazards or ideal memory when used.

## Verification gate
Check bit counts, bus widths, truth-table rows, signal polarity, stage order, PC/address arithmetic, byte/word distinction, endian assumptions, and whether a behavior is architecturally guaranteed or implementation-specific. A diagram analogy must not contradict the source model.

## Output
Use a small trace/table or stepwise state description when it simplifies the mechanism. Define acronyms once. Clearly label assumptions and do not invent a figure/table that was not supplied.
