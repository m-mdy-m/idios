# AGENTS.md

## Project

IDIOS is a local-first personal cognitive and learning operating system.

Its purpose is to provide one place for the user's learning sources, questions, answers, concepts, notes, highlights, quotes, goals, sessions, and relationships.

The project must remain simple, local, deterministic, and usable without AI.

---

# 1. Primary Engineering Principles

## 1.1 Simplicity over abstraction

Do not introduce abstractions unless they solve a real problem.

Prefer:

```text
simple domain model
simple storage
simple commands
simple state
simple UI
```

Avoid architecture for architecture's sake.

---

## 1.2 Local-first

IDIOS must work completely offline.

Do not require:

* API keys
* network connectivity
* cloud accounts
* remote databases
* AI services

Tests must be runnable offline.

---

## 1.3 Interactive-first

The main user interface is:

```bash
idios
```

The user should be able to perform common learning actions interactively.

Do not force the user to memorize commands such as:

```bash
idios question add ...
idios answer add ...
idios highlight add ...
```

Those operations may exist internally or as secondary CLI commands, but the interactive environment is primary.

---

# 2. Core User Experience

The user starts:

```bash
idios
```

The system restores or establishes context.

Example:

```text
Goal: Learn Assembly
Source: Programming from the Ground Up
Chapter: 2

>
```

The user types:

```text
> What is a register?
```

The system records a question.

The user types:

```text
> A small storage location inside the CPU.
```

The system records an answer when appropriate.

The user types:

```text
> highlight: ...
```

The system creates a highlight.

The user types:

```text
> note: ...
```

The system creates a note.

The user types:

```text
> concept: CPU Register
```

The system creates a concept.

---

# 3. Do Not Build a Chatbot

Interactive input must not imply that IDIOS needs an LLM.

The interpreter should be deterministic.

Use:

* explicit directives
* lightweight command recognition
* context
* predictable parsing

Do not add an LLM parser.

Do not call external AI APIs to understand commands.

---

# 4. Input Model

The interactive shell should support three types of input.

## 4.1 Directives

Prefixed with `:`.

Examples:

```text
:help
:search register
:sources
:questions
:concepts
:status
:quit
```

Directives are deterministic.

---

## 4.2 Explicit actions

Examples:

```text
highlight: registers are...
quote: ...
note: ...
concept: CPU Register
link CPU Register to Memory
```

These should be easy to recognize.

---

## 4.3 Natural learning statements

Example:

```text
Why does the CPU need registers?
```

This should become a question.

Example:

```text
I think registers are faster because they are inside the CPU.
```

This can become a note or answer depending on active context.

If classification is ambiguous, ask the user.

Do not guess silently when doing so could create incorrect data.

---

# 5. Ambiguity

When input is ambiguous:

```text
> registers are important
```

Do not invent a complex interpretation.

Ask:

```text
Save as:
1. Note
2. Answer to current question
3. Ignore

>
```

The user should be able to choose quickly.

---

# 6. Context

Context is essential.

Maintain:

```text
current_goal
current_source
current_location
current_concept
current_topic
current_session
```

Context should be persisted when useful.

Example:

```text
Source:
Programming from the Ground Up

Location:
Chapter 2
```

Then:

```text
> highlight this
```

can use the current source.

---

# 7. Session Behavior

Sessions are internal learning state.

Do not require:

```bash
idios session start
```

for normal usage.

Starting IDIOS should restore the previous context where possible.

Example:

```text
Welcome back.

Goal: Learn Assembly
Source: Programming from the Ground Up
Location: Chapter 2

Continue? [Y/n]
```

Session creation should happen automatically.

---

# 8. Domain Entities

Core entities:

```text
Source
Book
Article
Paper
Video
Website

Goal
Session

Question
Answer
Note
Highlight
Quote

Concept
Topic
Category
Tag

Person
Author

Relation
```

Do not create additional entity types unless there is a concrete use case.

---

# 9. Source Model

Use a generic source abstraction.

```text
Source
├── Book
├── Article
├── Paper
├── Video
└── Website
```

Shared source functionality should not be duplicated across individual source types.

---

# 10. File References

Never copy user books, papers, or other large external resources into IDIOS automatically.

Store references:

```text
path
url
```

Example:

```text
~/Shelf/Books/Assembly/book.pdf
```

The system must treat these as references.

---

# 11. Knowledge Graph

Graph relationships must be explicit.

Example:

```text
CPU
  └── contains
       └── Register
```

Do not use tags to represent semantic relationships.

Do not use categories to represent semantic relationships.

Use graph edges.

---

# 12. Search

Search must be local.

BM25 is the preferred initial implementation.

Do not add vector search just because it sounds modern.

Do not add embeddings without a demonstrated need.

Search should operate across relevant entities.

---

# 13. Storage

Keep storage local.

Preferred implementation:

```text
SQLite
```

unless a simpler storage mechanism is demonstrably better.

Use migrations only when necessary.

Do not introduce external database infrastructure.

---

# 14. IDs

Every persistent entity should have a stable ID.

IDs should not depend on titles.

Bad:

```text
id = "CPU Register"
```

Good:

```text
concept_01H...
```

Titles may change.

IDs should not.

---

# 15. Timestamps

Persistent records may contain:

```text
created_at
updated_at
```

Use consistent timezone handling.

Do not make timestamps part of the user experience unless useful.

---

# 16. Source Locations

A source interaction may contain:

```text
chapter
page
section
paragraph
timestamp
```

Only store fields that are available.

Do not require exact locations when the user does not provide them.

---

# 17. Error Handling

Errors should be human-readable.

Bad:

```text
ValueError: invalid enum: SOURCE_CONTEXT
```

Good:

```text
Could not find that source.

Try:
  :sources
```

Never dump internal implementation details during normal interactive use.

---

# 18. Data Safety

Never silently delete learning data.

For destructive operations:

```text
Delete Question #12?

[y/N]
```

Provide safe alternatives where possible.

---

# 19. CLI Design

The CLI should have a small surface.

Possible commands:

```text
idios
idios search <query>
idios show <id>
idios shelf
idios export
```

The interactive shell handles most operations.

Do not create dozens of top-level commands.

---

# 20. Interactive Commands

Use directives:

```text
:help
:status
:search
:sources
:questions
:concepts
:highlights
:quotes
:notes
:graph
:goals
:shelf
:quit
```

These are examples, not a requirement to implement every directive immediately.

---

# 21. Help

`:help` should show only useful commands.

Example:

```text
IDIOS

Learning:
  Ask a question normally.
  note: ...
  highlight: ...
  quote: ...
  concept: ...
  link A to B

Navigation:
  :search <query>
  :sources
  :questions
  :concepts
  :status

Exit:
  :quit
```

Do not print a giant manual.

---

# 22. UX Feedback

Every successful mutation should provide concise feedback.

Example:

```text
✓ Question saved
```

Not:

```text
QuestionEntity successfully persisted to SQLite database.
```

Internal terminology should not leak into normal UI.

---

# 23. Architecture

A simple architecture is preferred:

```text
CLI
 │
 ▼
Interactive Shell
 │
 ▼
Application Services
 │
 ├── Sources
 ├── Learning
 ├── Knowledge
 ├── Graph
 └── Search
 │
 ▼
Storage
```

Keep domain logic independent from terminal rendering.

---

# 24. Separation of Concerns

Do not put business logic directly inside CLI handlers.

Prefer:

```text
CLI
 → application service
 → domain
 → repository
```

The exact architecture may remain lightweight.

Do not create dozens of interfaces without need.

---

# 25. Testing

Tests should cover:

## Domain

* source creation
* question creation
* answer creation
* highlights
* quotes
* concepts
* relationships

## Context

* restore context
* change source
* change goal
* current location

## Search

* indexing
* BM25 ranking
* filtering
* entity retrieval

## Interactive shell

* question input
* note input
* highlight input
* directives
* ambiguity handling

## Persistence

* save
* reload
* update
* deletion safety

---

# 26. No AI in Tests

Tests must never depend on:

* OpenAI
* Gemini
* Qwen
* Claude
* remote APIs

The complete test suite must work offline.

---

# 27. Development Priority

Implement in this order:

```text
1. Storage
2. Domain model
3. Sources
4. Goals
5. Context
6. Questions / Answers
7. Notes
8. Highlights / Quotes
9. Concepts
10. Relationships
11. Search
12. Interactive shell
13. Shelf
14. Export
15. Polish
```

Do not implement everything simultaneously.

Build a working vertical slice early.

---

# 28. First Vertical Slice

The first usable version should support:

```text
idios
```

Then:

```text
> source Programming from the Ground Up
```

```text
> goal Learn Assembly
```

```text
> What is a register?
```

```text
> A small storage location inside the CPU.
```

```text
> note: registers are much faster than RAM
```

```text
> concept: CPU Register
```

```text
> :questions
```

and:

```text
> :search register
```

If this works reliably, the core idea is alive.

---

# 29. What Not To Do

Do not:

* add an AI dependency
* build a cloud backend
* add authentication
* build multi-user support
* create a web frontend before the core works
* build a vector database
* build a complex agent system
* require Obsidian
* copy source files
* create a huge command hierarchy
* overengineer the graph
* implement speculative features
* optimize before usage reveals a problem

---

# 30. Final Engineering Rule

When uncertain between two implementations:

> Choose the smaller implementation that preserves the user's learning experience.

IDIOS should feel like a small personal tool with a deep internal model.

Not like a giant platform.
