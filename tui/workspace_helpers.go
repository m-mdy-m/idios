package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"idios/internal/domain"
)

func noteLabelNames(labels []domain.NoteLabel) string {
	parts := make([]string, 0, len(labels))
	for _, label := range labels {
		parts = append(parts, label.Name)
	}
	return strings.Join(parts, ", ")
}

func splitNoteLabels(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, part := range parts {
		name := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(part), "#"))
		key := strings.ToLower(name)
		if name == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, name)
	}
	return out
}

func jobScheduleInput(job domain.ScheduledJob) string {
	switch job.ScheduleType {
	case domain.ScheduleOnce:
		if job.CompletedAt != nil || job.NextRunAt == nil {
			return time.Now().Add(time.Hour).Format("2006-01-02 15:04")
		}
		return job.NextRunAt.Local().Format("2006-01-02 15:04")
	case domain.ScheduleDaily:
		return job.ScheduleTime
	case domain.ScheduleWeekly:
		day := time.Weekday(job.ScheduleWeekday).String()
		return day[:3] + " " + job.ScheduleTime
	case domain.ScheduleMonthly:
		return fmt.Sprintf("%d %s", job.ScheduleDay, job.ScheduleTime)
	default:
		minutes := job.IntervalMinutes
		if minutes < 1 {
			minutes = 60
		}
		switch {
		case minutes%1440 == 0:
			return fmt.Sprintf("%dd", minutes/1440)
		case minutes%60 == 0:
			return fmt.Sprintf("%dh", minutes/60)
		default:
			return fmt.Sprintf("%dm", minutes)
		}
	}
}

func parseScheduledJobFields(scheduleType, details string) (domain.ScheduledJob, error) {
	job := domain.ScheduledJob{ScheduleType: strings.ToLower(strings.TrimSpace(scheduleType))}
	details = strings.TrimSpace(details)
	switch job.ScheduleType {
	case domain.ScheduleInterval:
		minutes, err := parseIntervalMinutes(details)
		if err != nil {
			return domain.ScheduledJob{}, err
		}
		job.IntervalMinutes = minutes
	case domain.ScheduleOnce:
		at, err := time.ParseInLocation("2006-01-02 15:04", details, time.Local)
		if err != nil {
			return domain.ScheduledJob{}, fmt.Errorf("one-time date/time must use YYYY-MM-DD HH:MM, for example 2026-10-25 14:30")
		}
		job.NextRunAt = &at
	case domain.ScheduleDaily:
		if _, err := time.Parse("15:04", details); err != nil {
			return domain.ScheduledJob{}, fmt.Errorf("daily time must use 24-hour HH:MM, for example 09:00")
		}
		job.ScheduleTime = details
	case domain.ScheduleWeekly:
		parts := strings.Fields(details)
		if len(parts) != 2 {
			return domain.ScheduledJob{}, fmt.Errorf("weekly schedule must be weekday and time, for example Mon 09:00")
		}
		weekday, err := parseWeekday(parts[0])
		if err != nil {
			return domain.ScheduledJob{}, err
		}
		if _, err := time.Parse("15:04", parts[1]); err != nil {
			return domain.ScheduledJob{}, fmt.Errorf("weekly time must use 24-hour HH:MM, for example Mon 09:00")
		}
		job.ScheduleWeekday, job.ScheduleTime = weekday, parts[1]
	case domain.ScheduleMonthly:
		parts := strings.Fields(strings.ReplaceAll(details, "@", " "))
		if len(parts) != 2 {
			return domain.ScheduledJob{}, fmt.Errorf("monthly schedule must be day and time, for example 15 09:00")
		}
		day, err := strconv.Atoi(parts[0])
		if err != nil || day < 1 || day > 31 {
			return domain.ScheduledJob{}, fmt.Errorf("monthly day must be a number from 1 to 31")
		}
		if _, err := time.Parse("15:04", parts[1]); err != nil {
			return domain.ScheduledJob{}, fmt.Errorf("monthly time must use 24-hour HH:MM, for example 15 09:00")
		}
		job.ScheduleDay, job.ScheduleTime = day, parts[1]
	default:
		return domain.ScheduledJob{}, fmt.Errorf("schedule type must be interval, once, daily, weekly or monthly")
	}
	return job, nil
}

func parseIntervalMinutes(value string) (int, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return 0, fmt.Errorf("interval is required; examples: 30m, 1h, 2d")
	}
	if strings.HasSuffix(value, "d") {
		n, err := strconv.Atoi(strings.TrimSpace(strings.TrimSuffix(value, "d")))
		if err == nil && n > 0 && n <= 365 {
			return n * 1440, nil
		}
		return 0, fmt.Errorf("day interval must be from 1d to 365d")
	}
	if strings.HasSuffix(value, "w") {
		n, err := strconv.Atoi(strings.TrimSpace(strings.TrimSuffix(value, "w")))
		if err == nil && n > 0 && n <= 52 {
			return n * 7 * 1440, nil
		}
		return 0, fmt.Errorf("week interval must be from 1w to 52w")
	}
	if n, err := strconv.Atoi(value); err == nil {
		if n >= 1 && n <= 525600 {
			return n, nil
		}
		return 0, fmt.Errorf("numeric interval is interpreted as minutes (1–525600)")
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration < time.Minute || duration%time.Minute != 0 || duration > 365*24*time.Hour {
		return 0, fmt.Errorf("invalid interval %q; use whole minutes, such as 30m, 1h, 2d or 1w", value)
	}
	return int(duration / time.Minute), nil
}

func parseWeekday(value string) (int, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	weekdays := map[string]time.Weekday{
		"sun": time.Sunday, "sunday": time.Sunday,
		"mon": time.Monday, "monday": time.Monday,
		"tue": time.Tuesday, "tues": time.Tuesday, "tuesday": time.Tuesday,
		"wed": time.Wednesday, "wednesday": time.Wednesday,
		"thu": time.Thursday, "thur": time.Thursday, "thurs": time.Thursday, "thursday": time.Thursday,
		"fri": time.Friday, "friday": time.Friday,
		"sat": time.Saturday, "saturday": time.Saturday,
	}
	day, ok := weekdays[value]
	if !ok {
		return 0, fmt.Errorf("weekday must be Sun, Mon, Tue, Wed, Thu, Fri or Sat")
	}
	return int(day), nil
}

func (m *model) filteredNotes() []domain.Note {
	out := make([]domain.Note, 0, len(m.notes))
	for _, note := range m.notes {
		if m.activeNoteLabel != "" {
			found := false
			for _, label := range note.Labels {
				if strings.EqualFold(label.Name, m.activeNoteLabel) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		parts := []string{note.Title, note.Content, note.LinkedEntityLabel, noteLabelNames(note.Labels), strings.Join(stringsToTagNames(note.Labels), " ")}
		if containsFold(strings.Join(parts, " "), m.search) {
			out = append(out, note)
		}
	}
	return out
}
