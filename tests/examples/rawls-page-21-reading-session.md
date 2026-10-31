# Shelf reading workflow example — Rawls, page 21

Fixture: `tests/fixtures/rawls-a-theory-of-justice-page-21.txt`

Book: **A Theory of Justice: Revised Edition**  
Page: **21**  
Local model used by CI: **`qwen2.5:3b` through Ollama**

## 1. Ask MIMI to make both book edits

Open page 21 in the PDF Reader and select the exact phrase:

> order and system

Send MIMI this request:

> Please write a concise standalone note about the central argument on this page, and highlight the selected phrase “order and system”. Add a short margin explanation to the highlight. Save both the highlight and the separate note to this book.

Expected application behavior:

- The Shelf action pipeline creates one exact highlight action from the reader-selected phrase. If the model proposes an approximate quote, the selected text replaces it.
- MIMI returns one separate note action with a useful title and a short explanation.
- The reader executes the highlight through the PDF annotation persistence path.
- The reader executes the note through the Shelf notes API.
- The UI reports each saved result independently. A model response alone is never shown as a successful save.

Illustrative normalized action envelope after the Shelf client combines MIMI’s note proposal with the reader’s authoritative selection:

```json
{
  "actions": [
    {
      "type": "highlight",
      "page": 21,
      "quote": "order and system",
      "note": "This is the test the theory must pass: making many considered judgments coherent.",
      "color": "purple"
    },
    {
      "type": "note",
      "title": "A theory must organize considered judgments",
      "content": "Rawls says a theory of justice must introduce order and system into our considered judgments across a wide range of questions. The later topics therefore test and may modify the proposed view; they are not merely peripheral applications.",
      "labels": ["reading-companion", "rawls"]
    }
  ]
}
```

## 2. Run the sequential discussion

Select these six participants, with MIMI · Chair last:

1. Orion · Reader
2. Sage · Tutor
3. Atlas · Steelman
4. Concept Connector
5. Vera · Evidence Auditor
6. MIMI · Chair

Use this discussion prompt:

> Analyze why Rawls insists that Chapters IV–VIII are not peripheral or merely applications. Explain what “order and system” means for our considered judgments, and connect that claim to the examples of theoretical revision in the acknowledgments. Preserve the distinction between the chapters’ importance and the reader’s freedom to choose which problems to approach first. Use only this page as direct evidence; do not invent the contents of unseen chapters.

The fresh Round 1 is intentionally independent: Reader, Tutor, Steelman, Concept Connector, and Evidence Auditor each fulfil their own role from the page, without pretending to respond to peers they have not seen. MIMI receives all four opening positions and must compare them against the decisive wording, explicitly rejecting any claim that turns reader preference into evidence that the chapters are optional or unimportant. If you choose a second round, the specialists then receive completed peer history and review real claims by name.

The source passage is supplied as `[source:rawls-page-21]`. The integration test uses the complete fixture, downloads Qwen through Ollama, checks that all twelve sequential discussion turns (six agents across two rounds) finish, and asks the same local model to generate MIMI’s standalone note as JSON. It validates the note title/content, the Reader’s exact source marker, the distinction between reader preference and chapter importance, and the selected phrase as an exact, unique page-21 anchor. CI logs model names, warnings, every contribution, and the note output.

## 3. Run locally

Start Ollama and pull the same model used in CI:

```sh
ollama pull qwen2.5:3b
ollama serve
```

In another terminal, run:

```sh
IDIOS_OLLAMA_INTEGRATION=1 IDIOS_OLLAMA_MODEL=qwen2.5:3b go test ./internal/ai -run '^TestOllamaBookDiscussionIntegration$' -count=1 -v -timeout 20m
```

The normal Go suite skips the Ollama integration test unless `IDIOS_OLLAMA_INTEGRATION=1` is set. The Web action-contract check is run by the existing Web build and verifies the mixed note/highlight action path without requiring a running browser or a live model.
