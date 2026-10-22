package validation

import "testing"

func TestSharedRules(t *testing.T) {
	if err := Text("title", "  ", true, 100); err == nil {
		t.Fatal("blank required text must fail")
	}
	if err := Text("title", "valid", true, 100); err != nil {
		t.Fatal(err)
	}
	if err := Text("description", string(make([]byte, 0)), false, 10); err != nil {
		t.Fatal(err)
	}
	if err := ISODate("due date", "2026-02-30", false); err == nil {
		t.Fatal("invalid calendar date must fail")
	}
	if err := ISODate("due date", "2026-10-22", false); err != nil {
		t.Fatal(err)
	}
	if err := HexColor("color", "#4af"); err != nil {
		t.Fatal(err)
	}
	if err := HexColor("color", "red"); err == nil {
		t.Fatal("invalid color must fail")
	}
	if err := URL("website", "javascript:alert(1)"); err == nil {
		t.Fatal("non-http URL must fail")
	}
	if err := OneOf("status", "open", "open", "closed"); err != nil {
		t.Fatal(err)
	}
}
