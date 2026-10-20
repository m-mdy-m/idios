package tui

import (
	"context"
	"fmt"
	"idios/internal/application"
	"idios/internal/domain"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
)

const maxBatchItems = 100

func isBulkForm(kind formKind) bool {
	return kind == formBulkIssues || kind == formBulkQuotes || kind == formBulkJobs
}

// batchLines uses one visible line per item. Empty lines are ignored so pasted lists
// from Markdown or spreadsheets do not accidentally create empty records.
func batchLines(value string) []string {
	value = strings.ReplaceAll(strings.ReplaceAll(value, "\r\n", "\n"), "\r", "\n")
	lines := strings.Split(value, "\n")
	items := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			items = append(items, line)
		}
	}
	return items
}

func validateBulkForm(kind formKind, fields []string, repositories []domain.Repository) (int, error) {
	get := func(index int) string {
		if index < 0 || index >= len(fields) {
			return ""
		}
		return strings.TrimSpace(fields[index])
	}
	var items []string
	switch kind {
	case formBulkIssues:
		items = batchLines(get(1))
		if get(0) == "" {
			return 0, fmt.Errorf("choose a repository before creating Issues")
		}
		found := false
		for _, repo := range repositories {
			if strings.EqualFold(repo.Name, get(0)) {
				found = true
				break
			}
		}
		if !found {
			return 0, fmt.Errorf("repository %q does not exist; choose one from Repositories", get(0))
		}
		priority := strings.ToLower(get(3))
		if priority != "" && priority != "low" && priority != "normal" && priority != "high" {
			return 0, fmt.Errorf("priority must be low, normal or high")
		}
	case formBulkQuotes:
		items = batchLines(get(0))
	case formBulkJobs:
		items = batchLines(get(0))
		job, err := parseScheduledJobFields(get(1), get(2))
		if err != nil {
			return 0, err
		}
		if job.ScheduleType == domain.ScheduleOnce && (job.NextRunAt == nil || !job.NextRunAt.After(time.Now())) {
			return 0, fmt.Errorf("one-time job date and time must be in the future")
		}
		kind := strings.ToLower(get(3))
		allowed := map[string]bool{"daily-plan": true, "weekly-review": true, "blocked-issues": true, "open-issues": true, "custom-reminder": true}
		if !allowed[kind] {
			return 0, fmt.Errorf("unsupported reminder kind %q", kind)
		}
	default:
		return 0, fmt.Errorf("unsupported batch form")
	}
	if len(items) == 0 {
		return 0, fmt.Errorf("add at least one item; one non-empty line creates one item")
	}
	if len(items) > maxBatchItems {
		return 0, fmt.Errorf("a batch can contain at most %d items; split this list into smaller batches", maxBatchItems)
	}
	return len(items), nil
}

func batchSummaryMessage(noun string, results []batchItemResult) string {
	succeeded, failed, skipped, notRun := 0, 0, 0, 0
	for _, result := range results {
		switch {
		case result.Success:
			succeeded++
		case result.Skipped:
			skipped++
		case result.NotRun:
			notRun++
		default:
			failed++
		}
	}
	return fmt.Sprintf("Batch finished: %d %s item(s) created, %d failed, %d skipped, %d not run.", succeeded, noun, failed, skipped, notRun)
}

func batchItemCount(kind formKind, fields []formField) int {
	index := batchInputIndex(kind)
	if index < 0 || index >= len(fields) {
		return 0
	}
	return len(batchLines(fields[index].value))
}

func batchInputIndex(kind formKind) int {
	if kind == formBulkIssues {
		return 1
	}
	return 0
}

func (m *model) retryFailedBatch() {
	if !isBulkForm(m.lastBatchForm) || len(m.lastBatchFields) == 0 {
		return
	}
	failed := make([]string, 0)
	m.batchCarryResults = nil
	for _, result := range m.batchResults {
		if !result.Success && result.Retryable {
			failed = append(failed, result.Item)
		} else {
			// Keep successful, skipped and permanent-failure outcomes visible
			// when this retry finishes; only retryable items are replaced.
			m.batchCarryResults = append(m.batchCarryResults, result)
		}
	}
	if len(failed) == 0 {
		m.batchCarryResults = nil
		m.status, m.statusError = "There are no retryable failures in the last batch.", false
		return
	}
	m.batchRetrying = true
	m.form = m.lastBatchForm
	m.formFields = append([]formField(nil), m.lastBatchFields...)
	index := batchInputIndex(m.form)
	m.formFields[index].value = strings.Join(failed, "\n")
	m.formReturnPage, m.formReturnRepo = pageBatchResults, m.repoContext
	m.formTitle = formTitle(m.form) + " · retry failed only"
	m.formError = ""
	m.formFocus = index
	m.formCaret = len([]rune(m.formFields[index].value))
	m.batchConfirm = false
	m.page = pageForm
	m.cursor, m.scroll = 0, 0
	m.status, m.statusError = fmt.Sprintf("Retrying %d failed item(s) only; previously successful items are excluded.", len(failed)), false
}

func (m *model) renderBatchResults(c *canvas, p palette) {
	c.title(m.batchTitle + " · Results")
	succeeded, failed, skipped, notRun, retryable := 0, 0, 0, 0, 0
	for _, result := range m.batchResults {
		switch {
		case result.Success:
			succeeded++
		case result.Skipped:
			skipped++
		case result.NotRun:
			notRun++
		default:
			failed++
		}
		if !result.Success && result.Retryable {
			retryable++
		}
	}
	c.muted("Each item was processed independently. A failure does not undo successful items.")
	c.panel("BATCH SUMMARY", fmt.Sprintf("%d succeeded · %d failed · %d skipped · %d not run · %d retryable", succeeded, failed, skipped, notRun, retryable))
	for _, result := range m.batchResults {
		label, tone := "FAILED", "danger"
		switch {
		case result.Success:
			label, tone = "CREATED", "success"
		case result.Skipped:
			label, tone = "SKIPPED", "warning"
		case result.NotRun:
			label, tone = "NOT RUN", "muted"
		}
		c.raw(p.badge(tone, label))
		c.space(1)
		c.lineText(truncateVisible(result.Item, max(8, c.width-17)))
		c.space(2)
		c.muted("  " + result.Detail)
		c.blank()
	}
	buttons := []buttonSpec{}
	if retryable > 0 {
		buttons = append(buttons, buttonSpec{fmt.Sprintf("Retry %d retryable only", retryable), "batch:retry-failed", true})
	}
	buttons = append(buttons, buttonSpec{"Back to workspace", "nav:overview", false}, buttonSpec{"Issues", "nav:issues", false}, buttonSpec{"Jobs", "nav:jobs", false})
	c.buttonRow(buttons...)
}

// nextBatchItemCmd executes exactly one write at a time. Returning a Bubble Tea
// command keeps the UI responsive while the application service performs I/O.
func (m *model) nextBatchItemCmd() tea.Cmd {
	run := m.batchRun
	if run == nil || run.index >= len(run.items) {
		return nil
	}
	item := run.items[run.index]
	if run.kind == formBulkIssues {
		if run.seen == nil {
			run.seen = map[string]bool{}
		}
		key := strings.ToLower(strings.TrimSpace(item))
		if run.seen[key] {
			return func() tea.Msg {
				return batchItemDoneMsg{result: batchItemResult{
					Item: item, Skipped: true,
					Detail: "Duplicate title in this batch; skipped without creating another Issue.",
				}}
			}
		}
		run.seen[key] = true
	}
	kind := run.kind
	fields := append([]string(nil), run.fields...)
	ctx, app := m.ctx, m.app
	return func() tea.Msg {
		return batchItemDoneMsg{result: createBatchItem(ctx, app, kind, fields, item)}
	}
}

func createBatchItem(ctx context.Context, app *application.Service, kind formKind, fields []string, item string) batchItemResult {
	result := batchItemResult{Item: item}
	if app == nil {
		result.Detail = "Application service is not available."
		result.Retryable = true
		return result
	}
	get := func(index int) string {
		if index < 0 || index >= len(fields) {
			return ""
		}
		return strings.TrimSpace(fields[index])
	}
	switch kind {
	case formBulkIssues:
		priority := get(3)
		if priority == "" {
			priority = "normal"
		}
		created, err := app.CreateIssue(ctx, get(0), item, get(2), "", "", priority)
		if err != nil {
			result.Detail, result.Retryable = err.Error(), true
			return result
		}
		result.Success = true
		result.Detail = fmt.Sprintf("Created %s#%d", created.RepositoryName, created.Number)
	case formBulkQuotes:
		created, err := app.CreateQuote(ctx, item, get(1))
		if err != nil {
			result.Detail, result.Retryable = err.Error(), true
			return result
		}
		result.Success = true
		result.Detail = "Saved quote · " + created.ID
	case formBulkJobs:
		job, err := parseScheduledJobFields(get(1), get(2))
		if err != nil {
			result.Detail = err.Error()
			return result
		}
		job.ID = ""
		job.Name = item
		job.ActionType = strings.ToLower(get(3))
		job.Description = get(4)
		job.Enabled = true
		job.Notify = !strings.EqualFold(get(5), "no") && !strings.EqualFold(get(5), "false") && get(5) != "0"
		created, err := app.CreateScheduledJob(ctx, job)
		if err != nil {
			result.Detail, result.Retryable = err.Error(), true
			return result
		}
		result.Success = true
		result.Detail = "Scheduled · " + scheduleSummary(created)
	default:
		result.Detail = "Unsupported batch operation."
	}
	return result
}

// scrollBatchContent supports long batches without requiring a mouse wheel.
func (m *model) scrollBatchContent(direction int) {
	step := max(1, m.bodyHeight-2)
	m.scroll = max(0, m.scroll+direction*step)
	m.clampViewport()
}

func (m *model) requestBatchCancel() {
	if m.batchRun == nil {
		return
	}
	m.batchRun.cancelRequested = true
	m.status = "Cancellation queued; the current item will finish, then the remaining items will be skipped."
	m.statusError = false
}

// finishBatch moves the completed batch to its result page and refreshes the
// snapshots used by the rest of the workspace.
func (m *model) finishBatch(cancelled bool) tea.Cmd {
	currentResults := []batchItemResult(nil)
	if m.batchRun != nil {
		currentResults = append(currentResults, m.batchRun.results...)
	}
	m.batchResults = append(append([]batchItemResult(nil), m.batchCarryResults...), currentResults...)
	m.batchCarryResults = nil
	m.batchRetrying = false
	m.batchRun = nil
	m.page = pageBatchResults
	m.cursor, m.scroll = 0, 0
	m.buttonFocus, m.focusAction, m.focusIndex = false, "", -1
	m.status = batchSummaryMessage(batchKindLabel(m.lastBatchForm), m.batchResults)
	if cancelled {
		m.status = "Batch stopped safely; completed records were kept and unattempted items can be retried."
	}
	m.statusError = false
	return loadSnapshotCmd(m.ctx, m.app)
}

func batchKindLabel(kind formKind) string {
	switch kind {
	case formBulkIssues:
		return "Issue"
	case formBulkQuotes:
		return "quote"
	case formBulkJobs:
		return "scheduled job"
	default:
		return "batch"
	}
}

func (m *model) renderBatchProgress(c *canvas, p palette) {
	run := m.batchRun
	if run == nil {
		c.title("Batch progress")
		c.panel("NO ACTIVE BATCH", "There is no batch currently running.")
		c.buttonRow(buttonSpec{"View results", "nav:overview", true})
		return
	}
	c.title(m.batchTitle + " · In progress")
	total := len(run.items)
	completed := len(run.results)
	succeeded, failed, skipped := 0, 0, 0
	for _, result := range run.results {
		switch {
		case result.Success:
			succeeded++
		case result.Skipped:
			skipped++
		case !result.NotRun:
			failed++
		}
	}
	c.panel("PROGRESS", fmt.Sprintf("%d / %d processed · %d succeeded · %d failed · %d skipped", completed, total, succeeded, failed, skipped))
	if run.cancelRequested {
		c.muted("Cancellation requested. The current operation will finish before the remaining items are skipped.")
	} else if run.index < total {
		c.panel("CURRENT ITEM", fmt.Sprintf("%d of %d\n%s", run.index+1, total, run.items[run.index]))
	}
	if completed > 0 {
		c.muted("COMPLETED ITEMS")
		for _, result := range run.results {
			label, tone := "FAILED", "danger"
			switch {
			case result.Success:
				label, tone = "CREATED", "success"
			case result.Skipped:
				label, tone = "SKIPPED", "warning"
			case result.NotRun:
				label, tone = "NOT RUN", "muted"
			}
			c.raw(p.badge(tone, label))
			c.space(1)
			c.lineText(truncateVisible(result.Item, max(8, c.width-18)))
			if result.Detail != "" {
				c.muted("  " + result.Detail)
			}
			c.blank()
		}
	}
	c.buttonRow(buttonSpec{"Cancel after current item", "batch:cancel", true})
	c.muted("Esc: request cancellation · Ctrl+Q: exit after current item finishes")
}
