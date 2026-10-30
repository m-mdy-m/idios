# Workspace guide

IDIOS stores work records in one local workspace database. The names below describe IDIOS concepts; they do not perform Git operations by themselves.

## Repositories

A Repository groups Issues, branches shown by the IDIOS workflow, labels, milestones, virtual files, settings, and activity. It is useful for one codebase, subject, or long-running effort.

An IDIOS Repository is **not** a Git repository. Creating one does not clone a directory, create a remote, push files, or make a real branch.

## Issues

An Issue is one unit of work. It can have a description, priority, due date, type, parent Issue, labels, milestone, and dependencies.

The available types include Task, Bug, Feature, and Research. The `depends_on` relation is the blocking workflow dependency. Other graph relations describe knowledge or context and do not automatically block an Issue.

Typical status flow:

1. Create an Issue.
2. Start it when work begins.
3. Record the result in an IDIOS Pull Request.
4. Accept the Pull Request when the work is complete.

An Issue is completed by accepting its IDIOS Pull Request; manually starting an Issue does not mark it done.

## Pull Requests and completions

An IDIOS Pull Request is an in-app review and completion record. It is not a GitHub Pull Request and does not merge a branch. It can group in-progress Issues from the same IDIOS Repository. Accepting it writes a completion record to IDIOS history.

## Projects

A Project groups Issues across Repositories. Use it for a larger goal that spans several workspaces.

## Knowledge and organization

- **Concepts** are reusable knowledge nodes.
- **Graph** shows typed relationships between supported entities.
- **Labels** classify Issues and other supported records.
- **Milestones** group work around a target or delivery point.
- **Notes** keep longer thoughts and working material.
- **Quotes** keep excerpts worth returning to.

## Activity, jobs, and actions

- **Activity** is a timeline of workspace events and learning activity.
- **Daily Plans** are dated checklists.
- **Scheduled Jobs** run supported actions on a schedule while the local service is active.
- **Actions Center** exposes available workspace actions.
- **Remove Items** provides a controlled place for destructive removal.
- **Logs** and **Help** expose diagnostics and in-app guidance.

See [CLI](cli.md) for terminal commands and [Routes](../reference/routes.md) for the page list.
