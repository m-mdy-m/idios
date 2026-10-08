import json

import pytest

from idios.cli.main import main
from idios.services.app import App


@pytest.fixture(autouse=True)
def isolated_home(monkeypatch, home):
    monkeypatch.setenv("IDIOS_HOME", str(home))


def seed(home):
    a = App(home)
    a.context.set_source(a.sources.add("Programming from the Ground Up", author="Jonathan Bartlett"))
    a.knowledge.concept("CPU Register")
    a.learning.highlight("Registers are fast")
    q = a.learning.ask("What is a register?")
    a.learning.answer(q, "Fast storage")
    a.knowledge.link("CPU Register", "Memory", "contains")
    a.close()


def test_search_show_and_shelf_subcommands(home, capsys):
    seed(home)
    assert main(["search", "register"]) == 0
    assert "Concept — CPU Register" in capsys.readouterr().out
    assert main(["show", "CPU", "Register"]) == 0
    assert "Related Concepts" in capsys.readouterr().out
    assert main(["shelf"]) == 0
    assert "Programming from the Ground Up" in capsys.readouterr().out


def test_export_json_and_markdown(home, tmp_path, capsys):
    seed(home)
    out = tmp_path / "out.json"
    assert main(["export", "--format", "json", "-o", str(out)]) == 0
    data = json.loads(out.read_text())
    assert data["sources"][0]["title"] == "Programming from the Ground Up"
    assert len(data["relations"]) == 1
    md = tmp_path / "out.md"
    assert main(["export", "-o", str(md)]) == 0
    text = md.read_text()
    assert "### Programming from the Ground Up" in text and "[[CPU Register]]" in text
    assert "[x] What is a register?" in text


def test_default_export_goes_to_the_data_folder(home, capsys):
    seed(home)
    assert main(["export"]) == 0
    assert list((home / "exports").glob("idios-*.md"))
