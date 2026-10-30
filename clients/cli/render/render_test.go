package render

import (
	"bytes"
	"strings"
	"testing"

	"idios/internal/domain"
)

// Go maps iterate in random order; command output must not shuffle between runs.
func TestFieldsSortKeys(t *testing.T) {
	var buf bytes.Buffer
	Fields(&buf, map[string]any{"b": 2, "a": 1, "c": 3})
	if got, want := buf.String(), "a: 1\nb: 2\nc: 3\n"; got != want {
		t.Fatalf("Fields = %q, want %q", got, want)
	}
}

func TestValuePrintsStringMapInSortedOrder(t *testing.T) {
	var buf bytes.Buffer
	Value(&buf, map[string]string{"status": "linked", "concept": "CPU"}, false)
	if got, want := buf.String(), "concept: CPU\nstatus: linked\n"; got != want {
		t.Fatalf("Value = %q, want %q", got, want)
	}
}

func TestValueJSONUsesFieldNames(t *testing.T) {
	var buf bytes.Buffer
	Value(&buf, domain.Repository{ID: "repo_1", Name: "CPU"}, true)
	if !strings.Contains(buf.String(), `"name": "CPU"`) {
		t.Fatalf("JSON output = %q", buf.String())
	}
}
