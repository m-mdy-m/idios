# Repository inspection contracts

This document is the P1 implementation contract for remote repository inspection. It deliberately separates provider facts from IDIOS presentation and keeps optional provider capabilities explicit.

## Shared provider capability model

Each adapter should describe support for the operations it can actually perform. UI visibility and actions must be derived from those capabilities rather than from provider name alone.

Capabilities should distinguish at least:
- repository metadata and README access;
- branch list and branch creation;
- recursive tree and file reads;
- commits and commit verification metadata;
- issues and change requests;
- releases, tags and downloadable assets;
- public repository search and forking.

An unsupported capability is not the same as an empty result. The UI must show a specific unavailable state and, where useful, a link or clone alternative.

## Code Browser and Remote Tree

Normalize provider tree entries into a hierarchical tree model with:
- stable path identity;
- directory/file kind;
- optional blob SHA, size and provider URL;
- children for directories.

Sort directories before files, then sort alphabetically within each group. Expand/collapse state is local UI state and must not be lost when a sibling is opened. Support path search and breadcrumbs. Use incremental directory loading when an adapter can fetch a directory independently; for recursive-only APIs, build the hierarchy once and avoid rendering a flat path list.

The editor/preview pane should expose path, branch/ref, size, revision and copy actions. Writes must keep existing optimistic concurrency/revision checks.

## README discovery and Markdown safety

Use a deterministic candidate order, beginning with `README.md`, `README`, `README.txt`, then provider-reported candidates. Do not scan every file body to guess a README.

Markdown rendering must:
- support headings, paragraphs, emphasis, links, lists, tables, blockquotes and fenced code;
- allow only safe `https:`, `http:` and appropriate relative links;
- avoid executing raw HTML or scripts;
- restrict embedded images to safe URL schemes and provide a fallback;
- preserve a plain-text path for unsupported or oversized files.

The same renderer policy should be reused by local and remote repository previews where possible.

## Commit provenance and verification

The provider-neutral commit model must distinguish:
- author identity/email/date;
- committer identity/email/date;
- commit SHA and message;
- signature state and type;
- signature verification reason/details;
- CI/status checks.

Signature state is one of `verified`, `unverified`, `unsigned`, or `unknown`. The UI must not infer a verified signature from a matching email or a provider avatar. CI status is a separate signal and must not be presented as cryptographic verification.

## Releases and Tags

Model Release and Git Tag separately.

A Release may include:
- tag name, release title and notes;
- draft/prerelease state;
- author and published/created dates;
- assets with name, platform hints, content type, byte size, download count and digest when supplied.

A Tag may include:
- tag name and target commit;
- lightweight/annotated type;
- annotation/message and tagger metadata;
- associated release reference, if the provider supplies one.

Load the newest release and newest tag first. Older records are paginated on demand. Comparison should use provider compare APIs or actual Git objects and report unavailable when neither exists.

## Explore and Fork

Explore search is separate from account-linked repository discovery. Public search should not require an OAuth account when provider APIs support anonymous requests. Search fields and pagination are provider-specific.

Search results remain unlinked until the user chooses Link or Fork. Fork is a provider mutation that requires connectivity and permission; it must not overwrite a local repository or imply an offline fork was created. Preview README/tree before Link when the provider supports it.

## Related issues

- #69 — hierarchical tree and README preview.
- #70 — commit provenance, signature verification and checks.
- #71 — releases, tags and assets.
- #72 — cross-provider Explore and Fork.
