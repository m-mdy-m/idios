package features

import (
	"reflect"
	"testing"
)

func TestCatalogIDsAreUniqueAndNavigableFeaturesHaveRoutes(t *testing.T) {
	t.Helper()
	seen := make(map[string]struct{}, len(catalog))
	for _, item := range Catalog() {
		if item.ID == "" {
			t.Fatal("feature with empty ID")
		}
		if _, exists := seen[item.ID]; exists {
			t.Fatalf("duplicate feature ID %q", item.ID)
		}
		seen[item.ID] = struct{}{}
		if item.WebNavigation && item.WebPath == "" {
			t.Errorf("web-navigation feature %q has no web path", item.ID)
		}
		if item.TUITopNavigation && item.TUIPage == "" {
			t.Errorf("TUI top-navigation feature %q has no TUI page", item.ID)
		}
		if item.TUICycleNavigation && item.TUIPage == "" {
			t.Errorf("TUI cycle-navigation feature %q has no TUI page", item.ID)
		}
		if item.TUISideNavigation && item.TUIPage == "" && item.TUIAction == "" {
			t.Errorf("TUI side-navigation feature %q has neither page nor action", item.ID)
		}
	}
}

func TestCatalogKeepsPreviouslyMissingTUIFeaturesVisibleAcrossClients(t *testing.T) {
	byID := make(map[string]Feature, len(catalog))
	for _, item := range Catalog() {
		byID[item.ID] = item
	}

	for _, id := range []string{
		"profile", "job-queue", "remove", "notification-settings", "keyboard-shortcuts", "updates",
		"repository-branches", "repository-labels", "repository-milestones", "repository-settings",
		"repository-activity", "repository-graph", "batch-progress", "batch-results",
		"issue-detail", "pull-request-detail", "project-detail", "concept-detail",
		"generic-form", "delete-confirmation", "quit-confirmation", "updates",
	} {
		item, ok := byID[id]
		if !ok {
			t.Errorf("feature %q is missing from the shared catalog", id)
			continue
		}
		if item.WebPath == "" && item.WebNavigation {
			t.Errorf("web-navigation feature %q has no Web route", id)
		}
		if item.TUIPage == "" && item.TUIAction == "" {
			t.Errorf("feature %q has no TUI destination or action", id)
		}
	}
}

func TestWebNavigationOrderingIsStableAndIncludesProfileJobsAndRemove(t *testing.T) {
	got := WebNavigation()
	ids := make([]string, 0, len(got))
	for _, item := range got {
		ids = append(ids, item.ID)
	}
	want := []string{
		"overview", "issues", "repositories", "projects", "pull-requests", "profile",
		"graph", "concepts", "notes", "quotes", "activity", "plans", "planning", "jobs",
		"commands", "remove", "logs", "help", "notifications", "settings",
	}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("WebNavigation() IDs = %v, want %v", ids, want)
	}
}

func TestNavigationFunctionsReturnDefensiveSlices(t *testing.T) {
	for name, get := range map[string]func() []Feature{
		"catalog":    Catalog,
		"web":        WebNavigation,
		"web-bottom": WebBottomNavigation,
		"tui-top":    TUITopNavigation,
		"tui-side":   TUISideNavigation,
		"tui-cycle":  TUICycleNavigation,
	} {
		first := get()
		if len(first) == 0 {
			t.Fatalf("%s navigation unexpectedly empty", name)
		}
		want := first[0]
		first[0].Label = "mutated by caller"
		second := get()
		if second[0].Label != want.Label {
			t.Errorf("%s navigation shares mutable metadata with caller", name)
		}
	}
}

func TestEveryTUIScreenHasAClientNeutralFeatureEntry(t *testing.T) {
	registered := make(map[string]bool, len(catalog))
	for _, item := range Catalog() {
		if item.TUIPage != "" {
			registered[item.TUIPage] = true
		}
	}
	pages := []string{
		"intro", "overview", "repositories", "projects", "issues", "pull-requests", "graph",
		"concepts", "notes", "quotes", "jobs", "notifications", "settings-notifications", "logs",
		"settings", "new", "repository-overview", "repository-issues", "repository-pull-requests",
		"repository-branches", "repository-graph", "repository-labels", "repository-milestones",
		"repository-settings", "repository-activity", "issue-detail", "pr-detail", "project-detail",
		"concept-detail", "form", "help", "upgrade", "quit-confirmation", "batch-results",
		"batch-progress", "command-palette", "remove-center", "delete-confirmation",
	}
	for _, page := range pages {
		if !registered[page] {
			t.Errorf("TUI screen %q has no entry in the shared feature inventory", page)
		}
	}
}
