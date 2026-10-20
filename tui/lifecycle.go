package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
)

func (m *model) openQuitConfirmation() {
	if m.page == pageQuit {
		return
	}
	m.quitPreviousPage = m.page
	m.quitPreviousRepo = m.repoContext
	m.page = pageQuit
	m.cursor, m.scroll = 0, 0
	m.buttonFocus, m.focusAction, m.focusIndex = false, "", -1
}

func (m *model) cancelQuitConfirmation() {
	m.page = m.quitPreviousPage
	if m.page == "" || m.page == pageQuit {
		m.page = pageOverview
	}
	m.repoContext = m.quitPreviousRepo
	m.cursor, m.scroll = 0, 0
}

func (m *model) startExitAnimation() tea.Cmd {
	// Never tear down the process while a batch write is in flight. Finish the
	// current item, mark the rest as not run, then show the goodbye animation.
	if m.batchRun != nil {
		m.exitAfterBatch = true
		m.requestBatchCancel()
		m.page = pageBatchProgress
		m.status = "Exit requested; the current item will finish before IDIOS closes."
		m.statusError = false
		return nil
	}
	m.exitAnimating = true
	m.exitFrame = 0
	m.page = pageQuit
	return exitAnimationTickCmd()
}

func exitAnimationTickCmd() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(time.Time) tea.Msg { return exitAnimationTickMsg{} })
}

func (m *model) renderQuitConfirmation(c *canvas, p palette) {
	if m.exitAnimating {
		frames := []string{"◐ Closing IDIOS…", "◓ Ending the UI loop…", "◑ Restoring your terminal…"}
		frame := min(m.exitFrame, len(frames)-1)
		c.title("IDIOS · Goodbye")
		c.blank()
		c.panel("SHUTTING DOWN", frames[frame], "The terminal will return to its normal state automatically.")
		c.muted(fmt.Sprintf("Step %d of %d", frame+1, len(frames)))
		return
	}
	c.title("Leave IDIOS?")
	c.muted("Exit through the normal shutdown path; the terminal screen will be restored for you.")
	if m.batchRun != nil {
		c.panel("BATCH IN PROGRESS", "Exiting will finish the current item, skip remaining items, and keep completed records.", "You can retry failed or not-run items later from the batch results.")
	} else if m.form != "" {
		c.panel("UNSAVED FORM", "The current form is still in memory. Exiting will discard this draft.", "Choose Stay to return to it before leaving.")
	} else {
		c.panel("WORKSPACE", "Records already created are stored in the local database. Scheduled reminders are evaluated while IDIOS is running; they do not become an OS background service.")
	}
	c.buttonRow(buttonSpec{"Exit IDIOS", "quit:exit", true}, buttonSpec{"Stay here", "quit:cancel", false})
	c.blank()
	c.muted("Enter: exit · Esc: cancel · Ctrl+C: graceful emergency exit")
}
