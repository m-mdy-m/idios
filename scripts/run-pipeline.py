#!/usr/bin/env python3
"""Download and run the current main branch of the shared pipeline project."""
from __future__ import annotations

import shutil
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
PIPELINE_URL = "https://github.com/m-mdy-m/pipeline.git"
PIPELINE_REF = "main"
PIPELINE_DIR = ROOT / ".tools" / "pipeline"


def run(command: list[str], *, cwd: Path = ROOT) -> None:
    subprocess.run(command, cwd=cwd, check=True)


def ensure_pipeline() -> Path:
    if sys.version_info < (3, 12):
        raise SystemExit("IDIOS pipeline requires Python 3.12 or newer.")

    git = shutil.which("git")
    if not git:
        raise SystemExit("Git is required to download the shared pipeline runner.")

    if PIPELINE_DIR.exists():
        try:
            remote = subprocess.run(
                [git, "-C", str(PIPELINE_DIR), "remote", "get-url", "origin"],
                check=True,
                capture_output=True,
                text=True,
            ).stdout.strip()
            current = subprocess.run(
                [git, "-C", str(PIPELINE_DIR), "rev-parse", "HEAD"],
                check=True,
                capture_output=True,
                text=True,
            ).stdout.strip()
        except (subprocess.CalledProcessError, OSError):
            shutil.rmtree(PIPELINE_DIR)
        else:
            if remote.rstrip("/") not in {
                PIPELINE_URL.rstrip("/"),
                "https://github.com/m-mdy-m/pipeline",
            }:
                shutil.rmtree(PIPELINE_DIR)
            else:
                try:
                    run([git, "-C", str(PIPELINE_DIR), "fetch", "--depth", "1", "origin", PIPELINE_REF])
                except subprocess.CalledProcessError:
                    print(
                        f"Warning: could not update pipeline/{PIPELINE_REF}; using cached commit {current}.",
                        file=sys.stderr,
                    )
                    return PIPELINE_DIR
                run([git, "-C", str(PIPELINE_DIR), "checkout", "--force", "--detach", "FETCH_HEAD"])
                return PIPELINE_DIR

    PIPELINE_DIR.parent.mkdir(parents=True, exist_ok=True)
    run([
        git,
        "clone",
        "--depth",
        "1",
        "--single-branch",
        "--branch",
        PIPELINE_REF,
        PIPELINE_URL,
        str(PIPELINE_DIR),
    ])
    return PIPELINE_DIR


def main(argv: list[str]) -> int:
    args = argv or ["list"]
    if args[0] not in {"validate", "list", "run"}:
        print(
            "Usage: python scripts/run-pipeline.py [validate|list|run TASK] [PIPELINE OPTIONS]",
            file=sys.stderr,
        )
        return 2

    try:
        pipeline = ensure_pipeline()
        command = [
            sys.executable,
            str(pipeline / "scripts" / "pipeline.py"),
            *args,
            "--root",
            str(ROOT),
            "--config",
            str(ROOT / "pipeline.toml"),
        ]
        return subprocess.run(command, cwd=ROOT, check=False).returncode
    except subprocess.CalledProcessError as exc:
        return exc.returncode or 1
    except OSError as exc:
        print(f"Could not run shared pipeline: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
