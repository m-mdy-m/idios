package localgit

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCLIEngineOfflineWorkflow(t *testing.T) {
	// A blank inherited committer name must not override the explicit/tag fallback identity.
	t.Setenv("GIT_COMMITTER_NAME", "")
	t.Setenv("GIT_AUTHOR_NAME", "")
	t.Setenv("GIT_AUTHOR_EMAIL", "")
	engine := NewCLIEngine()
	if !engine.Availability().Available {
		t.Skip("Git CLI is not installed in this test environment")
	}

	ctx := context.Background()
	repository := filepath.Join(t.TempDir(), "working-copy")
	if err := engine.Init(ctx, repository, "main"); err != nil {
		t.Fatalf("initialize repository: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repository, "README.md"), []byte("# Local first\n"), 0o600); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	if err := engine.Stage(ctx, repository, []string{"README.md"}); err != nil {
		t.Fatalf("stage file: %v", err)
	}
	status, err := engine.Status(ctx, repository)
	if err != nil {
		t.Fatalf("read staged status: %v", err)
	}
	if !status.Initialized || status.Staged != 1 || status.Clean {
		t.Fatalf("unexpected staged status: %+v", status)
	}
	commit, err := engine.Commit(ctx, repository, "Add local README", "IDIOS Test", "idios-test@example.invalid")
	if err != nil {
		t.Fatalf("create local commit: %v", err)
	}
	if commit.SHA == "" || commit.AuthorEmail != "idios-test@example.invalid" {
		t.Fatalf("commit provenance was not recorded: %+v", commit)
	}
	if err := engine.CreateTag(ctx, repository, "v0.1.0", "First local Git snapshot"); err != nil {
		t.Fatalf("create annotated tag: %v", err)
	}
	if err := engine.CreateTag(ctx, repository, "snapshot", ""); err != nil {
		t.Fatalf("create lightweight tag: %v", err)
	}
	tags, err := engine.Tags(ctx, repository)
	if err != nil {
		t.Fatalf("list local tags: %v", err)
	}
	if len(tags) != 2 {
		t.Fatalf("expected two local tags, got %+v", tags)
	}
	var annotated, lightweight bool
	for _, tag := range tags {
		switch tag.Name {
		case "v0.1.0":
			annotated = tag.Annotated && tag.TargetSHA == commit.SHA && tag.Message == "First local Git snapshot"
		case "snapshot":
			lightweight = !tag.Annotated && tag.TargetSHA == commit.SHA
		}
	}
	if !annotated || !lightweight {
		t.Fatalf("annotated and lightweight tags should point to the real commit: %+v", tags)
	}
	if err := engine.CreateBranch(ctx, repository, "feature/offline-work", "main"); err != nil {
		t.Fatalf("create branch: %v", err)
	}
	branches, err := engine.Branches(ctx, repository)
	if err != nil {
		t.Fatalf("list branches: %v", err)
	}
	if len(branches) != 2 || !branches[0].Current && !branches[1].Current {
		t.Fatalf("expected two branches and one checked out: %+v", branches)
	}
	if err := engine.SwitchBranch(ctx, repository, "main"); err != nil {
		t.Fatalf("switch branch: %v", err)
	}
	status, err = engine.Status(ctx, repository)
	if err != nil {
		t.Fatalf("read clean status: %v", err)
	}
	if !status.Clean || status.Branch != "main" {
		t.Fatalf("expected clean main branch, got %+v", status)
	}
	log, err := engine.Log(ctx, repository, 5)
	if err != nil {
		t.Fatalf("read commit history: %v", err)
	}
	if len(log) != 1 || log[0].SHA != commit.SHA {
		t.Fatalf("unexpected local history: %+v", log)
	}
}

func TestSafeRelativePathRejectsTraversal(t *testing.T) {
	for _, value := range []string{
		"../secret",
		"folder/../../secret",
		"/etc/passwd",
		`\\etc\passwd`,
		`C:\Windows\system.ini`,
		"C:relative-to-drive",
		`\\server\share\secret`,
		"",
	} {
		if _, err := safeRelativePath(value); err == nil {
			t.Errorf("expected %q to be rejected", value)
		}
	}
	if got, err := safeRelativePath("src/main.go"); err != nil || got != "src/main.go" {
		t.Fatalf("valid relative path rejected: %q, %v", got, err)
	}
}

func TestCLIEngineReportsUnavailableWithoutGit(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	engine := NewCLIEngine()
	status, err := engine.Status(context.Background(), filepath.Join(t.TempDir(), "not-initialized"))
	if err != nil {
		t.Fatalf("status should report unavailable Git as a capability, not an operation error: %v", err)
	}
	if status.Available || status.Initialized {
		t.Fatalf("expected unavailable, uninitialized status: %+v", status)
	}
	if status.Backend != "git-cli" || status.Reason == "" {
		t.Fatalf("expected a clear unavailable reason: %+v", status)
	}
}

func TestCLIEngineRejectsNestedDirectoryAsSeparateWorkingCopy(t *testing.T) {
	engine := NewCLIEngine()
	if !engine.Availability().Available {
		t.Skip("Git CLI is not installed in this test environment")
	}
	ctx := context.Background()
	root := filepath.Join(t.TempDir(), "parent-repository")
	if err := engine.Init(ctx, root, "main"); err != nil {
		t.Fatalf("initialize parent repository: %v", err)
	}
	nested := filepath.Join(root, "nested")
	if err := os.Mkdir(nested, 0o700); err != nil {
		t.Fatalf("create nested directory: %v", err)
	}
	status, err := engine.Status(ctx, nested)
	if err != nil {
		t.Fatalf("inspect nested directory: %v", err)
	}
	if status.Initialized {
		t.Fatalf("nested directory must not be accepted as an independent working copy: %+v", status)
	}
	if err := engine.Stage(ctx, nested, nil); err == nil {
		t.Fatal("staging from a nested path must not operate on the parent repository")
	}
}


func TestWorkingCopyLockSerializesConcurrentMutations(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "repository")
	release, err := lockWorkingCopy(directory, true)
	if err != nil {
		t.Fatalf("acquire first working-copy lock: %v", err)
	}
	acquired := make(chan struct{})
	go func() {
		unlock, lockErr := lockWorkingCopy(directory, true)
		if lockErr == nil {
			close(acquired)
			unlock()
		}
	}()
	select {
	case <-acquired:
		release()
		t.Fatal("a second writer acquired the same working-copy lock before the first writer released it")
	case <-time.After(40 * time.Millisecond):
	}
	release()
	select {
	case <-acquired:
	case <-time.After(2 * time.Second):
		t.Fatal("waiting writer did not acquire the working-copy lock after release")
	}
}

func TestCLIEngineHonorsCancelledCommandContext(t *testing.T) {
	engine := NewCLIEngine()
	if !engine.Availability().Available {
		t.Skip("Git CLI is not installed in this test environment")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := engine.run(ctx, "", "version"); err == nil {
		t.Fatal("expected a cancelled Git command to return an error")
	}
}
