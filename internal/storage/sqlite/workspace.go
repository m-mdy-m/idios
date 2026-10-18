package sqlite

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"idios/internal/domain"
)

func noteFrom(r Row) domain.Note {
	return domain.Note{
		ID: str(r, "id"), Title: str(r, "title"), Content: str(r, "content"),
		LinkedEntityType: str(r, "linked_entity_type"), LinkedEntityID: str(r, "linked_entity_id"),
		LinkedEntityLabel: str(r, "linked_entity_label"), GraphVisible: boolRow(r, "graph_visible"),
		CreatedAt: parseTime(r["created_at"]), UpdatedAt: parseTime(r["updated_at"]),
	}
}

const noteSelect = `SELECT n.id,n.title,n.content,n.linked_entity_type,n.linked_entity_id,n.graph_visible,n.created_at,n.updated_at,
CASE n.linked_entity_type
 WHEN 'repository' THEN (SELECT r.name FROM repositories r WHERE r.id=n.linked_entity_id)
 WHEN 'issue' THEN (SELECT r.name||'#'||i.number||' · '||i.title FROM issues i JOIN repositories r ON r.id=i.repository_id WHERE i.id=n.linked_entity_id)
 WHEN 'project' THEN (SELECT p.name FROM projects p WHERE p.id=n.linked_entity_id)
 WHEN 'concept' THEN (SELECT c.name FROM concepts c WHERE c.id=n.linked_entity_id)
 WHEN 'pull_request' THEN (SELECT r.name||'!'||pr.number||' · '||pr.title FROM pull_requests pr JOIN repositories r ON r.id=pr.repository_id WHERE pr.id=n.linked_entity_id)
 WHEN 'note' THEN (SELECT n2.title FROM notes n2 WHERE n2.id=n.linked_entity_id)
 ELSE '' END AS linked_entity_label FROM notes n`

func (s *Store) ListNotes(ctx context.Context) ([]domain.Note, error) {
	rows, err := s.p().all(ctx, noteSelect+` ORDER BY n.updated_at DESC,n.created_at DESC`)
	if err != nil {
		return nil, err
	}
	return collect(rows, noteFrom), nil
}

func (s *Store) CreateNote(ctx context.Context, title, content string) (domain.Note, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return domain.Note{}, fmt.Errorf("%w: note title is required", domain.ErrInvalid)
	}
	id, now := domain.NewID(), nowText()
	if _, err := s.p().changes(ctx, `INSERT INTO notes(id,title,content,created_at,updated_at) VALUES(?,?,?,?,?)`, id, title, strings.TrimSpace(content), now, now); err != nil {
		return domain.Note{}, err
	}
	_ = s.RecordActivity(ctx, "note", id, "note.created", marshalPayload(map[string]string{"title": title}))
	return s.getNote(ctx, id)
}

func (s *Store) UpdateNote(ctx context.Context, id, title, content string) (domain.Note, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return domain.Note{}, fmt.Errorf("%w: note title is required", domain.ErrInvalid)
	}
	changed, err := s.p().changes(ctx, `UPDATE notes SET title=?,content=?,updated_at=? WHERE id=?`, title, strings.TrimSpace(content), nowText(), id)
	if err != nil {
		return domain.Note{}, err
	}
	if changed == 0 {
		return domain.Note{}, fmt.Errorf("%w: note %q", domain.ErrNotFound, id)
	}
	_ = s.RecordActivity(ctx, "note", id, "note.updated", marshalPayload(map[string]string{"title": title}))
	return s.getNote(ctx, id)
}

func (s *Store) DeleteNote(ctx context.Context, id string) error {
	changed, err := s.p().changes(ctx, `DELETE FROM notes WHERE id=?`, id)
	if err != nil {
		return err
	}
	if changed == 0 {
		return fmt.Errorf("%w: note %q", domain.ErrNotFound, id)
	}
	return s.RecordActivity(ctx, "note", id, "note.deleted", "")
}

func (s *Store) getNote(ctx context.Context, id string) (domain.Note, error) {
	r, err := s.p().must(ctx, fmt.Sprintf("note %q", id), noteSelect+` WHERE n.id=?`, id)
	if err != nil {
		return domain.Note{}, err
	}
	return noteFrom(r), nil
}

func (s *Store) LinkNote(ctx context.Context, noteID, entityType, entityRef string, graphVisible bool) error {
	entityType = strings.TrimSpace(entityType)
	if entityType == "" || strings.TrimSpace(entityRef) == "" {
		return fmt.Errorf("%w: target type and reference are required", domain.ErrInvalid)
	}
	entityID, err := s.resolveEntityID(ctx, entityType, strings.TrimSpace(entityRef))
	if err != nil {
		return fmt.Errorf("resolve note target: %w", err)
	}
	changed, err := s.p().changes(ctx, `UPDATE notes SET linked_entity_type=?,linked_entity_id=?,graph_visible=?,updated_at=? WHERE id=?`, entityType, entityID, boolInt(graphVisible), nowText(), noteID)
	if err != nil {
		return err
	}
	if changed == 0 {
		return fmt.Errorf("%w: note %q", domain.ErrNotFound, noteID)
	}
	return s.RecordActivity(ctx, "note", noteID, "note.linked", marshalPayload(map[string]string{"entity_type": entityType, "entity_id": entityID, "graph_visible": strconv.FormatBool(graphVisible)}))
}

func quoteFrom(r Row) domain.Quote {
	return domain.Quote{ID: str(r, "id"), Text: str(r, "text"), Author: str(r, "author"), CreatedAt: parseTime(r["created_at"])}
}

func (s *Store) ListQuotes(ctx context.Context) ([]domain.Quote, error) {
	rows, err := s.p().all(ctx, `SELECT id,text,author,created_at FROM quotes ORDER BY created_at,id`)
	if err != nil {
		return nil, err
	}
	return collect(rows, quoteFrom), nil
}

func (s *Store) CreateQuote(ctx context.Context, text, author string) (domain.Quote, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return domain.Quote{}, fmt.Errorf("%w: quote text is required", domain.ErrInvalid)
	}
	id, now := domain.NewID(), nowText()
	if _, err := s.p().changes(ctx, `INSERT INTO quotes(id,text,author,created_at) VALUES(?,?,?,?)`, id, text, strings.TrimSpace(author), now); err != nil {
		return domain.Quote{}, err
	}
	_ = s.RecordActivity(ctx, "quote", id, "quote.created", "")
	r, err := s.p().must(ctx, "quote", `SELECT id,text,author,created_at FROM quotes WHERE id=?`, id)
	if err != nil {
		return domain.Quote{}, err
	}
	return quoteFrom(r), nil
}

// NextQuote rotates through saved quotes in insertion order and stores the next index in SQLite,
// so each application launch advances the quote independently from wall-clock randomness.
func (s *Store) NextQuote(ctx context.Context) (domain.Quote, error) {
	var result domain.Quote
	err := s.run(ctx, func(p provider) error {
		rows, err := p.all(ctx, `SELECT id,text,author,created_at FROM quotes ORDER BY created_at,id`)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		idx := 0
		if row, ok, err := p.row(ctx, `SELECT value FROM app_settings WHERE key='quotes.next_index'`); err != nil {
			return err
		} else if ok {
			if parsed, parseErr := strconv.Atoi(str(row, "value")); parseErr == nil && parsed >= 0 {
				idx = parsed
			}
		}
		result = quoteFrom(rows[idx%len(rows)])
		_, err = p.changes(ctx, `INSERT INTO app_settings(key,value,updated_at) VALUES('quotes.next_index',?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value,updated_at=excluded.updated_at`, strconv.Itoa((idx+1)%len(rows)), nowText())
		return err
	})
	if err != nil {
		return domain.Quote{}, err
	}
	if result.ID == "" {
		return domain.Quote{}, fmt.Errorf("%w: no quotes saved yet", domain.ErrNotFound)
	}
	return result, nil
}

func (s *Store) GetWorkspaceProfile(ctx context.Context) (domain.WorkspaceProfile, error) {
	r, err := s.p().must(ctx, "workspace profile", `SELECT username,full_name,bio,readme,updated_at FROM workspace_profile WHERE singleton_id='default'`)
	if err != nil {
		return domain.WorkspaceProfile{}, err
	}
	return domain.WorkspaceProfile{Username: str(r, "username"), FullName: str(r, "full_name"), Bio: str(r, "bio"), Readme: str(r, "readme"), UpdatedAt: parseTime(r["updated_at"])}, nil
}

func (s *Store) UpdateWorkspaceProfile(ctx context.Context, profile domain.WorkspaceProfile) (domain.WorkspaceProfile, error) {
	profile.Username, profile.FullName = strings.TrimSpace(profile.Username), strings.TrimSpace(profile.FullName)
	if profile.Username == "" || profile.FullName == "" {
		return domain.WorkspaceProfile{}, fmt.Errorf("%w: username and display name are required", domain.ErrInvalid)
	}
	_, err := s.p().changes(ctx, `INSERT INTO workspace_profile(singleton_id,username,full_name,bio,readme,updated_at) VALUES('default',?,?,?,?,?) ON CONFLICT(singleton_id) DO UPDATE SET username=excluded.username,full_name=excluded.full_name,bio=excluded.bio,readme=excluded.readme,updated_at=excluded.updated_at`, profile.Username, profile.FullName, strings.TrimSpace(profile.Bio), strings.TrimSpace(profile.Readme), nowText())
	if err != nil {
		return domain.WorkspaceProfile{}, err
	}
	return s.GetWorkspaceProfile(ctx)
}

func (s *Store) ListWorkspacePins(ctx context.Context) ([]domain.WorkspacePin, error) {
	rows, err := s.p().all(ctx, `SELECT entity_type,entity_id,label,color,position,created_at FROM workspace_pins ORDER BY position,created_at`)
	if err != nil {
		return nil, err
	}
	return collect(rows, func(r Row) domain.WorkspacePin {
		return domain.WorkspacePin{EntityType: str(r, "entity_type"), EntityID: str(r, "entity_id"), Label: str(r, "label"), Color: str(r, "color"), Position: int(i64(r["position"])), CreatedAt: parseTime(r["created_at"])}
	}), nil
}

// ToggleWorkspacePin adds/removes a locally important repository, issue, project or concept.
func (s *Store) ToggleWorkspacePin(ctx context.Context, entityType, entityRef, label, color string) (bool, error) {
	entityType = strings.TrimSpace(entityType)
	entityID, err := s.resolveEntityID(ctx, entityType, strings.TrimSpace(entityRef))
	if err != nil {
		return false, err
	}
	added := false
	err = s.run(ctx, func(p provider) error {
		if _, found, err := p.row(ctx, `SELECT entity_id FROM workspace_pins WHERE entity_type=? AND entity_id=?`, entityType, entityID); err != nil {
			return err
		} else if found {
			_, err = p.changes(ctx, `DELETE FROM workspace_pins WHERE entity_type=? AND entity_id=?`, entityType, entityID)
			return err
		}
		var pos int64
		row, ok, err := p.row(ctx, `SELECT COALESCE(MAX(position),-1)+1 AS next_position FROM workspace_pins`)
		if err != nil {
			return err
		}
		if ok {
			pos = i64(row["next_position"])
		}
		if color == "" {
			color = "accent"
		}
		if _, err := p.changes(ctx, `INSERT INTO workspace_pins(entity_type,entity_id,label,color,position,created_at) VALUES(?,?,?,?,?,?)`, entityType, entityID, strings.TrimSpace(label), color, pos, nowText()); err != nil {
			return err
		}
		added = true
		return nil
	})
	return added, err
}

func scheduledJobFrom(r Row) domain.ScheduledJob {
	return domain.ScheduledJob{ID: str(r, "id"), Name: str(r, "name"), Description: str(r, "description"), ActionType: str(r, "action_type"), IntervalMinutes: int(i64(r["interval_minutes"])), Enabled: boolRow(r, "enabled"), Notify: boolRow(r, "notify"), NextRunAt: ptrTime(r["next_run_at"]), LastRunAt: ptrTime(r["last_run_at"]), CreatedAt: parseTime(r["created_at"]), UpdatedAt: parseTime(r["updated_at"])}
}

func (s *Store) ListScheduledJobs(ctx context.Context) ([]domain.ScheduledJob, error) {
	rows, err := s.p().all(ctx, `SELECT id,name,description,action_type,interval_minutes,enabled,notify,next_run_at,last_run_at,created_at,updated_at FROM scheduled_jobs ORDER BY enabled DESC,next_run_at,name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	return collect(rows, scheduledJobFrom), nil
}

func (s *Store) CreateScheduledJob(ctx context.Context, job domain.ScheduledJob) (domain.ScheduledJob, error) {
	job.Name = strings.TrimSpace(job.Name)
	if job.Name == "" || job.IntervalMinutes < 1 {
		return domain.ScheduledJob{}, fmt.Errorf("%w: job name and a positive schedule interval are required", domain.ErrInvalid)
	}
	if job.ActionType == "" {
		job.ActionType = "custom-reminder"
	}
	if job.NextRunAt == nil {
		next := domain.Now().Add(time.Duration(job.IntervalMinutes) * time.Minute)
		job.NextRunAt = &next
	}
	id, now := domain.NewID(), nowText()
	_, err := s.p().changes(ctx, `INSERT INTO scheduled_jobs(id,name,description,action_type,interval_minutes,enabled,notify,next_run_at,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, id, job.Name, strings.TrimSpace(job.Description), job.ActionType, job.IntervalMinutes, boolInt(job.Enabled), boolInt(job.Notify), timeText(*job.NextRunAt), now, now)
	if err != nil {
		return domain.ScheduledJob{}, err
	}
	return s.getScheduledJob(ctx, id)
}

func (s *Store) getScheduledJob(ctx context.Context, id string) (domain.ScheduledJob, error) {
	r, err := s.p().must(ctx, "scheduled job", `SELECT id,name,description,action_type,interval_minutes,enabled,notify,next_run_at,last_run_at,created_at,updated_at FROM scheduled_jobs WHERE id=?`, id)
	if err != nil {
		return domain.ScheduledJob{}, err
	}
	return scheduledJobFrom(r), nil
}

func (s *Store) ToggleScheduledJob(ctx context.Context, id string) (domain.ScheduledJob, error) {
	changed, err := s.p().changes(ctx, `UPDATE scheduled_jobs SET enabled=1-enabled,updated_at=? WHERE id=?`, nowText(), id)
	if err != nil {
		return domain.ScheduledJob{}, err
	}
	if changed == 0 {
		return domain.ScheduledJob{}, fmt.Errorf("%w: scheduled job %q", domain.ErrNotFound, id)
	}
	return s.getScheduledJob(ctx, id)
}

func (s *Store) ProcessDueJobs(ctx context.Context, now time.Time, notificationsEnabled bool) (int, error) {
	if now.IsZero() {
		now = domain.Now()
	}
	processed := 0
	err := s.run(ctx, func(p provider) error {
		rows, err := p.all(ctx, `SELECT id,name,description,action_type,interval_minutes,notify FROM scheduled_jobs WHERE enabled=1 AND next_run_at<=? ORDER BY next_run_at`, timeText(now))
		if err != nil {
			return err
		}
		for _, r := range rows {
			id, name, description, action := str(r, "id"), str(r, "name"), str(r, "description"), str(r, "action_type")
			message := description
			switch action {
			case "daily-plan":
				message = "Time to plan today's work. Review pinned items and open Issues."
			case "weekly-review":
				message = "Time for a weekly review. Check activity, completed work and next priorities."
			case "blocked-issues":
				message = "Review the graph for Issues waiting on unfinished prerequisites."
			case "open-issues":
				message = "Review your open and in-progress Issues and choose the next small step."
			default:
				if strings.TrimSpace(message) == "" {
					message = "Your scheduled IDIOS reminder is due."
				}
			}
			if notificationsEnabled && boolRow(r, "notify") {
				if _, err := p.changes(ctx, `INSERT INTO notifications(id,title,body,level,created_at) VALUES(?,?,?,?,?)`, domain.NewID(), name, message, "info", timeText(now)); err != nil {
					return err
				}
			}
			if _, err := p.changes(ctx, `INSERT INTO job_runs(id,job_id,status,message,created_at) VALUES(?,?,?,?,?)`, domain.NewID(), id, "completed", message, timeText(now)); err != nil {
				return err
			}
			interval := max(1, int(i64(r["interval_minutes"])))
			next := now.Add(time.Duration(interval) * time.Minute)
			if _, err := p.changes(ctx, `UPDATE scheduled_jobs SET last_run_at=?,next_run_at=?,updated_at=? WHERE id=?`, timeText(now), timeText(next), timeText(now), id); err != nil {
				return err
			}
			processed++
		}
		return nil
	})
	return processed, err
}

func notificationFrom(r Row) domain.Notification {
	return domain.Notification{ID: str(r, "id"), Title: str(r, "title"), Body: str(r, "body"), Level: str(r, "level"), ReadAt: ptrTime(r["read_at"]), CreatedAt: parseTime(r["created_at"])}
}

func (s *Store) ListNotifications(ctx context.Context, limit int) ([]domain.Notification, error) {
	if limit < 1 || limit > 500 {
		limit = 50
	}
	rows, err := s.p().all(ctx, `SELECT id,title,body,level,read_at,created_at FROM notifications ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	return collect(rows, notificationFrom), nil
}

func (s *Store) MarkNotificationsRead(ctx context.Context) error {
	_, err := s.p().changes(ctx, `UPDATE notifications SET read_at=? WHERE read_at IS NULL`, nowText())
	return err
}

func (s *Store) SetAppSetting(ctx context.Context, key, value string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("%w: app setting key is required", domain.ErrInvalid)
	}
	_, err := s.p().changes(ctx, `INSERT INTO app_settings(key,value,updated_at) VALUES(?,?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value,updated_at=excluded.updated_at`, key, value, nowText())
	return err
}

func (s *Store) GetAppSetting(ctx context.Context, key string) (string, error) {
	r, ok, err := s.p().row(ctx, `SELECT value FROM app_settings WHERE key=?`, key)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", nil
	}
	return str(r, "value"), nil
}
