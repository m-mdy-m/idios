package cli

import (
	"encoding/json"
	"errors"
	"io"

	"idios/internal/domain"
	"idios/internal/logger"
)

// report writes a failure to stderr: a stable JSON envelope in JSON mode, one readable
// line plus a usage hint otherwise.
func report(w io.Writer, asJSON bool, args []string, err error, log *logger.Logger) {
	log.Errorf(err, "command failed", "command", args)
	if asJSON {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"protocol_version": 1,
			"error":            map[string]string{"code": errorCode(err), "message": err.Error()},
		})
		return
	}
	printUsage(w, args, err)
}

// errorCode maps domain errors to the stable codes machine clients rely on.
func errorCode(err error) string {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return "not_found"
	case errors.Is(err, domain.ErrConflict):
		return "conflict"
	case errors.Is(err, domain.ErrInvalid):
		return "invalid_input"
	case errors.Is(err, domain.ErrBlocked):
		return "blocked"
	case errors.Is(err, domain.ErrInvalidTransition):
		return "invalid_transition"
	case errors.Is(err, domain.ErrUnsupported):
		return "unsupported"
	default:
		return "internal_error"
	}
}
