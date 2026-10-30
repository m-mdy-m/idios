# IDIOS feature map

This page explains what each part of the application is for. The route is the local Web address for that page. You can also use the navigation on the left side of the workspace.

| Area | What it is for | Web route |
|---|---|---|
| Dashboard | See a short summary of your workspace and recent activity. | `/` |
| Repositories | Keep related Issues, labels, milestones, and activity together. | `/repositories` |
| Issues | Track a task, bug, feature, or research item from start to completion. | `/issues` |
| Projects | Group related Issues from one or more Repositories. | `/projects` |
| Pull Requests | Record and accept completed work inside IDIOS. | `/pull-requests` |
| Concepts | Save ideas and knowledge items. | `/concepts` |
| Graph | Explore links between Concepts. | `/graph` |
| Shelf | Connect book folders, read PDFs, and keep reading progress. | `/shelf` |
| Notes | Write and organize your own notes. | `/notes` |
| Quotes | Save excerpts and quotations for later use. | `/quotes` |
| Daily Plans | Make a dated checklist for the day. | `/plans` |
| Labels and Milestones | Classify Issues and group work toward a goal. | `/planning` |
| Scheduled Jobs | Run supported local actions on a schedule. | `/jobs` |
| Actions Center | Find and run operations that IDIOS exposes as actions. | `/commands` |
| Activity | Review supported changes and events from your workspace. | `/activity` |
| Notifications | Review reminders and application notifications. | `/notifications` |
| Profile | Manage your workspace profile and pinned items. | `/profile` |
| Remove Items | Review and remove supported workspace records. | `/remove` |
| Logs | Inspect local application diagnostics. | `/logs` |
| Help | Find application help and shortcuts. | `/help` |
| Settings | Change application, AI, notification, and update settings. | `/settings` |
| AI Settings | Configure providers, models, agents, and privacy behavior. | `/settings/ai` |

## How the areas connect

A Repository contains Issues. A Project can group Issues from different Repositories. Labels and Milestones help organize Issues. When a piece of work is complete, an IDIOS Pull Request can record and accept the result. Activity and Notifications help you review what happened.

Shelf, Notes, Quotes, Concepts, and Graph support learning and knowledge work. A book can have reading progress, PDF annotations, questions, and notes. Concepts can be connected to each other in Graph. You can connect a book to a Repository when reading leads to a practical task.

Daily Plans help you decide what to do. Scheduled Jobs run supported local actions while the IDIOS service is running.

## Important limits

- An IDIOS Repository is not a Git repository. It does not clone code or push commits.
- An IDIOS Pull Request is not a GitHub Pull Request. It does not merge source code.
- Scheduled Jobs run on the computer that runs IDIOS. They do not run while the computer or local service is stopped.
- AI features depend on the provider, model, and privacy mode you have configured.

For the meaning of the names used in IDIOS, read [Terms and concepts](terms.md). For technical ownership and source paths, read [Architecture](../arch/README.md).
