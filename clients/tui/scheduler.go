package tui

import (
	"context"
	"time"

	"idios/internal/application"

	tea "charm.land/bubbletea/v2"
)

type schedulerTickMsg struct{ at time.Time }
type schedulerProcessedMsg struct {
	count int
	err   error
}

func schedulerTickCmd(interval time.Duration) tea.Cmd {
	if interval < time.Second {
		interval = time.Second
	}
	return tea.Tick(interval, func(t time.Time) tea.Msg { return schedulerTickMsg{at: t} })
}

func processDueJobsCmd(ctx context.Context, app *application.Service, at time.Time, notificationsEnabled bool) tea.Cmd {
	return func() tea.Msg {
		count, err := app.ProcessDueJobs(ctx, at, notificationsEnabled)
		return schedulerProcessedMsg{count: count, err: err}
	}
}
