package validation

import (
	"strings"
	"testing"
)

func TestValidateCommandInputRejectsUnknownFieldsBeforeValidatingValues(t *testing.T) {
	_, err := ValidateCommandInput("issue.create", []Field{{Name: "title", Label: "Title", Type: "text", Required: true}}, map[string]any{"unexpected": 7, "title": 42})
	if err == nil || !strings.Contains(err.Error(), "unexpected") {
		t.Fatalf("error = %v, want unknown field to be reported first", err)
	}
}

func TestValidateCommandInputCollectsMissingRequiredFields(t *testing.T) {
	_, err := ValidateCommandInput("issue.create", []Field{{Name: "repo", Label: "Repository", Type: "text", Required: true}, {Name: "title", Label: "Title", Type: "text", Required: true}}, nil)
	if err == nil || !strings.Contains(err.Error(), "Repository") || !strings.Contains(err.Error(), "Title") {
		t.Fatalf("error = %v, want both required fields", err)
	}
}

func TestValidateCommandInputNormalizesAndRejectsInvalidData(t *testing.T) {
	fields := []Field{{Name: "name", Label: "Name", Type: "text", Required: true}, {Name: "priority", Label: "Priority", Type: "select", Options: []string{"low", "normal", "high"}}, {Name: "weekday", Label: "Weekday", Type: "number"}, {Name: "enabled", Label: "Enabled", Type: "boolean"}}
	got, err := ValidateCommandInput("fixture", fields, map[string]any{"name": "  daily review ", "priority": "high", "weekday": float64(3), "enabled": true})
	if err != nil {
		t.Fatal(err)
	}
	if got["name"] != "daily review" || got["weekday"] != 3 || got["enabled"] != true {
		t.Fatalf("normalized input = %#v", got)
	}
	for _, invalid := range []map[string]any{{"name": "job", "priority": "critical"}, {"name": "job", "weekday": 1.25}, {"name": "job", "weekday": "nine"}, {"name": "job", "enabled": "perhaps"}} {
		if _, err := ValidateCommandInput("fixture", fields, invalid); err == nil {
			t.Fatalf("expected invalid input to fail: %#v", invalid)
		}
	}
}

func TestValidateCommandInputSupportsDynamicSelectsAndDateTimeFormats(t *testing.T) {
	fields := []Field{{Name: "repo", Label: "Repository", Type: "select", Required: true, Dynamic: true}, {Name: "when", Label: "When", Type: "datetime"}, {Name: "due", Label: "Due", Type: "date"}}
	if _, err := ValidateCommandInput("fixture", fields, map[string]any{"repo": "local-repository-id", "when": "2026-10-22T13:30", "due": "2026-10-31"}); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateCommandInput("fixture", fields, map[string]any{"repo": "local-repository-id", "when": "next Tuesday"}); err == nil {
		t.Fatal("invalid datetime should be rejected")
	}
}
