package tui

import (
	"context"

	"idios/internal/application"
	"idios/internal/protocol"
)

// executeSharedAction routes a client action through the same versioned Go protocol consumed by
// Web. Rich TUI forms may still own presentation, but the operation and its validation are shared.
func executeSharedAction(ctx context.Context, app *application.Service, commandID string, input map[string]any) error {
	_, err := protocol.Execute(ctx, app, commandID, input)
	return err
}
