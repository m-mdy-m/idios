---
name: pdf-annotation-authoring
description: Create accurate, useful notes and highlights anchored to an explicitly requested PDF page, selected passage, or current reading location; use whenever the reader asks to annotate, highlight, or write a note.
---
# PDF Annotation & Note Authoring

## When to use
Use whenever the reader asks to highlight text, add/edit a margin note, create a study note from a page, annotate a passage, or explain why a highlighted passage matters. This skill applies to the PDF reader and other page-based reading surfaces when annotation actions are supported.

## Identify the requested operation first
1. Determine the target surface (PDF or page), target page, and whether the reader selected exact text.
2. Separate the requested action: **highlight**, **note**, **both**, or **draft only**. A request to explain text is not automatically permission to save an annotation; a request to write a note is not automatically permission to highlight.
3. Prefer the user's explicit selection. Otherwise use the explicitly named page or the current page. Never silently move an annotation to a different page.
4. If the target page or requested operation is genuinely ambiguous and cannot be resolved from the current reader state, ask one short clarifying question before writing.

## Highlighting: exact-source procedure
1. Use the actual extracted text for the target page as the authoritative source. Surrounding pages can clarify meaning but cannot provide words to highlight on this page.
2. Choose the smallest contiguous passage that captures the requested idea. Keep the author's original wording, punctuation, and language.
3. Before creating a highlight, verify that the proposed quote occurs verbatim on the target page. Normalize whitespace only if the annotation API requires it; do not rewrite words, combine separated fragments, or fabricate a quote.
4. If the page text is unavailable, scanned without OCR, or the exact phrase cannot be located, do not invent a highlight. Explain the limitation and ask the reader to select the passage or provide OCR/text if needed.
5. Avoid over-highlighting. Unless the reader requests otherwise, create no more than three highlights per request and prioritize definitions, thesis/claim, key evidence, or a crucial qualification.

## Writing useful notes
1. Write notes in the reader's requested language; if none is specified, use the language of the conversation.
2. Make each note specific to its anchor. In one to three concise sentences, state the idea in plain language, why it matters here, or how it connects to the author's argument. Add an example only when it genuinely clarifies the passage.
3. Distinguish what the author explicitly says from explanation, inference, and the assistant's own suggestion. Do not present an interpretation as a direct quote.
4. Use earlier pages for definitions and argument setup, the target page as primary evidence, and later pages only for continuity or clarification. Do not let later text change the wording of the anchor.
5. For a standalone study note not attached to a quote, include a short title and a compact explanation. Do not invent page numbers, citations, quotations, or claims about the book.
6. Preserve existing notes and highlights unless the reader explicitly asks to edit or delete them. Never replace, delete, or move an existing annotation as a side effect.

## Save and action honesty
1. Check which annotation actions the application actually supports before promising to save. If only drafting is available, present the note as a draft and clearly say it was not saved.
2. Use the application's supported annotation action and required anchor format; do not invent API calls, success states, or coordinates.
3. Report only actions confirmed by the tool/application result. If a highlight succeeds but its note fails (or vice versa), report each outcome separately and do not claim the whole task succeeded.
4. If an operation fails, preserve the draft and provide a concise actionable reason when known. Never say an annotation was saved merely because you generated its text.

## Verification gate
Before finishing, check: correct target page/surface; requested action only; every highlight quote is verbatim and contiguous on that page; each note matches its anchor; no existing annotation was changed unintentionally; and every claimed save/edit/delete is confirmed by the application.

## Output
After an annotation request, briefly report the page, the exact highlighted phrase(s), the note text or a short description, and which actions were actually saved. For draft-only requests, provide the draft and label it clearly as unsaved. Keep the explanation concise unless the reader asks for analysis.

## Never
- Never fabricate or paraphrase text inside quotation marks as a highlight.
- Never highlight text from a neighboring page while claiming it belongs to the target page.
- Never create annotations without explicit authorization.
- Never confuse a margin note, a standalone study note, and a PDF highlight.
- Never claim that an annotation was saved without confirmed success.
