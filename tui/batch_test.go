package tui

import (
	"context"
	"strings"
	"testing"

	"idios/internal/domain"
)

func TestBatchLinesTrimsBlankLinesAndNormalizesNewlines(t *testing.T) {
	got := batchLines("  first  \r\n\r\nsecond\n  third \r")
	want := []string{"first", "second", "third"}
	if len(got) != len(want) {
		t.Fatalf("batchLines() = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("batchLines()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestValidateBulkIssuesRequiresExistingRepositoryAndTitles(t *testing.T) {
	repositories := []domain.Repository{{Name: "IDIOS"}, {Name: "ARLIZ"}}
	fields := []string{"idios", "Fix navigation\nAdd batch creation", "shared brief", "normal"}
	count, err := validateBulkForm(formBulkIssues, fields, repositories)
	if err != nil || count != 2 {
		t.Fatalf("valid batch: count=%d err=%v, want 2 and nil", count, err)
	}
	fields[0] = "missing"
	if _, err := validateBulkForm(formBulkIssues, fields, repositories); err == nil {
		t.Fatal("unknown repository should fail validation")
	}
	fields = []string{"IDIOS", "\n  \n", "", "normal"}
	if _, err := validateBulkForm(formBulkIssues, fields, repositories); err == nil {
		t.Fatal("empty batch should fail validation")
	}
}

func TestBatchPreviewDoesNotCreateUntilConfirmed(t *testing.T) {
	m := newModel(context.Background(), nil, "", Options{NoColor: true})
	m.repositories = []domain.Repository{{Name: "IDIOS"}}
	m.beginForm(formBulkIssues)
	m.formFields[0].value = "IDIOS"
	m.formFields[1].value = "First issue\nSecond issue"
	m.formFields[2].value = "Shared description"
	m.formFields[3].value = "normal"
	if cmd := m.submitForm(); cmd != nil {
		t.Fatal("first submission should only show a preview")
	}
	if !m.batchConfirm {
		t.Fatal("first submission should enter batch confirmation state")
	}
	view := m.View()
	if !strings.Contains(view.Content, "BATCH PREVIEW") || !strings.Contains(view.Content, "2 item(s)") {
		t.Fatalf("batch preview should show the item count, got:\n%s", view.Content)
	}
}

func TestBulkFormDoesNotConfirmAfterEditing(t *testing.T) {
	m := newModel(context.Background(), nil, "", Options{NoColor: true})
	m.repositories = []domain.Repository{{Name: "IDIOS"}}
	m.beginForm(formBulkIssues)
	m.formFields[0].value = "IDIOS"
	m.formFields[1].value = "First issue"
	if cmd := m.submitForm(); cmd != nil || !m.batchConfirm {
		t.Fatal("expected first submission to preview")
	}
	m.formFocus = 1
	m.handleFormKey("x")
	if m.batchConfirm {
		t.Fatal("editing after preview must invalidate the previous confirmation")
	}
}

func TestSidebarAvoidsDuplicatingRepositoryList(t *testing.T) {
	m := newModel(context.Background(), nil, "", Options{NoColor: true})
	m.page = pageIssues
	m.width, m.height = 110, 36
	m.repositories = []domain.Repository{{Name: "CPU"}, {Name: "ARLIZ"}}
	view := m.View()
	if strings.Contains(view.Content, "YOUR REPOSITORIES") {
		t.Fatal("sidebar should not repeat the complete repository index")
	}
	for _, want := range []string{"QUICK CREATE", "Issues (bulk)", "Quotes (bulk)", "Jobs (bulk)", "Exit IDIOS", "Commands"} {
		if !strings.Contains(view.Content, want) {
			t.Errorf("workspace utility %q is missing", want)
		}
	}
}

func TestCommandPaletteFiltersAndRunsNavigationAction(t *testing.T) {
	m := newModel(context.Background(), nil, "", Options{NoColor: true})
	m.page = pageIssues
	m.openCommandPalette()
	m.commandQuery = "bulk save quotes"
	entries := m.filteredCommandEntries()
	if len(entries) != 1 || entries[0].action != "form:bulk-quotes" {
		t.Fatalf("filtered command entries = %#v, want only bulk quotes", entries)
	}
	m.commandQuery = "repositories"
	entries = m.filteredCommandEntries()
	if len(entries) == 0 {
		t.Fatal("command palette should find repository navigation")
	}
	m.closeCommandPalette()
	if m.page != pageIssues {
		t.Fatalf("closing command palette returned to %q, want %q", m.page, pageIssues)
	}
}

func TestQuitConfirmationCanBeCancelledAndExitHasAnimation(t *testing.T) {
	m := newModel(context.Background(), nil, "", Options{NoColor: true})
	m.page = pageJobs
	m.repoContext = "IDIOS"
	m.openQuitConfirmation()
	if m.page != pageQuit {
		t.Fatalf("quit page = %q, want %q", m.page, pageQuit)
	}
	m.cancelQuitConfirmation()
	if m.page != pageJobs || m.repoContext != "IDIOS" {
		t.Fatalf("cancelled quit should restore page and repository context: page=%q repo=%q", m.page, m.repoContext)
	}
	cmd := m.startExitAnimation()
	if cmd == nil || !m.exitAnimating || m.page != pageQuit {
		t.Fatal("exit request should start an animated shutdown")
	}
}

func TestBatchProcessesItemsSequentiallyAndKeepsPerItemResults(t *testing.T) {
	m := newModel(context.Background(), nil, "", Options{NoColor: true})
	m.lastBatchForm = formBulkQuotes
	m.batchTitle = "Quote batch"
	m.batchRun = &batchRunState{kind: formBulkQuotes, items: []string{"first quote", "second quote"}, fields: []string{"first quote\nsecond quote", "author"}}
	m.page = pageBatchProgress

	cmd := m.nextBatchItemCmd()
	if cmd == nil {
		t.Fatal("first item should produce a command")
	}
	msg := cmd()
	_, next := m.Update(msg)
	if next == nil {
		t.Fatal("after the first result, the second item should be scheduled")
	}
	msg = next()
	updated, _ := m.Update(msg)
	resultModel := updated.(*model)
	if resultModel.page != pageBatchResults {
		t.Fatalf("page after processing batch = %q, want batch results", resultModel.page)
	}
	if len(resultModel.batchResults) != 2 {
		t.Fatalf("batch results = %d, want independent result for both items", len(resultModel.batchResults))
	}
	for i, result := range resultModel.batchResults {
		if result.Item != []string{"first quote", "second quote"}[i] || result.Success {
			t.Errorf("result[%d] = %#v; nil app should yield a per-item failure", i, result)
		}
	}
}

func TestCancellingBatchMarksRemainingItemsRetryableNotRun(t *testing.T) {
	m := newModel(context.Background(), nil, "", Options{NoColor: true})
	m.lastBatchForm = formBulkQuotes
	m.batchTitle = "Quote batch"
	m.batchRun = &batchRunState{kind: formBulkQuotes, items: []string{"first", "second", "third"}, fields: []string{"first\nsecond\nthird"}}
	m.page = pageBatchProgress
	cmd := m.nextBatchItemCmd()
	m.requestBatchCancel()
	updated, _ := m.Update(cmd())
	resultModel := updated.(*model)
	if resultModel.page != pageBatchResults {
		t.Fatalf("page after cancellation = %q, want batch results", resultModel.page)
	}
	if len(resultModel.batchResults) != 3 {
		t.Fatalf("results count = %d, want 3 (one attempted, two not run)", len(resultModel.batchResults))
	}
	if !resultModel.batchResults[1].NotRun || !resultModel.batchResults[1].Retryable || !resultModel.batchResults[2].NotRun || !resultModel.batchResults[2].Retryable {
		t.Fatalf("unattempted items must be marked retryable: %#v", resultModel.batchResults)
	}
}

func TestDuplicateIssueTitlesAreSkippedWithoutRetry(t *testing.T) {
	m := newModel(context.Background(), nil, "", Options{NoColor: true})
	m.batchRun = &batchRunState{kind: formBulkIssues, items: []string{"Fix sidebar", " fix sidebar "}, seen: map[string]bool{}}
	first := m.nextBatchItemCmd()
	if first == nil || !m.batchRun.seen["fix sidebar"] {
		t.Fatal("first normalized issue title should be reserved for this batch")
	}
	_, _ = m.Update(first())
	second := m.nextBatchItemCmd()
	if second == nil {
		t.Fatal("duplicate should be reported as a result, not create a record")
	}
	msg := second().(batchItemDoneMsg)
	if !msg.result.Skipped || msg.result.Retryable || !strings.Contains(msg.result.Detail, "Duplicate title") {
		t.Fatalf("duplicate issue result = %#v", msg.result)
	}
}

func TestRetryKeepsEarlierSuccessesInFinalBatchReport(t *testing.T) {
	m := newModel(context.Background(), nil, "", Options{NoColor: true})
	m.page = pageBatchResults
	m.batchTitle = "Quote batch"
	m.lastBatchForm = formBulkQuotes
	m.lastBatchFields = []formField{{label: "Quotes", value: "saved quote\nfailed quote", multiline: true}, {label: "Author", value: "source"}}
	m.batchResults = []batchItemResult{
		{Item: "saved quote", Success: true, Detail: "Saved quote · quote-1"},
		{Item: "failed quote", Retryable: true, Detail: "temporary error"},
	}
	m.retryFailedBatch()
	if !m.batchRetrying || m.formFields[0].value != "failed quote" {
		t.Fatal("retry composer should contain only the failed quote")
	}
	if cmd := m.submitForm(); cmd != nil || !m.batchConfirm {
		t.Fatal("first submission should preview the retry")
	}
	cmd := m.submitForm()
	if cmd == nil || m.batchRun == nil || len(m.batchRun.items) != 1 || m.batchRun.items[0] != "failed quote" {
		t.Fatal("confirmed retry should schedule only the previously failed quote")
	}
	updated, _ := m.Update(cmd())
	resultModel := updated.(*model)
	if len(resultModel.batchResults) != 2 || !resultModel.batchResults[0].Success || resultModel.batchResults[1].Success {
		t.Fatalf("final report should preserve the earlier success and replace only the retry result: %#v", resultModel.batchResults)
	}
}

func TestExitDuringBatchWaitsForCurrentItemThenStartsGoodbye(t *testing.T) {
	m := newModel(context.Background(), nil, "", Options{NoColor: true})
	m.lastBatchForm = formBulkQuotes
	m.batchRun = &batchRunState{kind: formBulkQuotes, items: []string{"one", "two"}, fields: []string{"one\ntwo"}}
	m.page = pageBatchProgress
	if cmd := m.startExitAnimation(); cmd != nil {
		t.Fatal("exit during a live batch must wait instead of quitting immediately")
	}
	if !m.exitAfterBatch || !m.batchRun.cancelRequested || m.exitAnimating {
		t.Fatal("exit should request cancellation and defer the goodbye animation")
	}
	cmd := m.nextBatchItemCmd()
	updated, next := m.Update(cmd())
	resultModel := updated.(*model)
	if !resultModel.exitAnimating || resultModel.page != pageQuit || next == nil {
		t.Fatal("goodbye animation should start after the current item finishes")
	}
	if len(resultModel.batchResults) != 2 || !resultModel.batchResults[1].NotRun {
		t.Fatalf("shutdown should preserve current result and mark future item not run: %#v", resultModel.batchResults)
	}
}

func TestRetryPreservesNonRetryableResultsAndSkipsSuccessfulItems(t *testing.T) {
	m := newModel(context.Background(), nil, "", Options{NoColor: true})
	m.lastBatchForm = formBulkIssues
	m.lastBatchFields = []formField{{label: "Repository", value: "IDIOS"}, {label: "Issue titles", value: "existing\nfailed", multiline: true}, {label: "Shared description", value: "shared"}, {label: "Priority", value: "normal"}}
	m.batchResults = []batchItemResult{
		{Item: "existing", Success: true, Detail: "Created IDIOS#8"},
		{Item: "failed", Retryable: true, Detail: "temporary error"},
		{Item: "duplicate", Skipped: true, Detail: "Duplicate title"},
	}
	m.repositories = []domain.Repository{{Name: "IDIOS"}}
	m.retryFailedBatch()
	if m.formFields[1].value != "failed" {
		t.Fatalf("retry form titles = %q, want only failed", m.formFields[1].value)
	}
	if len(m.batchCarryResults) != 2 || !m.batchCarryResults[0].Success || !m.batchCarryResults[1].Skipped {
		t.Fatalf("carry-forward should retain success and skipped outcomes: %#v", m.batchCarryResults)
	}
}

func TestCommandPaletteDoesNotInterruptAnOpenForm(t *testing.T) {
	m := newModel(context.Background(), nil, "", Options{NoColor: true})
	m.page = pageForm
	m.form = formBulkQuotes
	m.openCommandPalette()
	if m.page != pageForm || m.form != formBulkQuotes || !m.statusError {
		t.Fatalf("command palette should not hide a live form: page=%q form=%q statusError=%v", m.page, m.form, m.statusError)
	}
}

func TestBatchPagesSupportKeyboardPaging(t *testing.T) {
	m := newModel(context.Background(), nil, "", Options{NoColor: true})
	m.page = pageBatchResults
	m.contentLines = 200
	m.bodyHeight = 30
	m.scrollBatchContent(1)
	if m.scroll != 28 {
		t.Fatalf("page down scroll = %d, want 28", m.scroll)
	}
	m.scrollBatchContent(-1)
	if m.scroll != 0 {
		t.Fatalf("page up should clamp at the start, got %d", m.scroll)
	}
}
