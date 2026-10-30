package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"idios/internal/application"
	"idios/internal/storage/sqlite"
)

func runCLI(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	factory := func(path string) (*application.Service, io.Closer, error) {
		store, err := sqlite.OpenStore(path)
		if err != nil {
			return nil, nil, err
		}
		return application.New(store), store, nil
	}
	code := Run(context.Background(), args, &stdout, &stderr, factory)
	return code, stdout.String(), stderr.String()
}

func TestJSONProtocolAndRepositoryCreation(t *testing.T) {
	db := filepath.Join(t.TempDir(), "engine.sqlite3")
	code, out, errOut := runCLI(t, "protocol", "--json")
	if code != 0 || errOut != "" {
		t.Fatalf("protocol: exit=%d stderr=%q", code, errOut)
	}
	var protocol map[string]any
	if err := json.Unmarshal([]byte(out), &protocol); err != nil {
		t.Fatalf("protocol output is not JSON: %v\n%s", err, out)
	}
	if protocol["protocol_version"] != float64(1) {
		t.Fatalf("unexpected protocol version: %#v", protocol["protocol_version"])
	}

	code, out, errOut = runCLI(t, "--db", db, "repo", "create", "--name", "CPU", "--json")
	if code != 0 || errOut != "" {
		t.Fatalf("repo create: exit=%d stderr=%q", code, errOut)
	}
	var repo map[string]any
	if err := json.Unmarshal([]byte(out), &repo); err != nil {
		t.Fatalf("repo output is not JSON: %v\n%s", err, out)
	}
	if repo["name"] != "CPU" {
		t.Fatalf("repo name = %#v, want CPU", repo["name"])
	}

	code, out, errOut = runCLI(t, "issue", "list", "--repo", "CPU", "--db", db, "--json")
	if code != 0 || errOut != "" {
		t.Fatalf("issue list with global flags after command: exit=%d stderr=%q", code, errOut)
	}
	var issues []any
	if err := json.Unmarshal([]byte(out), &issues); err != nil {
		t.Fatalf("issue list output is not JSON: %v\n%s", err, out)
	}
}

func TestJSONErrorsUseStableEnvelope(t *testing.T) {
	db := filepath.Join(t.TempDir(), "engine.sqlite3")
	code, _, errOut := runCLI(t, "--db", db, "--json", "issue", "start", "--id", "Missing#1")
	if code == 0 {
		t.Fatal("starting missing issue should fail")
	}
	var envelope struct {
		ProtocolVersion int `json:"protocol_version"`
		Error           struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(errOut), &envelope); err != nil {
		t.Fatalf("error output is not a JSON envelope: %v\n%s", err, errOut)
	}
	if envelope.ProtocolVersion != 1 || envelope.Error.Code != "not_found" || envelope.Error.Message == "" {
		t.Fatalf("unexpected error envelope: %+v", envelope)
	}
}

func TestVersionReportsStableRelease(t *testing.T) {
	code, stdout, stderr := runCLI(t, "version")
	if code != 0 {
		t.Fatalf("version exit code = %d, want 0; stderr=%q", code, stderr)
	}
	if got := strings.TrimSpace(stdout); got != "3.0.2" {
		t.Fatalf("version output = %q, want 3.0.2", got)
	}
	if stderr != "" {
		t.Fatalf("version wrote unexpected stderr: %q", stderr)
	}
}
