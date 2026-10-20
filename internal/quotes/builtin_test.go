package quotes

import (
	"strings"
	"testing"
)

func TestBuiltinQuotesHaveAttributionAndSources(t *testing.T) {
	items := Builtin()
	if len(items) < 10 {
		t.Fatalf("builtin quote count = %d, want at least 10", len(items))
	}
	seen := map[string]bool{}
	for _, item := range items {
		if strings.TrimSpace(item.Text) == "" || strings.TrimSpace(item.Author) == "" || strings.TrimSpace(item.Source) == "" || !strings.HasPrefix(item.SourceURL, "https://") {
			t.Errorf("quote is missing text, attribution, source or HTTPS reference: %+v", item)
		}
		if seen[item.Text] {
			t.Errorf("duplicate builtin quote text %q", item.Text)
		}
		seen[item.Text] = true
	}
}
