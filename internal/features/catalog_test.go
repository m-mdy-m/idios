package features

import (
	"reflect"
	"testing"
)

func TestCatalogIDsAreUniqueAndNavigableFeaturesHaveRoutes(t *testing.T) {
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
	}
}

func TestWebNavigationOrderingIsStableAndIncludesCoreDestinations(t *testing.T) {
	got := WebNavigation()
	ids := make([]string, 0, len(got))
	for _, item := range got {
		ids = append(ids, item.ID)
	}
	want := []string{
		"overview", "issues", "repositories", "projects", "pull-requests", "profile",
		"graph", "concepts", "shelf", "notes", "quotes", "activity", "plans", "planning", "jobs",
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
