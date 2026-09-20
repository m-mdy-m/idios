"""Typed errors for IDIOS (section 25 of the master design).

Every failure mode here must surface explicitly. Nothing in this
codebase should catch one of these and quietly return a fabricated
answer instead.
"""


class IdiosError(Exception):
    """Base class for all IDIOS-specific errors."""


class ModelUnavailableError(IdiosError):
    pass


class GPUUnavailableError(IdiosError):
    pass


class RetrievalEmptyError(IdiosError):
    pass


class ToolFailureError(IdiosError):
    def __init__(self, tool_name: str, detail: str):
        super().__init__(f"tool '{tool_name}' failed: {detail}")
        self.tool_name = tool_name
        self.detail = detail


class MalformedOutputError(IdiosError):
    pass


class LowConfidenceError(IdiosError):
    def __init__(self, confidence: float, threshold: float):
        super().__init__(f"confidence {confidence:.2f} below threshold {threshold:.2f}")
        self.confidence = confidence
        self.threshold = threshold


class VerificationFailureError(IdiosError):
    pass


class StaleProjectStateError(IdiosError):
    pass


class JEVNotConfiguredError(IdiosError):
    """Raised when the JEV backend is selected but no endpoint/credentials
    exist yet. IDIOS must never silently pretend a remote JEV call
    succeeded."""
