---
name: sequential-agent-dialogue
description: Strict low-token protocol for evidence-grounded sequential book debate, direct peer criticism, correction, and one final reader question.
---
# Sequential Debate Contract

## Trigger and authority
Use this contract for every multi-agent book discussion. The supplied page text is the only direct book evidence. Highlights, notes, and earlier agent statements are useful context but are not automatically true. Agent output is a claim to evaluate, never proof of itself.

## Required per-turn procedure
1. **Read the thread, not just the last line.** Use all prior contributions supplied to the turn, the reader's opening, and current page/annotation context. Identify the current round and whether this is the final Chair turn.
2. **Select one target.** After the opening Reader turn, name one actual prior participant by their exact displayed name and quote or accurately paraphrase one specific claim. Never attribute a claim to the wrong speaker.
3. **Classify the claim.** Choose exactly one: `supported`, `partly supported`, `unclear`, or `unsupported`. Do not confuse “not shown on this page” with “false.”
4. **Give a traceable reason.** Cite only an exact supplied `[source:ID]` marker for page evidence. If the source does not support the claim, say what is absent. Never invent a page, quotation, source ID, external search result, or unseen-chapter content.
5. **Improve the shared conclusion.** Add one precise correction, scope limit, counterargument, terminology clarification, or refinement. If the previous speaker's point is already correct, explicitly strengthen it with a distinct reason instead of manufacturing disagreement.
6. **Handle questions as agenda items.** During peer turns, never ask the reader directly. If a model raises a reader question early, rewrite it as an unresolved issue for the other agents to assess. Peers must first determine whether the ambiguity is real, evidence-resolvable, and consequential. Only the final Chair turn can ask one question, and only if the answer could change the conclusion.
7. **Respect the bounded debate.** Complete the requested number of rounds. Do not stop just because one agent has spoken or repeated a point. Do not continue past an explicit runtime cancellation or reader-wait state. On later rounds, revisit disputed claims and record whether the objection was resolved, narrowed, or remains open.

## Independent role contribution and argument fidelity

- The configured agent role is a deliverable, not a decorative name. Fulfil that role's distinct task before adding a general observation. Do not collapse Reader, Tutor, Translator, Reviewer, Evidence Auditor, Counterexample Tester, or Chair into the same generic response.
- Agreement is not a response by itself. Add a new implication, qualification, terminology clarification, evidence check, or bounded example. When there is no distinct supported addition, say so plainly rather than restating the previous speaker.
- Never speak as another participant, copy another agent's signature or style, or attribute another participant's claim to yourself. Treat the displayed transcript as immutable source data.
- When a later peer follows Mira · Chair, inspect at least one concrete Chair claim against the original supplied passage. A Chair conclusion is not a substitute for fresh verification and must not be echoed as a vote.
- Preserve logical force exactly. Track negation, contrast, modality, causal direction, and scope (for example: *not*, *rather than*, *merely*, *only*, *unless*, *may*, *in turn*). An explicitly rejected interpretation must not be presented as the author's conclusion or as an equally supported alternative.
- Identify the page's central argumentative move, then distinguish it from secondary acknowledgements, examples, qualifications, and methodological asides. Do not mistake a page's list of topics or names for its main claim.
- Treat history excerpts as partial when a boundary or shortening warning is present. Do not claim to remember omitted text. Re-check all material book claims against the complete source passages available to the current turn; peer history never upgrades a claim into evidence.

## Deterministic peer validation

- Every non-opening peer turn must identify at least one real completed participant by their displayed name or unambiguous role label. A statement such as “the previous claim” without a real speaker is insufficient.
- The final Chair must identify at least two actual peer positions when at least two completed peer contributions are available. Do not use anonymous phrases such as “the agents agree” in place of attribution.
- Do not copy a peer contribution verbatim. Agreement is allowed, but it must add a distinct reason, implication, qualification, or explicit statement that no further supported point is available.
- The runtime rejects an unreferenced peer turn, a verbatim echo, or a Chair conclusion that omits available named positions. It attempts one focused repair; a second failure remains failed and is not passed to later agents as a valid argument.

## Hard output contract
- Initial Reader turn: one central claim, one exact supporting textual detail, and one limited interpretation. If there is no evidence, state `Evidence: not supplied`.
- Normal peer turn: 2–3 short sentences and no more than 55 words. In order: named target and claim; verdict plus cited reason; one correction/refinement. No generic summary, preamble, headings, numbered list, or more than one argument.
- Final Chair turn: no more than 75 words before the optional question. Name the actual positions that agree or disagree, say what evidence supports each, state whether the disagreement is resolved, and present one refined conclusion. If needed, finish with exactly one `Question for you: ...` line. Otherwise ask nothing.
- Never expose hidden chain-of-thought. Give only a concise, auditable reason, not private deliberation.
- Finish every sentence. Do not exceed the word cap, leave a dangling list item, repeat the same point with new wording, or add unsupported claims to sound helpful.

## Mandatory rejection conditions
A turn is invalid if it invents a source or quote; relies on an unseen page; describes a guess as textual fact; misquotes a participant; repeats without advancing; fabricates disagreement or consensus; asks the reader before the final Chair; exceeds the output budget; or is visibly unfinished. Repair the whole response once using the violated rule. If it remains invalid, do not feed its content to later agents as trustworthy evidence; mark the contribution rejected and continue with valid contributions when possible.

## Verification gate
Before finishing, check: (1) correct named target, (2) verdict matches the actual evidence, (3) every source marker exists in the supplied context, (4) one useful refinement, (5) no premature reader question, (6) word count and complete sentences, and (7) no claim of consensus that the transcript does not support.

## Output
For every normal peer turn: one named claim, one verdict, one grounded reason, one refinement, at most 55 words. Final Chair: positions + unresolved point + refined conclusion, at most 75 words, optionally one consequential final-line question.
