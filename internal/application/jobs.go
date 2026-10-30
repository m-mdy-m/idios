package application

import (
	"context"
	"fmt"
	"idios/internal/domain"
	"idios/internal/ports"
	"idios/internal/validation"
	"strings"
	"time"
)

type JobsService struct{ store ports.JobsStore }

func (s *JobsService) ListScheduledJobs(ctx context.Context) ([]domain.ScheduledJob, error) {
	return s.store.ListScheduledJobs(ctx)
}

func validateScheduledJobAction(action string) error {
	action = strings.ToLower(strings.TrimSpace(action))
	if action == "" {
		action = "custom-reminder"
	}
	switch action {
	case "daily-plan", "weekly-review", "blocked-issues", "open-issues", "custom-reminder":
		return nil
	default:
		return fmt.Errorf("%w: unsupported reminder kind %q", domain.ErrInvalid, action)
	}
}

func (s *JobsService) CreateScheduledJob(ctx context.Context, job domain.ScheduledJob) (domain.ScheduledJob, error) {
	if err := invalidInput(validation.Text("job name", job.Name, true, 200)); err != nil {
		return domain.ScheduledJob{}, err
	}
	if err := invalidInput(validation.Text("job description", job.Description, false, 10000)); err != nil {
		return domain.ScheduledJob{}, err
	}
	if err := validateScheduledJobAction(job.ActionType); err != nil {
		return domain.ScheduledJob{}, err
	}
	prepared, err := domain.PrepareScheduledJob(job, domain.Now().Local())
	if err != nil {
		return domain.ScheduledJob{}, err
	}
	if prepared.ScheduleType == domain.ScheduleOnce && (prepared.NextRunAt == nil || !prepared.NextRunAt.After(domain.Now())) {
		return domain.ScheduledJob{}, fmt.Errorf("%w: one-time job date and time must be in the future", domain.ErrInvalid)
	}
	return s.store.CreateScheduledJob(ctx, prepared)
}

func (s *JobsService) UpdateScheduledJob(ctx context.Context, job domain.ScheduledJob) (domain.ScheduledJob, error) {
	if err := invalidInput(validation.Text("job name", job.Name, true, 200)); err != nil {
		return domain.ScheduledJob{}, err
	}
	if err := invalidInput(validation.Text("job description", job.Description, false, 10000)); err != nil {
		return domain.ScheduledJob{}, err
	}
	if strings.TrimSpace(job.ID) == "" {
		return domain.ScheduledJob{}, fmt.Errorf("%w: job ID is required", domain.ErrInvalid)
	}
	if err := validateScheduledJobAction(job.ActionType); err != nil {
		return domain.ScheduledJob{}, err
	}
	prepared, err := domain.PrepareScheduledJob(job, domain.Now().Local())
	if err != nil {
		return domain.ScheduledJob{}, err
	}
	if prepared.ScheduleType == domain.ScheduleOnce && (prepared.NextRunAt == nil || !prepared.NextRunAt.After(domain.Now())) {
		return domain.ScheduledJob{}, fmt.Errorf("%w: one-time job date and time must be in the future", domain.ErrInvalid)
	}
	return s.store.UpdateScheduledJob(ctx, prepared)
}

func (s *JobsService) DeleteScheduledJob(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("%w: job ID is required", domain.ErrInvalid)
	}
	return s.store.DeleteScheduledJob(ctx, id)
}

func (s *JobsService) ToggleScheduledJob(ctx context.Context, id string) (domain.ScheduledJob, error) {
	if strings.TrimSpace(id) == "" {
		return domain.ScheduledJob{}, fmt.Errorf("%w: job ID is required", domain.ErrInvalid)
	}
	return s.store.ToggleScheduledJob(ctx, id)
}

func (s *JobsService) RunScheduledJobNow(ctx context.Context, id string, now time.Time, notificationsEnabled bool) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("%w: job ID is required", domain.ErrInvalid)
	}
	return s.store.RunScheduledJobNow(ctx, id, now, notificationsEnabled)
}

func (s *JobsService) ProcessDueJobs(ctx context.Context, now time.Time, notificationsEnabled bool) (int, error) {
	return s.store.ProcessDueJobs(ctx, now, notificationsEnabled)
}
