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

- MIMI returns one exact highlight action anchored to the selected page text.
- MIMI returns one separate note action with a useful title and a short explanation.
- The reader executes the highlight through the PDF annotation persistence path.
- The reader executes the note through the Shelf notes API.
- The UI reports each saved result independently. A model response alone is never shown as a successful save.

Illustrative action envelope (the model must still quote the actual selected page text):

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

Select these four participants, with MIMI · Chair last:

1. Reader
2. Sage · Tutor
3. Atlas · Steelman
4. MIMI · Chair

Use this discussion prompt:

> Why does Rawls insist that Chapters IV–VIII are not peripheral or merely applications? Explain how the test of a theory relates to considered judgments, and preserve the qualification that readers may follow the problems that most concern them. Each agent should assess a real earlier contribution; MIMI should compare the actual positions rather than count agreement.

The source passage is supplied as `[source:rawls-page-21]`. The integration test uses the complete fixture and checks that all four sequential turns finish. It also logs model names, warnings, and each contribution for CI review.

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
