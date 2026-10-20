package domain

import "time"

type NoteLabel struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Color     string    `json:"color"`
	CreatedAt time.Time `json:"created_at"`
}

type Note struct {
	ID                string      `json:"id"`
	Title             string      `json:"title"`
	Content           string      `json:"content"`
	LinkedEntityType  string      `json:"linked_entity_type,omitempty"`
	LinkedEntityID    string      `json:"linked_entity_id,omitempty"`
	LinkedEntityLabel string      `json:"linked_entity_label,omitempty"`
	GraphVisible      bool        `json:"graph_visible"`
	Labels            []NoteLabel `json:"labels,omitempty"`
	CreatedAt         time.Time   `json:"created_at"`
	UpdatedAt         time.Time   `json:"updated_at"`
}

type Quote struct {
	ID        string    `json:"id"`
	Text      string    `json:"text"`
	Author    string    `json:"author,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type ScheduledJob struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	Description     string     `json:"description,omitempty"`
	ActionType      string     `json:"action_type"`
	ScheduleType    string     `json:"schedule_type"`
	IntervalMinutes int        `json:"interval_minutes,omitempty"`
	ScheduleTime    string     `json:"schedule_time,omitempty"`
	ScheduleWeekday int        `json:"schedule_weekday"`
	ScheduleDay     int        `json:"schedule_day,omitempty"`
	Enabled         bool       `json:"enabled"`
	Notify          bool       `json:"notify"`
	NextRunAt       *time.Time `json:"next_run_at,omitempty"`
	LastRunAt       *time.Time `json:"last_run_at,omitempty"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type Notification struct {
	ID        string     `json:"id"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	Level     string     `json:"level"`
	ReadAt    *time.Time `json:"read_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

type WorkspaceProfile struct {
	Username  string    `json:"username"`
	FullName  string    `json:"full_name"`
	Bio       string    `json:"bio"`
	Readme    string    `json:"readme"`
	UpdatedAt time.Time `json:"updated_at"`
}

type WorkspacePin struct {
	EntityType string    `json:"entity_type"`
	EntityID   string    `json:"entity_id"`
	Label      string    `json:"label,omitempty"`
	Color      string    `json:"color"`
	Position   int       `json:"position"`
	CreatedAt  time.Time `json:"created_at"`
}

type SeedReport struct {
	AlreadySeeded bool `json:"already_seeded"`
	Repositories  int  `json:"repositories"`
	Issues        int  `json:"issues"`
	Projects      int  `json:"projects"`
	Concepts      int  `json:"concepts"`
	Notes         int  `json:"notes"`
	Quotes        int  `json:"quotes"`
	Jobs          int  `json:"jobs"`
}
