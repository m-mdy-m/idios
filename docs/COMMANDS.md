# IDIOS command reference

Everything you can type, with examples. Output shown is real (colour omitted).

- [How input is understood](#how-input-is-understood)
- [Learning](#learning) · [Context](#context) · [Sources](#sources) · [Concepts and relations](#concepts-and-relations)
- [Planning: did, plan, done, skip](#planning) · [Review and reminders](#review-and-reminders)
- [Looking things up](#looking-things-up) · [Managing data](#managing-data) · [Screen and help](#screen-and-help)
- [Command line](#command-line) · [Environment variables](#environment-variables) · [Cheat sheet](#cheat-sheet)

## How input is understood

Each line you type is classified by a few fixed rules, in this order:

| You type | It is |
| --- | --- |
| `:name …` | a directive (command) |
| `keyword: text` | that kind of record (`note:`, `goal:`, `did:` …) |
| `link A to B` | a relation |
| `chapter 2`, `page 14` | a location update |
| a line ending in `?` | a question |
| anything else | IDIOS asks what to save it as |

```text
> registers are important
What should I save this as?

1. Note
2. Answer to current question
3. Ignore
> 1
✓ Note saved · n2
```

Every saved item gets a short id: `q3` question, `a1` answer, `n2` note, `h1` highlight, `qt1` quote,
`c1` concept, `s2` source, `g1` goal, `p1` person, `t1` topic, `k1` category, `tg1` tag, `tk4` task.
Use these ids with `:show`, `:delete`, `done`, `skip`, `answer`.

## Learning

### Question — end the line with `?`
```text
> Why does the CPU need registers?
✓ Question saved · q1
```
It becomes the *current question*. It remembers your goal, source, chapter and page.

### Answer
A reply that reads like an answer is attached to the open question:
```text
> Because registers give very fast CPU-local storage.
✓ Answer saved · a1
  to: Why does the CPU need registers?
```
Explicit forms:
```text
> answer: Because the ALU reads them directly.      ~ to the current question
> answer q3: Because the ALU reads them directly.   ~ to any older question
```

### `note:`  `highlight:`  `quote:`
```text
> note: RAM is slower because it is off-chip.
✓ Note saved · n1
> highlight: Registers can be accessed much faster than memory.
✓ Highlight saved · h1
> quote: "Simplicity is prerequisite for reliability."
✓ Quote saved · qt1
```
Highlights and quotes belong to the current source and location. Notes attach to the current context.

## Context

Context is what IDIOS remembers so you never repeat yourself: goal, source, chapter/page/section/time, concept, question.

| Input | Effect |
| --- | --- |
| `goal: Learn Assembly` | create or select a goal |
| `source: Ground Up` | select a source (a unique piece of the title is enough) |
| `chapter 2` · `page 14` · `section 2.3` · `time 12:30` | set the location (`time` is for videos) |
| `:forget` | forget the current concept and question |
| `:status` | show where you are and counts |

```text
> :status
IDIOS STATUS
Goal:      Learn Assembly
Source:    Programming from the Ground Up
Location:  Chapter 2
…
```
Quit and come back: IDIOS offers `Continue?` from where you stopped.

## Sources

Local files are **referenced, never copied**.

```text
> book: Programming from the Ground Up       ~ also article: paper: video: website:
✓ Source added and selected
Author (Enter to skip): Jonathan Bartlett
Path / URL (Enter to skip): ~/Shelf/Books/Assembly/pgu.pdf
```

| Input | Effect |
| --- | --- |
| `source add` | step-by-step: title, author, type, path/URL, shelf |
| `source: <name>` | switch to a registered source |
| `author: Jonathan Bartlett` | add an author to the current source |
| `category: Assembly` | shelf category for the current source |
| `tag: cpu, low-level` | tags for the current source |
| `url: https://…` · `path: ~/x.pdf` | where the current source lives |
| `:sources` · `:shelf` | list sources · the shelf grouped by type and category |

## Concepts and relations

```text
> concept: CPU Register
✓ Concept created
  linked to: What is a register?          ~ typed right after a question
> topic: Systems Programming               ~ files the current concept under a topic
> link CPU to CPU Register as contains
✓ Relation created
  CPU Register —related_to→ Memory
```
`link A to B` creates unknown names as concepts. `as <type>` is optional (default `related_to`):
`related_to` `depends_on` `uses` `contains` `part_of` `explains` `contrasts_with` `prerequisite_of`.

```text
> :graph CPU
CPU
└── contains → CPU Register
    └── related_to → Memory
> :concepts
```

## Planning

Tasks are small items with a day. `tk` ids.

```text
> did: Finished chapter 2 exercises          ~ done today
✓ Done saved · tk1
> plan: Read chapter 3                       ~ tomorrow is the default
✓ Planned · tk2
  Tomorrow · Sat 10 Oct
> plan monday: Review stack frames
> plan 2026-10-20: Exam
> plan +3d: Revisit pointers
> did yesterday: Read the paper              ~ log something you forgot
```
When words: `today` `tomorrow` `monday`…`sunday` (next one) `+Nd` or `YYYY-MM-DD`.

```text
> done tk2 tk4         ~ mark done (several at once)
✓ Marked done: Read chapter 3
> skip tk3             ~ drop it without doing it
> :plan                ~ overdue, today, tomorrow and the week ahead
Plan
Today · Fri 9 Oct
tk1   ✓ Finished exercises
Tomorrow · Sat 10 Oct
tk2   ○ Read chapter 3
```

## Review and reminders

The loop: **night** → `did:` + `plan:` → **next evening** reminder → `:review`.

| Input | Effect |
| --- | --- |
| `:review` | go through today's open tasks one by one |
| `:schedule` | show the review time and whether the reminder is on |
| `:schedule 21:30` | change the review time (24-hour) |
| `:schedule install` | shows the job, asks, then installs it |
| `:schedule remove` | removes only IDIOS's own job |
| `:schedule test` | sends a test notification now |

```text
> :review
tk2  Read chapter 3
  done? [y]es · [n]ot yet → tomorrow · [s]kip:
```
The result is saved as a note; "not yet" items move to tomorrow.

### What `:schedule install` actually does

It adds **one line** to your crontab (Task Scheduler on Windows):
```text
0 21 * * * /usr/bin/python3 -m idios remind # idios-reminder
```
Every day at 21:00 the OS runs `idios remind`: it sends a desktop notification listing your open
items, or prints text if no notifier exists, and stays silent if nothing is open. IDIOS itself runs no
background process. The next time you open `idios` after that time it also offers the review.

Notes:
- **WSL:** cron only runs while its service is up (`sudo service cron start`), and WSL must be running at 21:00.
  Notifications go to the Windows desktop through `powershell.exe` when available.
- **Linux desktop:** needs `notify-send` (package `libnotify-bin`).
- Check the line first with `idios schedule install --dry-run`; undo with `:schedule remove`.

## Looking things up

| Input | Shows |
| --- | --- |
| `:search register` | ranked results across everything (BM25, plain words) |
| `:show CPU Register` · `:show q1` | a source, concept, question or goal in detail |
| `:questions` · `:questions all` | open questions · every question |
| `:concepts` `:sources` `:goals` `:notes` `:highlights` `:quotes` | lists |
| `:graph [concept]` | relation tree |
| `:shelf` | sources by type and category |

```text
> :search register
1. Concept — CPU Register  [c1]
2. Question — What is a register? · Programming from the Ground Up  [q1]
3. Highlight — Registers are fast. · Programming from the Ground Up  [h1]
```

## Managing data

```text
> :delete q3
Delete Question q3? This also removes 1 answer. [y/N]
```
`:delete` always asks. Data lives in `~/.idios/idios.db` (SQLite). Export with `idios export` (below).

## Screen and help

| Input | Effect |
| --- | --- |
| `clear` · `cls` · `:clear` · Ctrl-L | clear the screen and redraw the context |
| `:help` | one-screen overview |
| `:help learn \| sources \| context \| concepts \| plan \| search \| manage \| examples` | topic with examples |
| `:intro` | replay the welcome banner |
| `:quit` · `quit` · `exit` · Ctrl-D | leave (everything is already saved) |

## Command line

```bash
idios                               # open the shell
idios search register               # search without opening the shell
idios show "CPU Register"
idios shelf
idios plan                          # today, tomorrow, the week
idios review                        # settle today's open tasks
idios remind                        # what the scheduler runs
idios schedule status|install|remove|test [--at 21:30] [--dry-run] [--yes]
idios export [-f markdown|json] [-o FILE]
idios run examples/01_first_session.idios     # play a script
idios --home /tmp/try …             # use another data folder
idios --version
```

## Environment variables

| Variable | Effect |
| --- | --- |
| `IDIOS_HOME` | data folder (default `~/.idios`) |
| `NO_COLOR=1` / `IDIOS_COLOR=always` | turn colour off / force it |
| `IDIOS_NO_LOGO=1` | hide the logo on interactive start |

## Cheat sheet

```text
LEARN     Why…?   answer: …   note: …   highlight: …   quote: …
CONTEXT   goal: …  source: …  chapter 2  page 14  section 2.3  time 12:30  :forget  :status
SOURCES   book: …  article: …  paper: …  video: …  website: …  source add  author: …  tag: …  url: …
GRAPH     concept: …  topic: …  link A to B [as type]  :graph [name]
PLAN      did: …  plan [when]: …  done tk1 tk2  skip tk3  :plan  :review  :schedule [HH:MM|install|remove|test]
FIND      :search …  :show X  :questions [all]  :concepts  :sources  :goals  :notes  :highlights  :quotes  :shelf
OTHER     :delete ID  clear  :help [topic]  :intro  :quit
```
