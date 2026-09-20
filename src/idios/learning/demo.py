"""Runs the exact scenario from the Learning Core refactor spec's
"Required Demonstration" (section 28) end to end, with zero AI calls,
to prove the Learning Core works completely standalone.

Run directly:  python -m idios.learning.demo
Or via the CLI:  idios learn-demo
"""
from __future__ import annotations

from pathlib import Path

from idios.learning.engine import LearningEngine
from idios.learning.models import CuriosityDisposition, EvidenceKind, SessionStage
from idios.storage.base import JsonFileStorageProvider


def run_demo(storage_path: Path) -> None:
    storage = JsonFileStorageProvider(storage_path)
    engine = LearningEngine(storage)

    goal = engine.create_goal(
        title="Learn GPIO for Talken", project="Talken", scope=["GPIO", "microcontroller", "resistor"]
    )
    print(f"Goal: {goal.title}")

    engine.set_definition_of_done(
        goal,
        criteria=[
            "understand GPIO output",
            "build LED circuit",
            "write firmware",
            "validate behavior",
            "document experiment",
        ],
    )

    question = engine.create_question(goal, "How does GPIO output work?")
    print(f"Question: {question.text}")

    task = engine.create_task(goal, "Control an LED", question=question)
    print(f"Task: {task.title}")

    session = engine.start_session(goal, task)
    print(f"Session: {session.stage.value}")

    session = engine.advance(session, SessionStage.EXPERIMENT)
    experiment = engine.record_experiment(
        task,
        description="Connect LED to GPIO pin through a resistor, toggle output",
        outcome="LED responded correctly to GPIO state",
    )
    print(f"Experiment: {experiment.description} -> {experiment.outcome}")

    e1 = engine.record_evidence(EvidenceKind.EXPERIMENT, "LED experiment completed", concept="GPIO output", task=task)
    e2 = engine.record_evidence(
        EvidenceKind.IMPLEMENTATION_COMPLETED, "firmware written and flashed", concept="GPIO output", task=task
    )
    print(f"Evidence: {e1.kind.value} + {e2.kind.value}")

    session = engine.advance(session, SessionStage.VALIDATE)
    concept = engine.validate_concept("GPIO output", open_gaps=["current limiting resistor sizing still unclear"])
    print(f"Concept '{concept.name}': {concept.status.value}; open gaps: {concept.open_gaps}")

    session = engine.advance(session, SessionStage.REFLECT)
    reflection = engine.reflect(
        session, understood=["GPIO output understood"], unclear=["current limiting resistor sizing"]
    )
    print(f"Reflection - understood: {reflection.understood}; unclear: {reflection.unclear}")

    curiosity = engine.capture_curiosity(
        "Why does silicon behave as a semiconductor?",
        disposition=CuriosityDisposition.USEFUL_LATER,
        origin_task=task,
    )
    print(f"Curiosity: \"{curiosity.question}\" -> {'parked' if curiosity.parked else 'kept active'}")

    session = engine.advance(session, SessionStage.NEXT_QUESTION)
    next_question = engine.create_next_question(goal, "Why is a resistor needed for the LED?")
    print(f"Next question: {next_question.text}")

    print()
    parked = engine.parking_lot()
    print(f"Parking lot ({len(parked)} item(s)):")
    for item in parked:
        print(f"  - {item.question}")


if __name__ == "__main__":
    import tempfile

    with tempfile.TemporaryDirectory() as tmp:
        run_demo(Path(tmp) / "learning-demo.json")
