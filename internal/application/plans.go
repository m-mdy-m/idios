package application

import (
	"context"
	"fmt"
	"idios/internal/ports"
	"strings"
	"time"

	"idios/internal/domain"
	"idios/internal/validation"
)

type PlanService struct{ store ports.PlanStore }

func validatePlanDate(value string) error {
	return invalidInput(validation.ISODate("plan date", value, true))
}

func (s *PlanService) ListPlans(ctx context.Context, from, to string) ([]domain.Plan, error) {
	if strings.TrimSpace(from) != "" {
		if err := invalidInput(validation.ISODate("start date", from, true)); err != nil {
			return nil, err
		}
	}
	if strings.TrimSpace(to) != "" {
		if err := invalidInput(validation.ISODate("end date", to, true)); err != nil {
			return nil, err
		}
	}
	if from != "" && to != "" && from > to {
		return nil, invalidInput(fmt.Errorf("start date must not be after end date"))
	}
	return s.store.ListPlans(ctx, from, to)
}

func validatePlan(plan domain.Plan, taskTitles []string) ([]string, error) {
	for _, field := range []struct {
		name, value string
		required    bool
		max         int
	}{{"plan title", plan.Title, true, 180}, {"plan description", plan.Description, false, 10000}, {"linked entity reference", plan.LinkedEntityID, false, 256}} {
		if err := invalidInput(validation.Text(field.name, field.value, field.required, field.max)); err != nil {
			return nil, err
		}
	}
	if err := validatePlanDate(plan.PlanDate); err != nil {
		return nil, err
	}
	if err := invalidInput(validation.OneOf("linked entity type", plan.LinkedEntityType, "issue", "repository")); err != nil {
		return nil, err
	}
	if plan.LinkedEntityType == "" && strings.TrimSpace(plan.LinkedEntityID) != "" {
		return nil, invalidInput(fmt.Errorf("linked entity type is required when linking a plan"))
	}
	if plan.LinkedEntityType != "" && strings.TrimSpace(plan.LinkedEntityID) == "" {
		return nil, invalidInput(fmt.Errorf("linked entity reference is required for %s plans", plan.LinkedEntityType))
	}
	if len(taskTitles) > 100 {
		return nil, invalidInput(fmt.Errorf("a plan can contain at most 100 tasks"))
	}
	clean := make([]string, 0, len(taskTitles))
	for i, title := range taskTitles {
		title = strings.TrimSpace(title)
		if title == "" {
			continue
		}
		if err := invalidInput(validation.Text(fmt.Sprintf("task %d", i+1), title, true, 500)); err != nil {
			return nil, err
		}
		clean = append(clean, title)
	}
	return clean, nil
}

func (s *PlanService) CreatePlan(ctx context.Context, plan domain.Plan, tasks []string) (domain.Plan, error) {
	clean, err := validatePlan(plan, tasks)
	if err != nil {
		return domain.Plan{}, err
	}
	if len(clean) == 0 {
		return domain.Plan{}, invalidInput(fmt.Errorf("a plan must contain at least one non-empty task"))
	}
	plan.Title = strings.TrimSpace(plan.Title)
	plan.LinkedEntityType = strings.TrimSpace(plan.LinkedEntityType)
	plan.LinkedEntityID = strings.TrimSpace(plan.LinkedEntityID)
	return s.store.CreatePlan(ctx, plan, clean)
}

func (s *PlanService) UpdatePlan(ctx context.Context, plan domain.Plan) (domain.Plan, error) {
	if err := invalidInput(validation.Text("plan reference", plan.ID, true, 256)); err != nil {
		return domain.Plan{}, err
	}
	if _, err := validatePlan(plan, nil); err != nil {
		return domain.Plan{}, err
	}
	plan.Title = strings.TrimSpace(plan.Title)
	plan.LinkedEntityType = strings.TrimSpace(plan.LinkedEntityType)
	plan.LinkedEntityID = strings.TrimSpace(plan.LinkedEntityID)
	return s.store.UpdatePlan(ctx, plan)
}

func (s *PlanService) DeletePlan(ctx context.Context, id string) error {
	if err := invalidInput(validation.Text("plan reference", id, true, 256)); err != nil {
		return err
	}
	return s.store.DeletePlan(ctx, strings.TrimSpace(id))
}
func (s *PlanService) AddPlanTask(ctx context.Context, planID, title string) (domain.PlanTask, error) {
	if err := invalidInput(validation.Text("plan reference", planID, true, 256)); err != nil {
		return domain.PlanTask{}, err
	}
	if err := invalidInput(validation.Text("task title", title, true, 500)); err != nil {
		return domain.PlanTask{}, err
	}
	return s.store.AddPlanTask(ctx, strings.TrimSpace(planID), strings.TrimSpace(title))
}
func (s *PlanService) SetPlanTaskDone(ctx context.Context, planID, taskID string, done bool) (domain.PlanTask, error) {
	for _, item := range []struct{ name, value string }{{"plan reference", planID}, {"task reference", taskID}} {
		if err := invalidInput(validation.Text(item.name, item.value, true, 256)); err != nil {
			return domain.PlanTask{}, err
		}
	}
	return s.store.SetPlanTaskDone(ctx, strings.TrimSpace(planID), strings.TrimSpace(taskID), done)
}

// CreatePlanReminder writes no duplicate notifications: SQLite atomically reserves one delivery
// per local date and reminder period. A nil result means there were no plans or this period had
// already been delivered for the date.
func (s *PlanService) CreatePlanReminder(ctx context.Context, date, period string, now time.Time) (*domain.Notification, error) {
	if err := validatePlanDate(date); err != nil {
		return nil, err
	}
	if err := invalidInput(validation.OneOf("reminder period", period, "morning", "midday", "evening")); err != nil {
		return nil, err
	}
	plans, err := s.store.ListPlans(ctx, date, date)
	if err != nil {
		return nil, err
	}
	if len(plans) == 0 {
		return nil, nil
	}
	total, done := 0, 0
	pending := make([]string, 0, 4)
	planTitles := make([]string, 0, 4)
	for _, plan := range plans {
		if len(planTitles) < 4 {
			planTitles = append(planTitles, plan.Title)
		}
		for _, task := range plan.Tasks {
			total++
			if task.Done {
				done++
			} else if len(pending) < 4 {
				pending = append(pending, plan.Title+": "+task.Title)
			}
		}
	}
	var title, body string
	switch period {
	case "morning":
		title = "Today's plans are ready ☀️"
		body = "Good morning, baby. These are your plans for today. Start with one small step and give yourself a fair chance."
	case "midday":
		title = "Plan check-in, baby? 🌿"
		body = "How are you doing with today's plans? Take a breath, check off what is done, and choose what comes next."
	case "evening":
		title = "Let's close the day gently ✨"
		body = "Quick wrap-up, baby. Mark what you finished and leave the rest for a clear plan tomorrow."
	}
	if len(planTitles) > 0 {
		body += " Today's plans: " + strings.Join(planTitles, "; ") + "."
	}
	if total > 0 {
		body += fmt.Sprintf(" Progress: %d of %d tasks done.", done, total)
	}
	if len(pending) > 0 {
		body += " Still on your list: " + strings.Join(pending, "; ") + "."
	}
	sent, err := s.store.CreatePlanReminderNotification(ctx, date, period, title, body, now)
	if err != nil {
		return nil, err
	}
	if !sent {
		return nil, nil
	}
	return &domain.Notification{Title: title, Body: body, Level: "info", RelatedEntityType: "plan-date", RelatedEntityID: date, CreatedAt: now}, nil
}
