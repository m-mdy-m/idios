#!/usr/bin/env python3
"""Check IDIOS documentation entry points and local Markdown links."""
from __future__ import annotations

import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
REQUIRED = (
    "README.md",
    "CHANGELOG.md",
    "docs/README.md",
    "docs/arch/README.md",
    "docs/develop/README.md",
    "docs/product/README.md",
    "docs/user/README.md",
    "docs/product/overview.md",
    "docs/product/features.md",
    "docs/user/getting-started.md",
    "docs/user/workspace.md",
    "docs/user/shelf.md",
    "docs/user/ai.md",
    "docs/develop/pipeline.md",
    "docs/arch/overview.md",
    "docs/reference/cli.md",
    "pipeline.toml",
    "scripts/run-pipeline.py",
)
FORBIDDEN = (
    "PROVIDER_REFRESH_REPORT.md",
    "REDESIGN_REPORT.md",
)
FORBIDDEN_DIRECTORIES = (
    "docs/architecture",
    "docs/development",
    "docs/guides",
)
LINK = re.compile(r"(?<!!)\[[^\]]*\]\(([^)]+)\)")
IMAGE = re.compile(r"!\[[^\]]*\]\(([^)]+)\)")
HTML_IMAGE = re.compile(r"<img\b[^>]*?\bsrc=[\"']([^\"']+)[\"']", re.IGNORECASE)


def local_target(source: Path, raw: str) -> Path | None:
    target = raw.strip().split()[0].strip("<>")
    if not target or target.startswith(("https://", "http://", "mailto:", "#", "data:")):
        return None
    target_path = target.split("#", 1)[0].split("?", 1)[0]
    if not target_path:
        return None
    return (source.parent / target_path).resolve()


def main() -> int:
    errors: list[str] = []

    for item in REQUIRED:
        if not (ROOT / item).is_file():
            errors.append(f"missing required file: {item}")

    for item in FORBIDDEN:
        if (ROOT / item).exists():
            errors.append(f"temporary report must not be present: {item}")
    for item in FORBIDDEN_DIRECTORIES:
        if (ROOT / item).exists():
            errors.append(f"legacy documentation directory must not be present: {item}")

    for path in ROOT.rglob("*.md"):
        if any(part in {".git", ".tools", "node_modules", "dist"} for part in path.parts):
            continue
        source = path.read_text(encoding="utf-8", errors="replace")
        targets = LINK.findall(source) + IMAGE.findall(source) + HTML_IMAGE.findall(source)
        for raw in targets:
            resolved = local_target(path, raw)
            if resolved is None:
                continue
            try:
                resolved.relative_to(ROOT.resolve())
            except ValueError:
                errors.append(f"link escapes repository: {path.relative_to(ROOT)} -> {raw}")
                continue
            if not resolved.exists():
                errors.append(f"broken local reference: {path.relative_to(ROOT)} -> {raw}")

    if errors:
        for error in errors:
            print(f"FAIL {error}", file=sys.stderr)
        print(f"Documentation check failed: {len(errors)} problem(s).", file=sys.stderr)
        return 1

    print("OK documentation: entry points exist, local links resolve, temporary reports are absent.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
