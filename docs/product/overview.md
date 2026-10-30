# What is IDIOS?

IDIOS is a local-first workspace for learning, planning, and development. It puts project records, reading material, notes, ideas, schedules, and optional AI assistance in one application on your computer.

You can use IDIOS in two ways:

- **CLI:** run commands from a terminal or use them in scripts.
- **Web:** open the local workspace in your browser.

Both interfaces use the same Go application and the same local SQLite database.

## What can you do?

| Goal | Start in IDIOS |
|---|---|
| Organize a codebase or long-running effort | Repositories and Projects |
| Break work into small steps | Issues, Labels, and Milestones |
| Record completed work | IDIOS Pull Requests and Activity |
| Learn from books and PDFs | Shelf, Notes, and Highlights |
| Connect ideas | Concepts and Graph |
| Plan a day | Daily Plans |
| Automate supported local work | Scheduled Jobs and Actions Center |
| Ask questions about reading material | AI Runtime and Shelf assistants |

See the [feature map](features.md) for a description and important limits of each area.

## How the pieces work together

A Repository is a workspace container. Issues describe work inside it. A Project can group Issues from several Repositories. When work is complete, an IDIOS Pull Request records the result. Activity keeps a timeline of supported events.

Shelf is a separate learning workflow. It connects book folders already on your computer. You can read PDFs, save annotations, write notes, and connect a book to a Repository when reading leads to a project.

Daily Plans and Scheduled Jobs help organize time and supported local actions. They are processed by the IDIOS local service, so that service must be running when scheduled work is due.

## What IDIOS is not

- It is not an operating-system kernel or a Linux/BSD distribution.
- An IDIOS Repository is not a Git repository and does not clone, commit, push, or create remote branches.
- An IDIOS Pull Request is not a GitHub Pull Request and does not merge code.
- An AI response is not a saved record until IDIOS confirms that the save succeeded.
- A scheduled local action cannot run while the computer and IDIOS service are stopped.

## Where your data lives

Workspace records are stored in a local SQLite database. Book PDFs stay in their connected folders. AI requests go only to providers allowed by your Settings → AI privacy mode. See [Privacy and local data](privacy.md) for details.
