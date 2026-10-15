# IDIOS

> A terminal notebook for learning. You read; IDIOS keeps your questions, answers, notes, highlights and ideas, and connects them.

```text
$ idios

Welcome back.

Last context:
Goal: Learn Assembly
Source: Programming from the Ground Up
Location: Chapter 2

Continue? [Y/n]

> Why does the CPU need registers?
✓ Question saved · q1

> Because registers provide very fast CPU-local storage.
✓ Answer saved · a1

> highlight: Registers can be accessed much faster than memory.
✓ Highlight saved · h1

> concept: CPU Register
✓ Concept created

> link CPU Register to Memory
✓ Relation created
```

Everything is stored in one SQLite file on your machine.

## Install

```bash
pip install .          # or: make setup for a dev environment
pip install ".[notifications]"  # optional desktop provider for Linux/macOS
idios
```

Requires Python 3.10+. Data lives in `~/.idios/` (set `IDIOS_HOME` to move it).

## Using it

Just type. IDIOS recognises a few explicit forms and files everything under your current context (goal, source, chapter/page), so you never repeat `--source`.

| You type | IDIOS saves |
|---|---|
| `Why is memory slower?` | a question |
| `Because it is off-chip.` / `A small storage…` | an answer to the open question |
| `note: …` `highlight: …` `quote: …` | a note / highlight / quote |
| `concept: CPU Register` | a concept (and selects it) |
| `link A to B` or `link A to B as contains` | a typed relation (unknown names become concepts) |
| `goal: …` `source: …` `book: …` `article: …` `paper: …` `video: …` `website: …` | sets context, registering what is new |
| `chapter 2` `page 14` `section 2.3` `time 12:30` | your location in the source |
| `author: …` `category: …` `tag: a, b` `topic: …` | metadata for the current source / concept |
| `answer q3: …` | an answer to an older question |

When a line could mean more than one thing, IDIOS asks:

```text
> registers are important
What should I save this as?

1. Note
2. Answer to current question
3. Ignore
```

Relation types: `related_to` `depends_on` `uses` `contains` `part_of` `explains` `contrasts_with` `prerequisite_of`.

Full reference with examples: [docs/COMMANDS.md](docs/COMMANDS.md).

### Directives

```text
:help  :status  :plan  :review  :schedule  :search <query>  :sources  :questions [all]  :concepts
:highlights  :quotes  :notes  :goals  :graph [concept]  :shelf
:show <name or id>  :delete <id>  :forget  clear  :intro  :quit
```

`:show` works for sources, concepts, questions and goals and accepts a name or a short id such as `q3`, `c1`, `s2`. `:delete` always asks first.

`source add` walks you through title, author, type, path/URL and shelf. Local files are **referenced, never copied**.

`url: https://…` and `path: ~/Shelf/book.pdf` set where the current source lives.

### The evening loop (did / plan / remind / review)

Tell IDIOS at night what you did, plan tomorrow, and let it ask you the next evening what got done.

```text
> did: Finished chapter 2 exercises      ~ done today
> plan: Read chapter 3                   ~ tomorrow (default)
> plan monday: Review stack frames       ~ today, tomorrow, monday…sunday, 2026-10-12, +3d
> :plan                                  ~ today and tomorrow
> done tk2 tk4   /   skip tk3
> :schedule 21:00                        ~ review time
> :schedule install                      ~ asks first, then adds a cron / Task Scheduler job
> :review                                ~ done? [y]es · [n]ot yet → tomorrow · [s]kip
```

At the review time the OS job runs `idios remind` and sends a native desktop notification. Click it to open a dark, card-based schedule dashboard with overdue, today, completed, skipped and upcoming items, colored counters, the daily review time and linked goal/source context. The window is read-only.

- **Windows 10:** IDIOS uses WinRT Toast notifications and registers a per-user `idios:` link so clicking the toast can open the schedule even after the reminder job exits. The Windows path uses built-in PowerShell/WinRT and does not need the optional provider extra. Task Scheduler runs the reminder only while you're logged in.
- **Linux/BSD:** `desktop-notifier` is preferred when installed; `notify-send` is the fallback. From the source checkout, install `pip install ".[notifications]"` for the optional provider. `notify-send` may be supplied by `libnotify-bin`.
- **macOS:** from the source checkout, install the optional provider with `pip install ".[notifications]"`; `terminal-notifier` can also provide click-to-open activation. Native Notification Center delivery depends on macOS notification permissions and a supported/signed Python build; see the [desktop-notifier macOS notes](https://github.com/samschott/desktop-notifier#notes-on-macos).
- **Schedule window:** IDIOS uses a reusable dark Tkinter theme for its wordmark, cards, buttons, status chips and scrollable sections. Tkinter is included with the standard Windows Python installer and many Python builds; on minimal Linux installations you may need the OS package `python3-tk`.
- **WSL:** interactive notifications need WSLg/DBus or another working desktop notification provider. Cron must be running, and WSL must be active at the scheduled time.

If no interactive notification backend is available, IDIOS prints the reminder instead. When you next open IDIOS interactively it can offer the review, save the result as a note and move unfinished items to tomorrow. Nothing is installed without your confirmation; `idios schedule install --dry-run` shows the exact scheduler command. Outside the shell: `idios plan`, `idios review`, `idios remind`, `idios schedule status|install|remove|test`.

### First run and clearing the screen

The very first time you start `idios` at a terminal it shows a welcome banner (ASCII logo, what it is, three things to try) and waits for Enter. It appears once; `:intro` replays it. Every later interactive start shows the logo with a one-line tagline (`IDIOS_NO_LOGO=1` hides it).

`clear` (or `cls`, `:clear`, or Ctrl-L) wipes the screen and redraws your goal/source/location. It changes no data. To forget the current concept and question instead, use `:forget`.

### Help with examples

`:help` is one screen. Each topic adds worked examples you can copy:

```text
:help learn   :help sources   :help context   :help concepts
:help plan    :help search  :help manage    :help examples
```

### Colour

Colour is automatic on a terminal and off when piped. `NO_COLOR=1` disables it, `IDIOS_COLOR=always` forces it.

### Outside the shell

```bash
idios search register
idios show "CPU Register"
idios shelf
idios export --format markdown   # or json; written to ~/.idios/exports/
idios run examples/01_first_session.idios   # play a script as a transcript
idios --home /tmp/try run …                 # use a throwaway data folder
```

## Examples

[`examples/`](examples/README.md) has runnable sessions: a first session, a paper, a video course, building a graph, what happens when IDIOS is unsure, the Python API, and the evening loop.

```bash
./examples/run.sh --all     # or: make examples
```

## Architecture

```text
cli → shell (parser · render · io) → services → domain
                                         ↓
                                 storage (SQLite)
```

- `domain/` typed dataclasses, ULID-style ids, user-facing errors
- `storage/` schema (`PRAGMA user_version`) and repositories — the only SQL
- `services/` context & sessions, learning, knowledge & graph, sources & shelf, BM25 search, views, export
- `shell/` input parser, interpreter loop, text rendering

Rules from `AGENTS.md` are enforced by `tests/test_architecture.py`.

## Project health with psx

The shape of this repository (README, license, tests, CI, security policy, …) is checked by [psx](https://github.com/m-mdy-m/psx). Rules live in [`psx.yml`](psx.yml).

```bash
psx check            # read-only report
psx fix --dry-run    # preview anything missing
make psx             # same as psx check --fail-on error
```

CI runs it on every push, and `tests/test_experience.py` verifies that every file `psx.yml` expects exists.

## Development

```bash
make setup && source .venv/bin/activate
make check           # lint + tests
make help            # all targets
```

MIT licensed.
