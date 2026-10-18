# IDIOS V1 Workflow

## Issue lifecycle

```text
open ──start (prerequisites clear)──> in_progress
  ^                                      │
  │                                      ├─ create PR with mandatory description
  │                                      │
  │                                      v
cancelled <─ cancel                 open Pull Request
                                         │
                                         ├─ close ──> closed PR (Issue remains in_progress)
                                         │
                                         └─ accept (prerequisites + children complete)
                                                │
                                                v
                                    accepted PR + CompletionRecord on main
                                                │
                                                v
                                           Issue: done
```

An Issue cannot be manually moved to `done`. It is completed only by accepting its PR. V1 PR acceptance is a logical workflow operation; it does not merge source files or perform a Git operation.

## Blocking prerequisites across repositories

An explicit `depends_on` edge makes the source Issue a prerequisite of another Issue. For example:

```text
CPU#1 Learn electricity fundamentals
       ^
       │ depends_on prerequisite target
       │
ARLIZ#1 Write chapter three
```

The CLI syntax is `issue depends --issue ARLIZ#1 --on CPU#1`; to remove an obsolete dependency before starting the Issue, use `issue undepends --issue ARLIZ#1 --on CPU#1`. The ARLIZ Issue remains `open` and cannot start while CPU#1 is unfinished. After CPU#1's PR is accepted and its CompletionRecord is committed to CPU/main's logical history, the prerequisite is satisfied and ARLIZ#1 can start.

A semantic Concept edge is not a blocker. Linking ARLIZ#1 to the `Electricity fundamentals` Concept expresses knowledge relevance; it does not, by itself, prevent starting the Issue. If this knowledge has to be learned first, add the explicit Issue dependency as well.

Dependency and parent/child cycles are rejected. A parent Issue cannot start or have its PR accepted until its active Sub-issues are completed. Cancelling a Sub-issue removes that child from the parent's blocking set; by contrast, a cancelled explicit prerequisite does not count as completed, so remove that dependency with `issue undepends` if the prerequisite is no longer required. Dependencies and Sub-issue structure are changeable only while the dependent/parent Issue remains `open`. Sub-issues belong to the same Repository as their parent; dependencies can cross Repositories.

## PR and CompletionRecord

1. Start an eligible Issue.
2. Create a PR with a non-empty description. It records what was done, relevant reasoning, outcome and review notes.
3. Review and accept the PR.
4. In one transaction, IDIOS marks the PR accepted, appends a CompletionRecord to the PR's target Branch (`main` in V1), marks the Issue `done`, and adds activity events.

The CompletionRecord is the analogue for “accepted work was recorded” rather than a Git commit. It has a sequence and parent-completion link within a Branch. There are no Git hashes or file diffs in V1. Keep the PR description as the full explanation to avoid entering the same narrative twice; the completion summary should be concise.

An Issue can have at most one open PR and one accepted PR in V1. Closing a PR does not complete the Issue; a new PR can be opened for the Issue while it remains in progress.

## Branches

New Repositories start with `main`, which is their default Branch and the target for PRs. Branch is a first-class Domain entity and the schema reserves relationships for future branching. V1 does not provide create/switch/delete/merge operations for any non-main Branch.

## Projects, labels and milestones

- A Project can collect Issues from multiple Repositories.
- A Label can be global or local to a Repository. Repository views display both. When assignment uses a name that exists at both scopes, the local label takes precedence.
- A Milestone is scoped to a Repository or to a Project. To assign an Issue to a Project milestone, the Issue must first be in that Project.
- Typed knowledge relations help navigate Concepts and work, but only Issue dependency edges enforce readiness.
