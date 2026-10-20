package domain

import (
	"testing"
	"time"
)

func TestPrepareScheduledJobSupportsIntervalAndCalendarSchedules(t *testing.T) {
	loc := time.Local
	now := time.Date(2026, 10, 20, 8, 30, 0, 0, loc)
	cases := []struct {
		name string
		job  ScheduledJob
		want time.Time
	}{
		{
			name: "interval hourly",
			job:  ScheduledJob{Name: "hourly", ScheduleType: ScheduleInterval, IntervalMinutes: 60},
			want: now.Add(time.Hour),
		},
		{
			name: "daily at wall time",
			job:  ScheduledJob{Name: "daily", ScheduleType: ScheduleDaily, ScheduleTime: "09:00"},
			want: time.Date(2026, 10, 20, 9, 0, 0, 0, loc),
		},
		{
			name: "weekly at wall time",
			job:  ScheduledJob{Name: "weekly", ScheduleType: ScheduleWeekly, ScheduleWeekday: int(time.Monday), ScheduleTime: "09:00"},
			want: time.Date(2026, 10, 26, 9, 0, 0, 0, loc),
		},
		{
			name: "monthly day",
			job:  ScheduledJob{Name: "monthly", ScheduleType: ScheduleMonthly, ScheduleDay: 21, ScheduleTime: "10:15"},
			want: time.Date(2026, 10, 21, 10, 15, 0, 0, loc),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := PrepareScheduledJob(tc.job, now)
			if err != nil {
				t.Fatal(err)
			}
			if got.NextRunAt == nil || !got.NextRunAt.Equal(tc.want) {
				t.Fatalf("next run = %v, want %v", got.NextRunAt, tc.want)
			}
		})
	}
}

func TestNextScheduledOccurrenceSkipsMissedIntervalsWithoutDrift(t *testing.T) {
	base := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	job := ScheduledJob{ScheduleType: ScheduleInterval, IntervalMinutes: 60, NextRunAt: &base}
	got, err := NextScheduledOccurrence(job, base.Add(3*time.Hour+10*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	want := base.Add(4 * time.Hour)
	if !got.Equal(want) {
		t.Fatalf("next run = %s, want cadence-preserving %s", got, want)
	}
}

func TestMonthlyScheduleClampsToLastDayOfMonth(t *testing.T) {
	loc := time.Local
	after := time.Date(2026, 2, 1, 0, 0, 0, 0, loc)
	job := ScheduledJob{ScheduleType: ScheduleMonthly, ScheduleDay: 31, ScheduleTime: "17:45"}
	got, err := NextScheduledOccurrence(job, after)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 2, 28, 17, 45, 0, 0, loc)
	if !got.Equal(want) {
		t.Fatalf("next monthly run = %s, want %s", got, want)
	}
}

func TestPrepareScheduledJobRejectsInvalidSchedules(t *testing.T) {
	now := time.Now()
	cases := []ScheduledJob{
		{Name: "bad interval", ScheduleType: ScheduleInterval, IntervalMinutes: 0},
		{Name: "bad daily", ScheduleType: ScheduleDaily, ScheduleTime: "tomorrow"},
		{Name: "bad weekly day", ScheduleType: ScheduleWeekly, ScheduleWeekday: 7, ScheduleTime: "09:00"},
		{Name: "bad monthly day", ScheduleType: ScheduleMonthly, ScheduleDay: 32, ScheduleTime: "09:00"},
		{Name: "bad type", ScheduleType: "whenever"},
	}
	for _, job := range cases {
		if _, err := PrepareScheduledJob(job, now); err == nil {
			t.Errorf("PrepareScheduledJob(%+v) succeeded; want error", job)
		}
	}
}
