package domain

import "errors"

var (
	ErrNotFound          = errors.New("not found")
	ErrConflict          = errors.New("conflict")
	ErrInvalid           = errors.New("invalid input")
	ErrBlocked           = errors.New("issue is blocked by incomplete prerequisites")
	ErrInvalidTransition = errors.New("invalid workflow transition")
	ErrUnsupported       = errors.New("not supported in this version")
	ErrUnavailable       = errors.New("temporarily unavailable")
	ErrPermission        = errors.New("permission denied")
	ErrRateLimited       = errors.New("rate limited")
)
