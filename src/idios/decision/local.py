"""LocalDecisionEngine — deterministic, model-free decision path
(section 7.4 of the master design).

Deliberately not called "JEV" — see AGENTS.md. This is a small,
transparent rule-based baseline so the orchestrator has a working
decision path before any LLM, or the real JEV service, is wired up.
Until JEV exists as a real service, treat this as the primary decision
engine, not a fallback (see docs/decisions.md).
"""
from __future__ import annotations

from idios.domain.state import DecisionAction, DecisionResult, DecisionState, TaskState


class LocalDecisionEngine:
    """A transparent, inspectable stand-in for a learned decision policy."""

    def decide(self, state: DecisionState) -> DecisionResult:
        if state.current_task_state == TaskState.NEW:
            return DecisionResult(
                action=DecisionAction.RETRIEVE,
                confidence=0.9,
                reason_codes=["new_task_needs_context"],
            )

        if state.weak_concepts:
            return DecisionResult(
                action=DecisionAction.PRACTICE,
                confidence=0.7,
                reason_codes=["weak_concepts_present"],
                constraints={"concepts": state.weak_concepts},
                fallback_action=DecisionAction.TEACH,
            )

        if state.current_project:
            return DecisionResult(
                action=DecisionAction.PROJECT_TASK,
                confidence=0.6,
                reason_codes=["project_context_available_no_weak_concepts"],
            )

        return DecisionResult(
            action=DecisionAction.QUESTION_USER,
            confidence=0.4,
            reason_codes=["insufficient_signal"],
            fallback_action=DecisionAction.ANSWER,
        )
