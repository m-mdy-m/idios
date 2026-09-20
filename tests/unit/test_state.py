from idios.domain.state import DecisionAction, DecisionResult, DecisionState, TaskState


def test_decision_state_defaults():
    state = DecisionState(goal="learn transistors")
    assert state.current_task_state == TaskState.NEW
    assert state.weak_concepts == []
    assert state.task_id


def test_decision_result_confidence_bounds():
    result = DecisionResult(action=DecisionAction.ANSWER, confidence=0.5)
    assert 0.0 <= result.confidence <= 1.0
