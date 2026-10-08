"""Hard rules from AGENTS.md, enforced mechanically."""
import re
from pathlib import Path

SRC = Path(__file__).resolve().parents[1] / "src" / "idios"
BANNED = ("openai", "anthropic", "google.generativeai", "genai", "qwen", "requests",
          "httpx", "urllib.request", "http.client", "socket", "aiohttp", "numpy", "torch")


def imports(path: Path) -> set[str]:
    found = set()
    for line in path.read_text(encoding="utf-8").splitlines():
        m = re.match(r"\s*(?:from|import)\s+([\w.]+)", line)
        if m:
            found.add(m.group(1))
    return found


def test_no_ai_or_network_imports_anywhere():
    for path in SRC.rglob("*.py"):
        for name in imports(path):
            assert not any(name == b or name.startswith(b + ".") for b in BANNED), (path, name)


def test_domain_and_services_never_touch_the_terminal():
    for folder in ("domain", "services", "storage"):
        for path in (SRC / folder).rglob("*.py"):
            text = path.read_text(encoding="utf-8")
            assert "print(" not in text and "input(" not in text, path
            assert "idios.shell" not in text and "idios.cli" not in text, path


def test_only_repositories_write_sql():
    for path in SRC.rglob("*.py"):
        if path.parent.name == "storage" or path.name == "export.py":
            continue
        text = path.read_text(encoding="utf-8")
        assert "conn.execute(" not in text and "SELECT " not in text, path


def test_runtime_has_no_third_party_dependencies():
    import pytest
    tomllib = pytest.importorskip("tomllib")  # Python 3.11+
    data = tomllib.loads((SRC.parents[1] / "pyproject.toml").read_text())
    assert data["project"]["dependencies"] == []
