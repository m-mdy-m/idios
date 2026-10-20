# Repository, Issue and Pull Request templates

IDIOS templates make new work records more useful from the beginning. They provide a repeatable structure for context, goals, acceptance criteria, verification and risk—not just a short description box. Templates are loaded from the user configuration and copied into a new record when it is created.

## What the default templates look like

The bundled templates are short on purpose: each section has a one-line prompt you overwrite, and fill-in lines such as `Resolves:`, `Blocked by:` and `Impact: low / medium / high` take a value instead of a paragraph. Delete any section that does not apply.

- **Issue templates** (`task`, `bug`, `feature`, `research`) all end with a checkable **Done when** list (or **Deliverable** for research), so an Issue can be accepted against something concrete. The `bug` template keeps the `Expected behavior` and `Steps to reproduce` headings.
- **Pull request template**: Summary (with `Resolves:` and complete/partial scope), Changes, Verification (with a block for the real command output), Risk (impact, what could break, how to undo) and Review focus.

## Where templates live

The TUI configuration is normally:

- Windows: `%USERPROFILE%\.idios\config.json`
- Linux / macOS: `~/.idios/config.json`
- Custom path: set `IDIOS_CONFIG` to the file you want IDIOS to use.

The first TUI launch creates this file with defaults. IDIOS merges a user's partial configuration over the packaged defaults, so **you only need to edit the sections you want to customize**. Keep the file valid JSON, including commas and double quotes. Restart IDIOS after editing so the new templates are loaded.

IDIOS uses these configuration keys, with the exact spelling shown:

- `REPOSITORY_TEMPLATE`
- `ISSUE_TEMPLATE`
- `PULL_REQUEST_TEMPLATE`

`PULL_REQUEST_TEMPLATE` is spelled **REQUEST**, not `REUQEST`.

These settings control the local IDIOS TUI. They are separate from the GitHub repository files under `.github/ISSUE_TEMPLATE/` and `.github/PULL_REQUEST_TEMPLATE.md`, which are used by GitHub's own web interface. Those GitHub files are for contributors to this repository; they carry the project's checks (`make check`, `make smoke`), compatibility checklist (schema, JSON protocol, layering) and bug-report fields (version, where it happens, steps).

## Repository overview template

When you create a Repository, IDIOS fills its multiline **README / repository overview** field from `REPOSITORY_TEMPLATE.description`. The initial scaffold includes Purpose, Scope, Getting started, and Architecture/decisions. The literal `{{repository}}` is replaced with the new Repository name.

Example override:

```json
{
  "REPOSITORY_TEMPLATE": {
    "description": "# {{repository}}\n\n## Purpose\n\nWhat is this repository responsible for?\n\n## Scope\n\n- In scope:\n- Out of scope:\n\n## Getting started\n\n1. Add setup steps.\n\n## Architecture and decisions\n\nRecord important decisions and related IDIOS Issues.\n"
  }
}
```

This overview is stored as the Repository's description and appears in that Repository's Overview page. Editing the template later affects future Repositories only; it does not rewrite existing descriptions.

## Issue templates

`ISSUE_TEMPLATE` contains a default key and a map of templates. The bundled keys are:

| Key | Use it for | What it prompts you to record |
| --- | --- | --- |
| `task` | Planned work | Goal, context, plan, acceptance criteria, dependencies and notes |
| `bug` | Unexpected behavior | Summary, expected/actual behavior, reproduction steps, environment and regression checks |
| `feature` | A proposed capability | User problem, proposed behavior, impact, acceptance criteria, alternatives and trade-offs |
| `research` | Investigation | Question/hypothesis, why it matters, investigation plan, findings and follow-up |

### Choosing a template in the TUI

1. Open **New → Issue** or choose **+ New Issue** from a Repository page.
2. The first field is **Issue template**. It starts with the configured default, normally `task`.
3. To switch template, focus that field, press `Ctrl+U` to clear it, type a key such as `bug`, and press `Enter`. IDIOS loads that template's body and title prefix.
4. Use `Tab` / `Shift+Tab` to change fields. The Description / issue brief is a multiline editor: `Enter` inserts a line break, arrow keys move through lines, and `Ctrl+Enter` creates the Issue.
5. Give the Issue a specific title. If a template has a `title_prefix`, IDIOS keeps or adds that prefix without duplicating it.

Every Issue still belongs to a Repository. A parent Issue creates a Sub-issue in the same Repository; work that must wait for an Issue in another Repository should use the blocking `depends_on` relationship instead.

### Customize an Issue template

For example, add a special `design` template and make it the default:

```json
{
  "ISSUE_TEMPLATE": {
    "default": "design",
    "templates": {
      "design": {
        "name": "Design proposal",
        "title_prefix": "[Design]: ",
        "body": "## Goal\n\nWhat outcome are we designing for?\n\n## Constraints\n\nList technical, product, and compatibility constraints.\n\n## Proposed design\n\nDescribe the approach and important components.\n\n## Alternatives considered\n\nCompare at least one alternative and explain the trade-off.\n\n## Acceptance criteria\n\n- [ ] Design questions are resolved.\n- [ ] Risks and migration concerns are recorded.\n- [ ] Follow-up implementation Issues are identified.\n"
      }
    }
  }
}
```

A template definition supports:

- `name`: human-readable name shown after applying the template.
- `title_prefix`: optional prefix inserted at the start of the Issue title.
- `body`: multiline starter content. Use `\n` in JSON strings to encode new lines.

Template keys should be short lowercase identifiers such as `design`, `bug`, or `research`. Existing bundled templates remain available when you add a custom key; config is merged recursively. To change only the bug template, put just `"bug"` under `ISSUE_TEMPLATE.templates` and override its fields.

## Pull Request template

When an Issue is in progress and is not blocked, open its detail page and choose **Create pull request**. The form preloads the configured body and, when the Issue is already selected, suggests a title based on the Issue title. The starter template asks for:

- A concise summary and the reason for the change.
- The related Issue.
- Implementation details and design trade-offs.
- Tests and verification results.
- Risks, follow-up work and reviewer guidance.

Use `Tab` to move between form fields, `Enter` to add lines inside the multiline description, and `Ctrl+Enter` to create the PR record. Before submitting, replace every checklist item or placeholder that matters with actual information; do not claim that a test passed unless it ran.

### Customize the Pull Request template

```json
{
  "PULL_REQUEST_TEMPLATE": {
    "title_prefix": "Review: ",
    "body": "## Summary\n\nWhat changed and why?\n\n## Related Issue\n\nLink the Issue reference, such as `IDIOS#12`.\n\n## Verification\n\n- [ ] Unit tests run\n- [ ] Manual workflow checked\n- [ ] Documentation updated\n\nCommands and results: list the commands you ran and their actual results.\n\n## Risk and rollback\n\nDescribe potential regressions, migration concerns and the recovery plan.\n\n## Reviewer guide\n\nPoint reviewers to the critical code paths and decisions.\n"
  }
}
```

`title_prefix` is optional. `body` is copied into new PR descriptions. Like Issue templates, template changes are not retroactive: previously created Pull Requests keep the explanation they already stored.

## How templates relate to the work graph

Templates shape the text entered into a record; they do not automatically create graph edges or change workflow state. After creating the Issue, use IDIOS actions to:

- Add Sub-issues for smaller pieces of the same work.
- Add a blocking prerequisite if the Issue cannot start until another is complete.
- Assign labels and milestones.
- Link Concepts for reusable knowledge.
- Add the Issue to a Project that tracks a wider goal.

An Issue cannot be started while required blockers are unfinished. The usual workflow is `open → in_progress → Pull Request → accepted → done`. Accepting a Pull Request records a logical completion in IDIOS; it does not perform a Git commit or merge source files.

## Scope and CLI behavior

These configured templates currently apply to the interactive TUI forms. CLI commands are intentionally explicit so scripts remain deterministic; provide `--title` and `--description` when creating an Issue, or `--issue` and `--description` when creating a Pull Request. The TUI templates are text starters, not a remote service, GitHub Markdown renderer, review policy, or validation engine. Core workflow rules are validated separately by the Application Engine.
