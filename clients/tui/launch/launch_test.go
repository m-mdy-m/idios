package tuilaunch

import (
	"context"
	"path/filepath"
	"testing"
)

func TestCleanAndDemoVariantsUseIndependentDatabases(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	cleanPath := filepath.Join(root, "clean", "workspace.sqlite3")
	demoPath := filepath.Join(root, "demo", "workspace.sqlite3")

	cleanApp, cleanCloser, err := openApplication(ctx, cleanPath, false)
	if err != nil {
		t.Fatalf("open clean workspace: %v", err)
	}
	cleanRepos, err := cleanApp.ListRepositories(ctx)
	if err != nil {
		_ = cleanCloser.Close()
		t.Fatalf("list clean repositories: %v", err)
	}
	if len(cleanRepos) != 0 {
		_ = cleanCloser.Close()
		t.Fatalf("new clean workspace contains %d repositories; want 0", len(cleanRepos))
	}
	if err := cleanCloser.Close(); err != nil {
		t.Fatalf("close clean workspace: %v", err)
	}

	demoApp, demoCloser, err := openApplication(ctx, demoPath, true)
	if err != nil {
		t.Fatalf("open demo workspace: %v", err)
	}
	demoRepos, err := demoApp.ListRepositories(ctx)
	if err != nil {
		_ = demoCloser.Close()
		t.Fatalf("list demo repositories: %v", err)
	}
	if len(demoRepos) == 0 {
		_ = demoCloser.Close()
		t.Fatal("demo workspace contains no repositories after automatic seeding")
	}
	if err := demoCloser.Close(); err != nil {
		t.Fatalf("close demo workspace: %v", err)
	}

	// Reopening the standard database must still be clean after the demo database was seeded.
	cleanAgain, cleanCloser, err := openApplication(ctx, cleanPath, false)
	if err != nil {
		t.Fatalf("reopen clean workspace: %v", err)
	}
	defer cleanCloser.Close()
	cleanRepos, err = cleanAgain.ListRepositories(ctx)
	if err != nil {
		t.Fatalf("re-list clean repositories: %v", err)
	}
	if len(cleanRepos) != 0 {
		t.Fatalf("demo seed leaked into clean workspace: got %d repositories", len(cleanRepos))
	}
}

func TestDemoLaunchSeedIsIdempotent(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "demo.sqlite3")
	first, firstCloser, err := openApplication(ctx, path, true)
	if err != nil {
		t.Fatalf("first demo launch: %v", err)
	}
	firstRepos, err := first.ListRepositories(ctx)
	if err != nil {
		_ = firstCloser.Close()
		t.Fatal(err)
	}
	if err := firstCloser.Close(); err != nil {
		t.Fatal(err)
	}

	second, secondCloser, err := openApplication(ctx, path, true)
	if err != nil {
		t.Fatalf("second demo launch: %v", err)
	}
	defer secondCloser.Close()
	secondRepos, err := second.ListRepositories(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(secondRepos) != len(firstRepos) {
		t.Fatalf("demo launch duplicated repositories: first=%d second=%d", len(firstRepos), len(secondRepos))
	}
}
