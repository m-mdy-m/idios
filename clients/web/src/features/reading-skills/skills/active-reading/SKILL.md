---
name: active-reading
description: Strict, compact close reading for short or weak local models; extract only a supported claim, evidence, and bounded interpretation.
---
# Active Reading — Minimal Evidence Contract

## Required procedure
1. Read only the supplied passage and notes. Do not infer the whole book from one page or from the title.
2. Identify one central claim actually present in the supplied text. If no claim is explicit, label it as an inference.
3. Select one exact, short evidence phrase or a precise textual detail. Quotes must match the supplied passage character-for-character; never reconstruct from memory.
4. Separate the author's explicit statement from the agent's interpretation. Preserve qualifiers, negation, scope, and who the claim applies to.
5. Do not add related concepts merely because they are associated with the topic. For example, never attribute autonomy, self-respect, equality, or utilitarianism to a passage unless the supplied text supports that connection.
6. If source text is missing, unreadable, or insufficient, say exactly what is missing. Do not fill gaps with generic subject knowledge.

## Required output for a simple reading-analysis request
Return exactly these three lines and nothing else:
- **Claim:** one sentence, no broader than the passage.
- **Evidence:** one exact short quote or precise detail, followed by its supplied `[source:ID]` marker. If unavailable, write `Evidence: not supplied`.
- **Reading:** one sentence explaining what the evidence supports and what remains interpretation.

Maximum 45 words total. No introduction, conclusion, numbered list, extra key concepts, generic essay, or fourth item. Finish every line completely; never start a point you cannot finish.

## Debate mode
When another named agent has spoken, do not repeat the three-line analysis. Address one specific claim by name and do exactly one of these: correct it with evidence, narrow an overstatement, test a premise, or add a missing distinction. Under 45 words. Do not ask the reader a question; reserve reader questions for the final chair after peer review.

## Final quality gate
- Is the central claim present in the supplied passage?
- Is the evidence literal and traceable to a supplied source ID?
- Is interpretation clearly distinguished from the author's statement?
- Are all three lines complete and under the word limit?
- Did the answer avoid adding concepts unsupported by the passage?

## Never
Never invent quotations, citations, page numbers, author intent, other agents' positions, or saved annotations. Never claim that a plausible general fact appears in the book unless the passage shows it.

## Output
Use the exact three-line contract defined above for simple reading-analysis requests. In debate mode, use the short named-claim response defined above.
