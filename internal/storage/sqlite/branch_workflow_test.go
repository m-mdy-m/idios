package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"idios/internal/domain"
)

func TestBranchesAndDraftPullRequestWorkflow(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(filepath.Join(t.TempDir(), "branches.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	repo, err := store.CreateRepositoryWithDefaultBranch(ctx, "BranchRepo", "workflow test", "trunk")
	if err != nil {
		t.Fatal(err)
	}
	branches, err := store.ListBranches(ctx, repo.ID)
	if err != nil || len(branches) != 1 || branches[0].Name != "trunk" || !branches[0].IsDefault {
		t.Fatalf("initial branch = %+v, err=%v; want default trunk", branches, err)
	}
	feature, err := store.CreateBranch(ctx, repo.ID, "feature/work", branches[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if feature.IsDefault || feature.BaseBranchID == nil || *feature.BaseBranchID != branches[0].ID {
		t.Fatalf("new branch metadata = %+v", feature)
	}
	if _, err := store.CreateBranch(ctx, repo.ID, "feature/work", branches[0].ID); err == nil {
		t.Fatal("duplicate branch name unexpectedly succeeded")
	}

	issue, err := store.CreateIssue(ctx, domain.Issue{RepositoryID: repo.ID, Title: "Implement branch workflow", Priority: "normal", BranchID: &feature.ID})
	if err != nil {
		t.Fatal(err)
	}
	started, err := store.StartIssue(ctx, issue.ID)
	if err != nil || started.Status != domain.IssueInProgress {
		t.Fatalf("start issue = %+v, err=%v", started, err)
	}
	pr, err := store.CreatePullRequestWithBranches(ctx, issue.ID, "Review branch workflow", "Describe and verify the changes", feature.ID, branches[0].ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if !pr.IsDraft || pr.SourceBranchName != feature.Name || pr.TargetBranchName != branches[0].Name {
		t.Fatalf("pull request branch/draft data = %+v", pr)
	}
	if _, _, _, err := store.AcceptPullRequest(ctx, pr.ID, "Should not be accepted while draft"); err == nil {
		t.Fatal("draft pull request was accepted before being marked ready")
	}
	ready, err := store.MarkPullRequestReady(ctx, pr.ID)
	if err != nil || ready.IsDraft {
		t.Fatalf("mark ready = %+v, err=%v", ready, err)
	}
	accepted, completion, completedIssue, err := store.AcceptPullRequest(ctx, pr.ID, "Branch workflow tested")
	if err != nil {
		t.Fatal(err)
	}
	if accepted.Status != domain.PRAccepted || completedIssue.Status != domain.IssueDone || completion.BranchID != branches[0].ID {
		t.Fatalf("accepted PR workflow: pr=%+v completion=%+v issue=%+v", accepted, completion, completedIssue)
	}
}

func TestParentIssueReassignmentRejectsCycles(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(filepath.Join(t.TempDir(), "issue-parents.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	repo, err := store.CreateRepository(ctx, "ParentRepo", "parent issue test")
	if err != nil {
		t.Fatal(err)
	}
	parent, err := store.CreateIssue(ctx, domain.Issue{RepositoryID: repo.ID, Title: "Parent"})
	if err != nil {
		t.Fatal(err)
	}
	child, err := store.CreateIssue(ctx, domain.Issue{RepositoryID: repo.ID, Title: "Child"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AssignIssueParent(ctx, child.ID, parent.ID); err != nil {
		t.Fatalf("assign parent: %v", err)
	}
	updated, err := store.GetIssue(ctx, child.ID)
	if err != nil || updated.ParentIssueID == nil || *updated.ParentIssueID != parent.ID {
		t.Fatalf("child parent = %+v, err=%v", updated.ParentIssueID, err)
	}
	if err := store.AssignIssueParent(ctx, parent.ID, child.ID); err == nil {
		t.Fatal("cyclic parent relationship unexpectedly succeeded")
	}
	if err := store.AssignIssueParent(ctx, child.ID, ""); err != nil {
		t.Fatalf("clearing parent: %v", err)
	}
}
