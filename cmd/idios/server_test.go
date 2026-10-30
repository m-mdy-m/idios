package main

import (
	"errors"
	"strings"
	"testing"
)

func TestOpenRunningServerOpensExistingWorkspace(t *testing.T) {
	var opened string
	var output strings.Builder
	handled, err := openRunningServer(
		"http://127.0.0.1:43170",
		&output,
		func(string) (bool, map[string]any) { return true, map[string]any{"status": "ok"} },
		func(url string) error { opened = url; return nil },
	)
	if err != nil {
		t.Fatalf("openRunningServer returned error: %v", err)
	}
	if !handled {
		t.Fatal("running instance was not handled")
	}
	if opened != "http://127.0.0.1:43170" {
		t.Fatalf("browser URL = %q", opened)
	}
	if !strings.Contains(output.String(), "already running") {
		t.Fatalf("missing reuse message: %q", output.String())
	}
}

func TestOpenRunningServerLeavesStoppedServerAlone(t *testing.T) {
	called := false
	handled, err := openRunningServer(
		"http://127.0.0.1:43170",
		&strings.Builder{},
		func(string) (bool, map[string]any) { return false, nil },
		func(string) error { called = true; return nil },
	)
	if err != nil || handled || called {
		t.Fatalf("handled=%v err=%v openerCalled=%v", handled, err, called)
	}
}

func TestOpenRunningServerReportsBrowserFailure(t *testing.T) {
	want := errors.New("browser handler unavailable")
	handled, err := openRunningServer(
		"http://127.0.0.1:43170",
		&strings.Builder{},
		func(string) (bool, map[string]any) { return true, nil },
		func(string) error { return want },
	)
	if !handled || !errors.Is(err, want) {
		t.Fatalf("handled=%v err=%v, want handled and original browser error", handled, err)
	}
}
