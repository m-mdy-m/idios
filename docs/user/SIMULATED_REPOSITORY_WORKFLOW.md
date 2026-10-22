# Simulated Repository Workflow

IDIOS repositories are local domain records, not checked-out Git repositories. The workflow borrows useful ideas from code-hosting platforms while keeping every operation inside the IDIOS Engine and its SQLite database.

## Issues and PRs

Every issue has one of four types: `task`, `bug`, `feature`, or `research`. Start all issues included in a pull request before opening or merging it. A PR may include a primary issue and additional comma-separated issue references, but every linked issue must belong to the same repository, be `in_progress`, and have no blocking prerequisite.

Accepting a PR is IDIOS's simulated merge operation. In one SQLite transaction it marks the PR as accepted, creates one immutable completion record with a short SHA-256-derived simulated commit ID, snapshots the virtual file tree, and moves every linked issue to `done`. The completion sequence follows the target workflow branch. This does not invoke Git, merge commits, or modify files on the host filesystem.

## Repository history

The History view is a tree: each simulated commit opens its PR, work summary, and all included issues. Expand an issue to inspect its saved description, type, priority, creation timestamp, and completion timestamp. The repository overview summarizes commit count and completed issue counts by type.

## Virtual file tree

Each repository starts with a virtual `README.md`. Add and edit text files from the Files tab. Paths are relative to the simulated repository root; parent traversal, absolute paths, reserved `.idios/` metadata paths, and files larger than 2 MiB are rejected. The file tree is stored in SQLite and does not represent files on disk.

## Tags and snapshots

Creating a tag records the latest completion sequence on the default workflow branch. Once at least one new-format simulated commit exists, the tag snapshots that commit’s immutable file tree, not later unmerged edits. If the repository has no simulated commits yet, the first tag snapshots the current virtual file tree. Legacy completions created before file snapshots existed also fall back to the current tree because their historical contents cannot be reconstructed. Tags are immutable and unique within a repository. The first tag includes history from the beginning; each later tag records completions after the previous tag and up to the new tag's sequence. Compare shows added, modified, and deleted files between snapshots. Downloaded ZIPs include the snapshot, `.idios/tag.json`, and `.idios/history.json`; the manifest identifies the previous tag and its download URL. Older tags remain separately downloadable from the Tags view.

A downloaded archive is an IDIOS snapshot, not a Git bundle. It does not contain real Git objects or commit-compatible metadata.
