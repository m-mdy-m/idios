# Examples

Each `.idios` file is a script of exactly what you would type. Lines starting with `#` are
narration. Play one (nothing touches your real `~/.idios`):

```bash
./examples/run.sh 01          # by number
./examples/run.sh --all       # everything (also: make examples)
idios --home /tmp/try run examples/01_first_session.idios
```

| File | Shows |
| --- | --- |
| `01_first_session.idios` | goal, source, chapter, question → answer, note, highlight, concept, link, search, status |
| `02_reading_a_paper.idios` | paper with URL, author, tags, shelf category, section/page, quote, topic |
| `03_video_course.idios` | video with `time`, notes, highlight, quote, listing commands |
| `04_building_a_graph.idios` | typed relations (`as contains`, `as uses`), `:graph`, concept view |
| `05_when_unsure.idios` | ambiguity menus, `answer q2: …`, delete confirmation |
| `06_python_api.py` | the same services used from Python |

## What a session looks like

```text
> goal: Learn Assembly
✓ Goal created
> book: Programming from the Ground Up
✓ Source added and selected
> chapter 2
✓ Context updated
> What is a register?
✓ Question saved · q1
> A small storage location inside the CPU.
✓ Answer saved · a1
  to: What is a register?
> concept: CPU Register
✓ Concept created
  linked to: What is a register?
> link CPU Register to Memory
✓ Relation created
  CPU Register —related_to→ Memory
  (new concept: Memory)
> :search register
SEARCH: register

1. Concept — CPU Register  [c1]
2. Question — What is a register? · Programming from the Ground Up  [q1]
```

## When IDIOS is unsure

```text
> registers are important
What should I save this as?

1. Note
2. Answer to current question
3. Ignore
> 2
✓ Answer saved · a1
```

Write your own: any text file of inputs works, one per line. Put your data somewhere safe
with `IDIOS_HOME` or `--home`.
