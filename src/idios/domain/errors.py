"""Errors that are meant to be shown to the user.

Every IdiosError carries a human-readable message and an optional hint.
The shell prints them as-is; internals never leak into normal use.
"""
from __future__ import annotations


class IdiosError(Exception):
    """A problem the user can understand and fix."""

    def __init__(self, message: str, hint: str = "") -> None:
        super().__init__(message)
        self.message = message
        self.hint = hint

    def render(self) -> str:
        if self.hint:
            return f"{self.message}\n\nTry:\n  {self.hint}"
        return self.message


class NotFound(IdiosError):
    pass


class Invalid(IdiosError):
    pass
