from idios.decision.local import LocalDecisionEngine
from idios.domain.state import DecisionAction, DecisionState, TaskState


def test_new_task_routes_to_retrieve():
    engine = LocalDecisionEngine()
    result = engine.decide(DecisionState(goal="g", current_task_state=TaskState.NEW))
    assert result.action == DecisionAction.RETRIEVE


def test_weak_concepts_route_to_practice():
    engine = LocalDecisionEngine()
    state = DecisionState(
        goal="g",
        current_task_state=TaskState.DECIDE,
        weak_concepts=["endianness"],
    )
    result = engine.decide(state)
    assert result.action == DecisionAction.PRACTICE
    assert result.fallback_action == DecisionAction.TEACH


def test_no_signal_asks_user():
    engine = LocalDecisionEngine()
    state = DecisionState(goal="g", current_task_state=TaskState.DECIDE)
    result = engine.decide(state)
    assert result.action == DecisionAction.QUESTION_USER
