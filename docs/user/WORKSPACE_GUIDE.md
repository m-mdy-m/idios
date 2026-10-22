# How the IDIOS workspace works

IDIOS is a place to describe and organize work. It can sit beside your source-code folders, but it does not operate on Git repositories or send work to GitHub/GitLab.

## Main items

### Repositories

A repository is a named group of work inside IDIOS. It has its own issues, labels, milestones, activity, and settings. Creating one does not clone a Git repository or create files on disk.

### Issues

An issue describes one piece of work. Give it a title, details, priority, and due date when useful. Large issues can have child issues. Issues can be open, in progress, done, or cancelled. Each issue also carries a type — `task`, `bug`, `feature`, or `research` — so delivered work can be counted by kind.

An issue can depend on another issue. For example, `Write tests` can depend on `Implement parser`. IDIOS marks that dependency as a blocker; other knowledge links do not automatically block work.

### IDIOS pull requests

An IDIOS pull request records the work done for an issue and the explanation of the result. You can update it while working and accept it when ready. Accepting it creates a completion record and marks the issue done. One simulated pull request can also cover several in-progress issues of the same repository; accepting it completes them together and records one immutable simulated commit.

**This is not a GitHub pull request.** IDIOS has no Git binary and does not merge real code or touched files; the repository history, virtual files, and release tags you see inside IDIOS are records the IDIOS Engine stores itself. See [Simulated repository workflow](SIMULATED_REPOSITORY_WORKFLOW.md).

### Projects

A project groups related issues, including issues from different IDIOS repositories. Use a project for a larger goal such as a release, migration, or research effort.

### Notes and labels

Notes hold decisions, ideas, investigation results, or context you want to keep. A note can have several comma-separated labels, such as `design, decision, research`. Labels make notes easier to search and group. Where supported, a note can also link to another workspace item.

### Scheduled jobs and notifications

Jobs can run once at a chosen date and time, at an interval, daily, weekly, or monthly. You can pause a schedule, enable it again, run it immediately, edit it, or delete it. Scheduled work is processed while the terminal UI or the installed background service is running.

Notifications collect reminders and other events. Browser notification pop-ups also depend on your browser's permission settings.

### Daily plans

A plan is a dated checklist for a day: a title, a description, and an ordered list of completable tasks. A plan can stay personal or link to a repository or issue. Morning, midday, and evening reminders can check in on the plans of the current date, and the Activity Atlas colors days by completed task ratio. See [Daily Plans](DAILY_PLANS.md).

### Concepts and graph

Concepts are reusable knowledge items. The graph shows how issues, repositories, concepts, and other supported items are connected. A link in the graph is not necessarily a blocker: issue dependencies are the relationships that control whether blocked work can start.

## A simple workflow

1. Create a repository to hold a related set of issues.
2. Create an issue with a clear title and a useful description.
3. Add child issues if the work needs to be split up.
4. Add dependencies when one task must wait for another.
5. Start the issue when work begins.
6. Create an IDIOS pull request to record the result.
7. Accept it when the work is complete. IDIOS then adds a completion record and marks the issue done.

You can also create a project for a cross-repository goal, and keep supporting decisions in labelled notes.

## Custom templates

IDIOS can use configurable templates for repository descriptions, issue forms, and pull-request records. The templates are stored in the settings file. See [Settings and data](SETTINGS.md).
