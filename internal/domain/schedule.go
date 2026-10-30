package domain

import (
	"fmt"
	"strings"
	"time"
)

const (
	ScheduleInterval = "interval"
	ScheduleOnce     = "once"
	ScheduleDaily    = "daily"
	ScheduleWeekly   = "weekly"
	ScheduleMonthly  = "monthly"
)

// PrepareScheduledJob fills defaults, validates the schedule's shape, and computes its first
// occurrence when the caller has not supplied one. The caller decides whether past one-off
// times should be rejected; storage also uses this for deterministic due-job tests/imports.
func PrepareScheduledJob(job ScheduledJob, now time.Time) (ScheduledJob, error) {
	job.Name = strings.TrimSpace(job.Name)
	job.Description = strings.TrimSpace(job.Description)
	job.ActionType = strings.TrimSpace(strings.ToLower(job.ActionType))
	job.ScheduleType = strings.ToLower(strings.TrimSpace(job.ScheduleType))
	if job.Name == "" {
		return ScheduledJob{}, fmt.Errorf("%w: job name is required", ErrInvalid)
	}
	if job.ActionType == "" {
		job.ActionType = "custom-reminder"
	}
	if job.ScheduleType == "" {
		job.ScheduleType = ScheduleInterval
	}
	if now.IsZero() {
		now = Now()
	}
	if now.Location() != time.Local {
		now = now.In(time.Local)
	}

	switch job.ScheduleType {
	case ScheduleInterval:
		if job.IntervalMinutes < 1 || job.IntervalMinutes > 525600 {
			return ScheduledJob{}, fmt.Errorf("%w: interval must be between 1 minute and 365 days", ErrInvalid)
		}
		job.ScheduleTime = ""
		job.ScheduleWeekday = 0
		job.ScheduleDay = 0
		if job.NextRunAt == nil {
			next := now.Add(time.Duration(job.IntervalMinutes) * time.Minute)
			job.NextRunAt = &next
		}
	case ScheduleOnce:
		if job.NextRunAt == nil || job.NextRunAt.IsZero() {
			return ScheduledJob{}, fmt.Errorf("%w: a one-time job needs a date and time", ErrInvalid)
		}
		job.IntervalMinutes = 0
		job.ScheduleTime = ""
		job.ScheduleWeekday = 0
		job.ScheduleDay = 0
	case ScheduleDaily:
		if _, err := time.Parse("15:04", job.ScheduleTime); err != nil {
			return ScheduledJob{}, fmt.Errorf("%w: daily time must use HH:MM (24-hour time)", ErrInvalid)
		}
		job.IntervalMinutes, job.ScheduleWeekday, job.ScheduleDay = 0, 0, 0
		next, err := nextCalendarOccurrence(job, now)
		if err != nil {
			return ScheduledJob{}, err
		}
		job.NextRunAt = &next
	case ScheduleWeekly:
		if job.ScheduleWeekday < 0 || job.ScheduleWeekday > 6 {
			return ScheduledJob{}, fmt.Errorf("%w: weekday must be between 0 (Sunday) and 6 (Saturday)", ErrInvalid)
		}
		if _, err := time.Parse("15:04", job.ScheduleTime); err != nil {
			return ScheduledJob{}, fmt.Errorf("%w: weekly time must use HH:MM (24-hour time)", ErrInvalid)
		}
		job.IntervalMinutes, job.ScheduleDay = 0, 0
		next, err := nextCalendarOccurrence(job, now)
		if err != nil {
			return ScheduledJob{}, err
		}
		job.NextRunAt = &next
	case ScheduleMonthly:
		if job.ScheduleDay < 1 || job.ScheduleDay > 31 {
			return ScheduledJob{}, fmt.Errorf("%w: monthly day must be between 1 and 31", ErrInvalid)
		}
		if _, err := time.Parse("15:04", job.ScheduleTime); err != nil {
			return ScheduledJob{}, fmt.Errorf("%w: monthly time must use HH:MM (24-hour time)", ErrInvalid)
		}
		job.IntervalMinutes, job.ScheduleWeekday = 0, 0
		next, err := nextCalendarOccurrence(job, now)
		if err != nil {
			return ScheduledJob{}, err
		}
		job.NextRunAt = &next
	default:
		return ScheduledJob{}, fmt.Errorf("%w: unsupported schedule type %q", ErrInvalid, job.ScheduleType)
	}
	return job, nil
}

// NextScheduledOccurrence returns the first scheduled time strictly after after. Interval jobs
// keep their original cadence while the local server was not running; missed intervals are skipped
// instead of causing a burst of stale notifications. Calendar schedules retain local wall time.
func NextScheduledOccurrence(job ScheduledJob, after time.Time) (time.Time, error) {
	if after.IsZero() {
		after = Now()
	}
	switch strings.ToLower(strings.TrimSpace(job.ScheduleType)) {
	case "", ScheduleInterval:
		if job.IntervalMinutes < 1 {
			return time.Time{}, fmt.Errorf("%w: interval must be positive", ErrInvalid)
		}
		if job.NextRunAt == nil || job.NextRunAt.IsZero() {
			return after.Add(time.Duration(job.IntervalMinutes) * time.Minute), nil
		}
		base := job.NextRunAt.In(after.Location())
		if base.After(after) {
			return base, nil
		}
		interval := time.Duration(job.IntervalMinutes) * time.Minute
		missed := after.Sub(base)/interval + 1
		return base.Add(missed * interval), nil
	case ScheduleOnce:
		return time.Time{}, nil
	case ScheduleDaily, ScheduleWeekly, ScheduleMonthly:
		return nextCalendarOccurrence(job, after)
	default:
		return time.Time{}, fmt.Errorf("%w: unsupported schedule type %q", ErrInvalid, job.ScheduleType)
	}
}

func nextCalendarOccurrence(job ScheduledJob, after time.Time) (time.Time, error) {
	clock, err := time.Parse("15:04", job.ScheduleTime)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: schedule time must use HH:MM (24-hour time)", ErrInvalid)
	}
	loc := time.Local
	local := after.In(loc)
	hour, minute := clock.Hour(), clock.Minute()
	candidate := time.Date(local.Year(), local.Month(), local.Day(), hour, minute, 0, 0, loc)
	switch job.ScheduleType {
	case ScheduleDaily:
		if !candidate.After(local) {
			candidate = candidate.AddDate(0, 0, 1)
		}
	case ScheduleWeekly:
		dayDelta := (job.ScheduleWeekday - int(local.Weekday()) + 7) % 7
		candidate = candidate.AddDate(0, 0, dayDelta)
		if !candidate.After(local) {
			candidate = candidate.AddDate(0, 0, 7)
		}
	case ScheduleMonthly:
		for monthDelta := 0; monthDelta < 24; monthDelta++ {
			monthBase := time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, loc).AddDate(0, monthDelta, 0)
			day := job.ScheduleDay
			lastDay := time.Date(monthBase.Year(), monthBase.Month()+1, 0, 0, 0, 0, 0, loc).Day()
			if day > lastDay {
				day = lastDay
			}
			candidate = time.Date(monthBase.Year(), monthBase.Month(), day, hour, minute, 0, 0, loc)
			if candidate.After(local) {
				return candidate, nil
			}
		}
		return time.Time{}, fmt.Errorf("%w: could not calculate next monthly occurrence", ErrInvalid)
	default:
		return time.Time{}, fmt.Errorf("%w: calendar schedule required", ErrInvalid)
	}
	return candidate, nil
}
