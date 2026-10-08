"""Interactive help: a short overview, plus one screen per topic with examples.

Text is written in a tiny markup so it can be coloured (or left plain):

    # Title            section heading
    > what you type    an example input line
    ✓ what you see     an example result line
    ~ aside            dimmed explanation
    anything else      plain text; `backticked` words are commands
"""
from __future__ import annotations

import re

from idios.domain.errors import Invalid
from idios.shell.style import style

OVERVIEW = """\
# Learning
  Why do registers matter?  ~ saves a question
  Because they sit inside the CPU.  ~ answers the open question
  note: ...  highlight: ...  quote: ...
  concept: CPU Register
  link CPU Register to Memory

# Where you are
  goal: Learn Assembly
  source: Programming from the Ground Up
  chapter 2        page 14

# Look things up
  :search register     :status     :show CPU Register
  :questions  :concepts  :sources  :shelf  :graph

# More help, with examples
  :help learn  sources  context  concepts  search  manage  examples
  clear  ~ clear the screen
  :intro  ~ replay the welcome banner

# Exit
  :quit
"""

TOPICS: dict[str, str] = {
    "learn": """\
# Questions, answers, notes
~ Just type. A line ending in ? is a question.
> Why does the CPU need registers?
✓ Question saved · q1
~ A reply that reads like an answer goes to the open question.
> Because registers give very fast CPU-local storage.
✓ Answer saved · a1
  to: Why does the CPU need registers?
> note: RAM is slower because it is off-chip.
✓ Note saved · n1

# Highlights and quotes belong to the current source
> highlight: Registers can be accessed much faster than memory.
✓ Highlight saved · h1
> quote: "Simplicity is prerequisite for reliability."
✓ Quote saved · qt1

# When IDIOS is not sure, it asks
> registers are important
What should I save this as?
1. Note
2. Answer to current question
3. Ignore
> 1
✓ Note saved · n2

# Answer an older question
> answer q3: Because the ALU reads them directly.
""",
    "sources": """\
# Register what you read
> book: Programming from the Ground Up
✓ Source added and selected
Author (Enter to skip): Jonathan Bartlett
Path / URL (Enter to skip): ~/Shelf/Books/Assembly/pgu.pdf
~ Files are referenced, never copied. Other types:
  `article:`  `paper:`  `video:`  `website:`

# Or answer every field
> source add
Title / Author / Type / Path or URL / Shelf

# Fill in details later (for the current source)
> author: Jonathan Bartlett
> category: Assembly
> tag: cpu, low-level

# Come back to a source
> source: Ground Up
✓ Source selected
~ A unique piece of the title is enough.

# See your shelf
> :shelf
BOOKS
Assembly
  Programming from the Ground Up
""",
    "context": """\
# Context is filed automatically
> goal: Learn Assembly
✓ Goal created
> source: Programming from the Ground Up
✓ Source selected
> chapter 2
✓ Context updated
> page 14
✓ Context updated
~ Every question, note, highlight and quote now remembers
~ goal, source, chapter and page. No flags to repeat.

# Other locations
> section 2.3
> time 12:30            ~ for videos

# Starting again
~ Quit and come back: IDIOS offers to continue where you stopped.
> :forget               ~ forget the current concept and question
> clear                 ~ clear the screen (also :clear)
""",
    "concepts": """\
# Concepts
> concept: CPU Register
✓ Concept created
~ Typing it right after a question links that question to it.

# Relations connect concepts
> link CPU to CPU Register as contains
✓ Relation created
> link CPU Register to Memory
✓ Relation created
  CPU Register —related_to→ Memory
~ Unknown names become concepts. Types:
  related_to  depends_on  uses  contains  part_of
  explains  contrasts_with  prerequisite_of

# Look around
> :graph CPU
CPU
└── contains → CPU Register
    └── related_to → Memory
> :show CPU Register
~ sources, questions, related concepts, counts
> topic: Systems Programming
~ files the current concept under a topic
""",
    "search": """\
# Search everything you have recorded
> :search register
SEARCH: register
1. Concept — CPU Register  [c1]
2. Question — Why does the CPU need registers?  [q1]
3. Highlight — Registers can be accessed much faster…  [h1]
~ Plain words, ranked (BM25). Registers matches register.

# Jump to a result by its id
> :show q1
> :show c1

# From outside the shell
  `idios search register`
  `idios show "CPU Register"`
""",
    "manage": """\
# Lists
> :questions            ~ open questions (`:questions all` for every one)
> :concepts   :sources   :goals   :notes   :highlights   :quotes
> :status               ~ where you are and how much you have

# Delete (always asks first)
> :delete q3
Delete Question q3? This also removes 1 answer. [y/N]

# Export
  `idios export`                    ~ Markdown in ~/.idios/exports
  `idios export -f json -o out.json`

# Colour
  Set `NO_COLOR=1` to turn colour off, or `IDIOS_COLOR=always` to force it.
""",
    "examples": """\
# A complete first session
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
> highlight: Registers can be accessed much faster than memory.
✓ Highlight saved · h1
> concept: CPU Register
✓ Concept created
  linked to: What is a register?
> link CPU Register to Memory
✓ Relation created
> :search register
~ Runnable scripts: examples/ (idios run examples/01_first_session.idios)
""",
}
TOPICS["example"] = TOPICS["examples"]
TOPICS["sessions"] = TOPICS["context"]
TOPICS["graph"] = TOPICS["concepts"]
TOPICS["source"] = TOPICS["sources"]
TOPIC_NAMES = ["learn", "sources", "context", "concepts", "search", "manage", "examples"]

_TICKS = re.compile(r"`([^`]+)`")
_DIRECTIVE = re.compile(r"(?<![\w`:])(:[a-z]+)")
_KEYWORD = re.compile(
    r"^(\s*)((?:note|highlight|quote|answer|concept|topic|goal|source|book|article|paper|"
    r"video|website|author|category|tag):|link|chapter|page|section)(?=\s|$)")
_ASIDE_COLUMN = 38


def _inline(text: str) -> str:
    text = _TICKS.sub(lambda m: style.command(m.group(1)), text)
    text = _DIRECTIVE.sub(lambda m: style.command(m.group(1)), text)
    return _KEYWORD.sub(lambda m: m.group(1) + style.command(m.group(2)), text)


def _with_aside(visible: str, styled: str, aside: str) -> str:
    if not aside:
        return styled
    pad = " " * max(2, _ASIDE_COLUMN - len(visible))
    return styled + pad + style.dim(aside)


def render(markup: str) -> str:
    out = []
    for raw in markup.rstrip("\n").split("\n"):
        line = raw.rstrip()
        stripped = line.lstrip()
        indent = line[: len(line) - len(stripped)]
        if line.startswith("# "):
            out.append(style.heading(line[2:]))
        elif stripped.startswith("~ "):
            out.append(indent + style.dim(stripped[2:]))
        elif line.startswith("> "):
            body, _, aside = line[2:].partition("  ~ ")
            body, aside = body.rstrip(), _TICKS.sub(r"\1", aside)
            out.append(_with_aside("> " + body, style.dim("> ") + style.command(body), aside))
        elif line.startswith("✓"):
            out.append(style.ok(line))
        else:
            body, _, aside = line.partition("  ~ ")
            body = body.rstrip()
            visible = _TICKS.sub(r"\1", body)
            out.append(_with_aside(visible, _inline(body), aside))
    return "\n".join(out)


def topic(name: str = "") -> str:
    key = name.strip().lower().lstrip(":")
    if not key:
        return render(OVERVIEW)
    if key not in TOPICS:
        raise Invalid(f"There is no help topic called '{name.strip()}'.",
                      hint=":help " + " | ".join(TOPIC_NAMES))
    return render(TOPICS[key])
