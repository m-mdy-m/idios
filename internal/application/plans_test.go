package application

import (
	"context"
	"testing"

	"idios/internal/domain"
)

func TestCreatePlanRequiresAtLeastOneTask(t *testing.T) {
	service := &PlanService{}
	_, err := service.CreatePlan(context.Background(), domain.Plan{
		Title:    "Plan without tasks",
		PlanDate: "2026-10-23",
	}, []string{"", "   "})
	if err == nil {
		t.Fatal("expected a plan with no non-empty tasks to be rejected")
	}
}

func TestValidatePlanAllowsAnUnlinkedWorkspacePlan(t *testing.T) {
	plan := domain.Plan{Title: "Workspace plan", PlanDate: "2026-10-23"}
	tasks, err := validatePlan(plan, []string{"Write notes"})
	if err != nil {
		t.Fatalf("unlinked plan should be valid: %v", err)
	}
	if len(tasks) != 1 || tasks[0] != "Write notes" {
		t.Fatalf("unexpected sanitized task list: %#v", tasks)
	}
}
