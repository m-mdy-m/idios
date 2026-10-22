package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"idios/internal/domain"
)

func TestPlansTasksAndReminderDeliveryPersist(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "plans.sqlite3")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	repo, err := store.CreateRepository(ctx, "PlanRepo", "plans test")
	if err != nil {
		t.Fatal(err)
	}
	issue, err := store.CreateIssue(ctx, domain.Issue{RepositoryID: repo.ID, Title: "Implement planned task"})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := store.CreatePlan(ctx, domain.Plan{
		Title:            "Finish the feature",
		Description:      "Small steps, done deliberately.",
		PlanDate:         "2026-10-23",
		LinkedEntityType: "issue",
		LinkedEntityID:   issue.ID,
	}, []string{"Review design", "Implement feature", "Write tests"})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Tasks) != 3 || plan.LinkedEntityID != issue.ID || plan.LinkedEntityLabel == "" {
		t.Fatalf("created plan did not retain tasks/link: %+v", plan)
	}

	completed, err := store.SetPlanTaskDone(ctx, plan.ID, plan.Tasks[0].ID, true)
	if err != nil || !completed.Done || completed.CompletedAt == nil {
		t.Fatalf("complete task = %+v, err=%v", completed, err)
	}
	listed, err := store.ListPlans(ctx, "2026-10-23", "2026-10-23")
	if err != nil || len(listed) != 1 || !listed[0].Tasks[0].Done {
		t.Fatalf("list plans = %+v, err=%v", listed, err)
	}

	now := time.Date(2026, 10, 23, 8, 30, 0, 0, time.Local)
	created, err := store.CreatePlanReminderNotification(ctx, "2026-10-23", "morning", "Today's plans", "A friendly nudge", now)
	if err != nil || !created {
		t.Fatalf("first reminder delivery created=%v err=%v", created, err)
	}
	duplicate, err := store.CreatePlanReminderNotification(ctx, "2026-10-23", "morning", "Today's plans", "Duplicate", now.Add(time.Minute))
	if err != nil || duplicate {
		t.Fatalf("duplicate reminder delivery created=%v err=%v", duplicate, err)
	}
	notifications, err := store.ListNotifications(ctx, 50)
	if err != nil || len(notifications) != 1 || notifications[0].RelatedEntityType != "plan-date" || notifications[0].RelatedEntityID != "2026-10-23" {
		t.Fatalf("reminder notification = %+v, err=%v", notifications, err)
	}
}

func TestPlanRemindersOnlyDeliverForDatesWithPlans(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(filepath.Join(t.TempDir(), "no-plan-reminders.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	created, err := store.CreatePlanReminderNotification(ctx, "2026-10-24", "midday", "Reminder", "No plans", time.Now())
	if err != nil || created {
		t.Fatalf("reminder with no plans created=%v err=%v", created, err)
	}
	notifications, err := store.ListNotifications(ctx, 50)
	if err != nil || len(notifications) != 0 {
		t.Fatalf("notifications = %+v, err=%v; want none", notifications, err)
	}
}
