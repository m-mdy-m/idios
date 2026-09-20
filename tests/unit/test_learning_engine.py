from pathlib import Path

import ast

import pytest

from idios.learning.engine import LearningEngine
from idios.learning.models import ConceptStatus, CuriosityDisposition, EvidenceKind, GoalStatus, SessionStage
from idios.storage.base import JsonFileStorageProvider


def _engine(tmp_path: Path) -> LearningEngine:
    return LearningEngine(JsonFileStorageProvider(tmp_path / "state.json"))


def test_full_slice_works_with_zero_ai(tmp_path: Path):
    """The exact 'Required Demonstration' scenario (refactor spec,
    section 28), run with no AI, no JEV, no model, no network — only
    the Learning Core plus JSON storage."""
    engine = _engine(tmp_path)

    goal = engine.create_goal("Learn GPIO for Talken", project="Talken")
    assert goal.status == GoalStatus.ACTIVE

    engine.set_definition_of_done(goal, ["understand GPIO output", "build LED circuit"])
    question = engine.create_question(goal, "How does GPIO output work?")
    task = engine.create_task(goal, "Control an LED", question=question)
    session = engine.start_session(goal, task)

    session = engine.advance(session, SessionStage.EXPERIMENT)
    engine.record_experiment(task, "Connect LED to GPIO pin", outcome="LED responded")
    engine.record_evidence(EvidenceKind.EXPERIMENT, "LED experiment completed", concept="GPIO output", task=task)

    session = engine.advance(session, SessionStage.VALIDATE)
    concept = engine.validate_concept("GPIO output", open_gaps=["resistor sizing unclear"])
    assert concept.status == ConceptStatus.VALIDATED

    reflection = engine.reflect(session, understood=["GPIO output"], unclear=["resistor sizing"])
    assert reflection.unclear == ["resistor sizing"]

    curiosity = engine.capture_curiosity(
        "Why does silicon behave as a semiconductor?", CuriosityDisposition.USEFUL_LATER, origin_task=task
    )
    assert curiosity.parked is True
    assert any(c.id == curiosity.id for c in engine.parking_lot())

    next_question = engine.create_next_question(goal, "Why is a resistor needed for the LED?")
    assert next_question.goal_id == goal.id


def test_cannot_validate_without_evidence(tmp_path: Path):
    engine = _engine(tmp_path)
    with pytest.raises(ValueError):
        engine.validate_concept("never touched")


def test_session_cannot_move_backward(tmp_path: Path):
    engine = _engine(tmp_path)
    goal = engine.create_goal("g")
    session = engine.start_session(goal)
    session = engine.advance(session, SessionStage.VALIDATE)
    with pytest.raises(ValueError):
        engine.advance(session, SessionStage.LEARN)


def test_curiosity_relevant_now_is_not_parked(tmp_path: Path):
    engine = _engine(tmp_path)
    curiosity = engine.capture_curiosity("quick related question", CuriosityDisposition.RELEVANT_NOW)
    assert curiosity.parked is False


def test_definition_of_done_tracks_remaining_criteria():
    from idios.learning.models import DefinitionOfDone

    dod = DefinitionOfDone(criteria=["a", "b", "c"])
    dod.mark_met("a")
    assert dod.remaining() == ["b", "c"]
    assert not dod.is_met()
    dod.mark_met("b")
    dod.mark_met("c")
    assert dod.is_met()


def _imported_module_names(path: Path) -> set[str]:
    tree = ast.parse(path.read_text(encoding="utf-8"))
    names: set[str] = set()
    for node in ast.walk(tree):
        if isinstance(node, ast.Import):
            names.update(alias.name for alias in node.names)
        elif isinstance(node, ast.ImportFrom) and node.module:
            names.add(node.module)
    return names


def test_learning_core_has_no_ai_imports():
    """Acceptance criterion from the refactor spec (section 29): the
    Learning Core must not import idios.decision, idios.models, or any
    known AI/model library. This parses actual import statements via
    `ast`, so a docstring or comment mentioning the forbidden names
    (like this file's own) can't produce a false positive."""
    import idios.learning.engine as engine_module
    import idios.learning.models as models_module

    forbidden_prefixes = ("idios.decision", "idios.models", "ollama", "openai", "anthropic")
    for module in (engine_module, models_module):
        for imported in _imported_module_names(Path(module.__file__)):
            assert not imported.startswith(forbidden_prefixes), (
                f"{module.__name__} imports '{imported}', which reaches into a forbidden "
                "AI/cognitive-layer module"
            )
