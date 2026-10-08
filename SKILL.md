# IDIOS

> A personal cognitive and learning operating system.

## 1. Purpose

IDIOS is a local-first personal learning environment.

It is the place where the user's learning activity lives:

* Books
* Articles
* Papers
* Videos
* Websites
* Authors / people
* Concepts
* Topics
* Categories
* Tags
* Questions
* Answers
* Goals
* Learning sessions
* Highlights
* Quotes
* Notes
* Relationships
* References to local files

IDIOS does **not** replace reading.

The user reads the source themselves and uses IDIOS while reading to record, connect, question, and retrieve what they learn.

The core system must work without:

* AI APIs
* LLMs
* Embedding APIs
* Cloud services
* External databases
* Internet access

AI may become an optional integration in the future, but it must never be required by the core system.

---

# 2. Core Philosophy

## 2.1 The user should think, not operate software

The biggest design principle is:

> The user must never need to remember IDIOS commands in order to learn.

Bad:

```bash
idios question add \
  "What is a register?" \
  --source "Programming from the Ground Up"
```

Good:

```text
$ idios

> What exactly is a register?
```

IDIOS should understand that this is a question and record it.

The CLI exists to support learning, not to turn learning into database administration.

---

## 2.2 Interactive first

The primary interface is:

```bash
idios
```

This launches the interactive learning environment.

The user can:

* ask questions
* answer questions
* add notes
* save highlights
* save quotes
* create concepts
* connect concepts
* register sources
* inspect knowledge
* search
* navigate their learning state

without memorizing a large command system.

---

## 2.3 Local-first

All important data belongs to the user.

IDIOS stores its own metadata and knowledge state locally.

External resources such as books and papers are referenced rather than copied.

Example:

```text
~/Shelf/
├── Books/
│   ├── Assembly/
│   ├── Linux/
│   └── Networking/
├── Papers/
├── Articles/
└── Videos/
```

IDIOS may store:

```text
source.path = ~/Shelf/Books/Assembly/book.pdf
```

It must not automatically copy the book into the repository.

---

# 3. Mental Model

IDIOS consists of five broad areas.

```text
SOURCES
    ↓
LEARNING
    ↓
KNOWLEDGE
    ↓
RELATIONSHIPS
    ↓
RETRIEVAL
```

## Sources

Things the user learns from:

* Book
* Article
* Paper
* Video
* Website
* Document

## Learning

The user's interaction with sources:

* Goal
* Session
* Question
* Answer
* Highlight
* Quote
* Note

## Knowledge

Things the user learns about:

* Concept
* Topic
* Person
* Author
* Category

## Relationships

Connections between entities:

```text
Assembly
    ↓
CPU
    ↓
Register
    ↓
Memory
```

## Retrieval

The ability to find previously recorded knowledge.

Search must work locally and deterministically.

---

# 4. Domain Model

## 4.1 Source

`Source` is the generic abstraction for anything the user learns from.

```text
Source
├── Book
├── Article
├── Paper
├── Video
└── Website
```

Common fields:

```text
id
title
type
authors
tags
categories
created_at
updated_at
description
```

Type-specific fields may include:

```text
Book
    path

Article
    url

Paper
    url
    path

Video
    url

Website
    url
```

A source can exist without a local file.

A local path is a reference, not ownership of the file.

---

# 5. Local File References

IDIOS must support references to files outside the project.

Example:

```text
~/Shelf/Books/Assembly/Programming_From_The_Ground_Up.pdf
```

The file itself is not part of IDIOS.

The metadata should contain a stable source ID and the current path.

Example:

```json
{
  "id": "src_001",
  "type": "book",
  "title": "Programming from the Ground Up",
  "path": "~/Shelf/Books/Assembly/Programming_From_The_Ground_Up.pdf"
}
```

Paths should be stored in a platform-aware way.

IDIOS should not assume that all users use Unix paths.

---

# 6. Goal

A goal represents something the user wants to learn.

Example:

```text
Learn Assembly
```

A goal may relate to:

* Sources
* Concepts
* Topics
* Questions
* Sessions

Example:

```text
Goal
└── Learn Assembly
    ├── Programming from the Ground Up
    ├── CPU
    ├── Registers
    ├── Memory
    └── 12 questions
```

---

# 7. Session

A session represents a period of active learning.

However:

> Sessions are implementation concepts, not user-facing bureaucracy.

The user should generally not need to manually create one.

Bad:

```bash
idios session start Assembly
```

Preferred:

```bash
idios
```

IDIOS infers or restores the active context.

Example:

```text
Goal: Learn Assembly
Source: Programming from the Ground Up
Chapter: 2
```

The session may internally contain:

```text
started_at
ended_at
goal
source
location
activity
```

The user should only interact with sessions explicitly when they need to inspect or manage them.

---

# 8. Question

A question is a first-class learning object.

Example:

```text
What is the difference between a register and memory?
```

A question may have:

```text
id
text
source
location
concepts
status
answer
created_at
updated_at
```

Statuses:

```text
open
answered
```

Questions may remain unanswered indefinitely.

That is valid.

---

# 9. Answer

An answer belongs to a question.

An answer may be added later.

Example:

```text
Question:
What is a register?

Answer:
A small storage location directly inside the CPU.
```

The answer does not have to be AI-generated.

In the core system, answers are human-created.

---

# 10. Highlight

A highlight is an excerpt the user wants to retain from a source.

Example:

```text
Registers are much faster than memory...
```

A highlight should preserve source context when available:

```text
source
chapter
page
section
location
text
```

---

# 11. Quote

A quote is a short passage that the user especially wants to preserve.

Quote and highlight are intentionally separate concepts.

```text
Highlight
    = useful source excerpt

Quote
    = memorable / meaningful passage
```

---

# 12. Note

A note is user-authored knowledge that is not necessarily a direct source excerpt.

Example:

```text
Registers are CPU-local storage while RAM is external main memory.
```

Notes may reference:

* Sources
* Concepts
* Questions
* Goals
* Sessions

---

# 13. Concept

A concept represents something the user wants to understand.

Example:

```text
CPU Register
```

A concept may be connected to:

```text
Sources
Questions
Notes
Highlights
Quotes
Other concepts
Topics
```

Example:

```text
CPU Register

Sources:
  Programming from the Ground Up
  Computer Systems

Questions:
  What is a register?
  Why are registers faster?

Related:
  CPU
  Memory
  Instruction
  Assembly
```

---

# 14. Topic

A topic groups knowledge conceptually.

Example:

```text
Programming
└── Systems Programming
    └── Assembly
```

Topics may be hierarchical.

Topics are organizational structures.

They are not the same thing as semantic graph relationships.

---

# 15. Category

Categories provide classification.

Example:

```text
Programming
├── Low Level
│   └── Assembly
├── Networking
└── Web
```

Categories may be hierarchical.

Do not use categories as a replacement for graph relationships.

---

# 16. Tag

Tags are lightweight labels.

Example:

```text
assembly
cpu
low-level
linux
x86
```

Tags should remain simple.

Do not turn tags into a complex ontology.

---

# 17. Person / Author

People should be first-class entities.

Example:

```text
Jonathan Bartlett
```

A person may have:

```text
books
articles
papers
topics
concepts
notes
```

An author is a person associated with a source.

Avoid duplicating person records unnecessarily.

---

# 18. Knowledge Graph

IDIOS maintains a graph connecting entities.

Example:

```text
Assembly
   │
   ├── uses ──> CPU
   │             │
   │             └── contains ──> Register
   │
   └── interacts_with ──> Memory
```

Graph edges should be typed.

Examples:

```text
related_to
depends_on
uses
contains
part_of
explains
contrasts_with
prerequisite_of
```

The graph must remain lightweight.

Do not build a giant ontology engine.

---

# 19. Search

Search is a core feature.

It must be:

* local
* deterministic
* fast
* understandable
* independent from AI

BM25 is an appropriate baseline.

Search should cover:

* Sources
* Concepts
* Questions
* Answers
* Notes
* Highlights
* Quotes
* Goals
* People
* Tags
* Topics

Example:

```bash
idios search "register"
```

Results may include:

```text
1. CPU Register
2. Programming from the Ground Up
3. Question: Why are registers faster?
4. Highlight: ...
5. Note: ...
```

---

# 20. Interactive Environment

Running:

```bash
idios
```

must open the primary learning interface.

Example:

```text
IDIOS
────────────────────────────────────

Goal: Learn Assembly
Source: Programming from the Ground Up
Chapter: 2

>
```

The user can type natural learning input.

### Question

```text
> Why does the CPU need registers?
```

IDIOS:

```text
✓ Question saved

Why does the CPU need registers?
Source: Programming from the Ground Up · Chapter 2
Status: open
```

### Answer

```text
> Registers are fast CPU-local storage.
```

IDIOS:

```text
✓ Answer recorded
```

### Highlight

```text
> highlight: Registers provide extremely fast access...
```

### Quote

```text
> quote: "..."
```

### Note

```text
> note: RAM is slower than registers.
```

### Concept

```text
> concept: CPU Register
```

### Relation

```text
> link CPU Register to Memory
```

---

# 21. Directives

Natural input is the default.

Explicit directives may be provided for operations where ambiguity is undesirable.

Use a small prefix:

```text
:help
:search
:sources
:questions
:concepts
:highlights
:quotes
:notes
:graph
:status
:quit
```

These directives are navigation/control tools.

They should not be required for normal learning.

---

# 22. Context

The interactive environment maintains context.

Possible context:

```text
Goal
Source
Chapter
Page
Section
Current concept
Current topic
Current session
```

Example:

```text
Goal: Learn Assembly
Source: Programming from the Ground Up
Chapter: 2
```

Then:

```text
> highlight this
```

should use the current source context when possible.

Context should reduce repetitive input.

---

# 23. Context Changes

The user may explicitly change context:

```text
> source Programming from the Ground Up
```

or:

```text
> chapter 3
```

or:

```text
> goal Learn Assembly
```

The exact syntax may evolve.

The important requirement is:

> Context must be cheap to change.

---

# 24. Learning Workflow

A typical workflow:

```text
$ idios

Goal: Learn Assembly
Source: Programming from the Ground Up
Chapter: 2

> Why are registers necessary?

✓ Question saved

> I think because they're much faster than RAM.

✓ Answer saved

> highlight: ...

✓ Highlight saved

> concept: CPU Register

✓ Concept created

> link CPU Register to Memory

✓ Relation created
```

The user continues reading.

No command memorization should be necessary.

---

# 25. Shelf

The user's actual files may live outside IDIOS.

Example:

```text
~/Shelf/
├── Books/
├── Papers/
├── Articles/
└── Videos/
```

IDIOS provides a logical view:

```bash
idios shelf
```

Example:

```text
BOOKS

Assembly
  1. Programming from the Ground Up
  2. Modern X86 Assembly Language Programming

Linux
  3. The Linux Programming Interface

Networking
  4. Computer Networking
```

The shelf is a logical index, not a file manager.

---

# 26. Obsidian Compatibility

IDIOS may integrate with Markdown and Obsidian.

However:

> Obsidian is not a required runtime dependency.

IDIOS should remain functional without Obsidian.

Possible future capabilities:

```text
export markdown
import markdown
sync markdown
```

Do not make synchronization a core requirement in the first implementation.

---

# 27. Storage

The first implementation should favor simplicity.

A local SQLite database is preferred if relational querying and graph relationships become cumbersome with JSON.

Alternative:

```text
JSON
```

is acceptable for a very early prototype.

Do not introduce:

* PostgreSQL
* Redis
* Elasticsearch
* Neo4j
* cloud databases

for the core application.

IDIOS is a personal local application.

---

# 28. No AI

The following are explicitly outside the core:

```text
OpenAI
Gemini
Qwen
Claude
NotebookLM
Embedding APIs
LLM agents
Cloud inference
Vector databases
```

Do not design the core around them.

Future integrations may exist, but the system must remain complete without them.

---

# 29. Non-Goals

IDIOS is not:

* a social network
* a cloud knowledge platform
* an AI chatbot
* a document storage system
* a generic project management tool
* a full note-taking application
* an enterprise LMS
* a replacement for books
* an automatic learning machine

The user remains the learner.

IDIOS is the environment that records and organizes the learning process.

---

# 30. Design Rule

Whenever a feature is proposed, ask:

> Does this make learning easier, or does it make operating IDIOS more complicated?

If it makes operation more complicated without a meaningful learning benefit, reject it.

The system should feel small.

The data model may be rich.

The interface must remain simple.
