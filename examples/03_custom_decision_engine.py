"""Example 03 — Implementing and wiring a custom decision engine.

Shows how to replace LocalDecisionEngine with your own policy while
keeping everything else unchanged. The custom engine here is trivial
(always says REVIEW) but illustrates the protocol contract.

Run from the repo root (with venv activated):
    python examples/03_custom_decision_engine.py
"""
from __future__ import annotations

from idios.domain.state import (
    DecisionAction,
    DecisionResult,
    DecisionState,
    TaskState,
)


# ── implement the protocol ────────────────────────────────────────────────────
# There is no formal ABC to inherit from — just match the signature.
# See idios/decision/base.py for the Protocol definition.

class PriorityReviewEngine:
    """A toy engine that always pushes toward review/reflect cycles.

    Replace this with your own logic: query a local model, run a
    scoring function, or call an external service.
    """

    # Map every TaskState to the action you want to push.
    _STATE_MAP: dict[TaskState, DecisionAction] = {
        TaskState.NEW:           DecisionAction.RETRIEVE,
        TaskState.CLASSIFY:      DecisionAction.RETRIEVE,
        TaskState.RETRIEVE:      DecisionAction.REVIEW,
        TaskState.DECIDE:        DecisionAction.REVIEW,
        TaskState.PREPARE:       DecisionAction.PROJECT_TASK,
        TaskState.ACT:           DecisionAction.VERIFY,
        TaskState.VERIFY:        DecisionAction.REFLECT,
        TaskState.REFLECT:       DecisionAction.REVIEW,
        TaskState.COMMIT_MEMORY: DecisionAction.ANSWER,
        TaskState.COMPLETE:      DecisionAction.ANSWER,
    }

    def decide(self, state: DecisionState) -> DecisionResult:
        action = self._STATE_MAP.get(state.current_task_state, DecisionAction.QUESTION_USER)
        confidence = 0.85 if state.current_task_state in self._STATE_MAP else 0.3

        return DecisionResult(
            action=action,
            confidence=confidence,
            reason_codes=["priority_review_policy", state.current_task_state.value],
            fallback_action=DecisionAction.QUESTION_USER,
        )


# ── exercise it ───────────────────────────────────────────────────────────────

engine = PriorityReviewEngine()

scenarios = [
    DecisionState(goal="learn attention mechanism", current_task_state=TaskState.NEW),
    DecisionState(goal="learn attention mechanism", current_task_state=TaskState.RETRIEVE),
    DecisionState(goal="implement encoder", current_task_state=TaskState.ACT,
                  current_project="mini-transformer"),
    DecisionState(goal="implement encoder", current_task_state=TaskState.REFLECT,
                  weak_concepts=["multi-head attention"]),
]

for state in scenarios:
    result = engine.decide(state)
    print(f"[{state.current_task_state.value:<14}] "
          f"action={result.action.value:<15} "
          f"confidence={result.confidence:.2f}  "
          f"reason={result.reason_codes}")
