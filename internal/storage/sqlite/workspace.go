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

func noteLabelFrom(r Row) domain.NoteLabel {
	return domain.NoteLabel{ID: str(r, "id"), Name: str(r, "name"), Color: str(r, "color"), CreatedAt: parseTime(r["created_at"])}
}

func (s *Store) ListNotes(ctx context.Context) ([]domain.Note, error) {
	rows, err := s.p().all(ctx, noteSelect+` ORDER BY n.updated_at DESC,n.created_at DESC`)
	if err != nil {
		return nil, err
	}
	notes := collect(rows, noteFrom)
	if err := s.loadNoteLabels(ctx, notes); err != nil {
		return nil, err
	}
	return notes, nil
}

func (s *Store) loadNoteLabels(ctx context.Context, notes []domain.Note) error {
	if len(notes) == 0 {
		return nil
	}
	byID := make(map[string][]domain.NoteLabel, len(notes))
	// Keep each IN query comfortably under SQLite builds whose bind-variable limit is 999.
	const batchSize = 400
	for start := 0; start < len(notes); start += batchSize {
		end := min(start+batchSize, len(notes))
		args := make([]any, 0, end-start)
		marks := make([]string, 0, end-start)
		for _, note := range notes[start:end] {
			marks = append(marks, "?")
			args = append(args, note.ID)
		}
		rows, err := s.p().all(ctx, `SELECT a.note_id,l.id,l.name,l.color,l.created_at
			FROM note_label_assignments a JOIN note_labels l ON l.id=a.label_id
			WHERE a.note_id IN (`+strings.Join(marks, ",")+`) ORDER BY l.name COLLATE NOCASE`, args...)
		if err != nil {
			return err
		}
		for _, row := range rows {
			byID[str(row, "note_id")] = append(byID[str(row, "note_id")], noteLabelFrom(row))
		}
	}
	for i := range notes {
		notes[i].Labels = byID[notes[i].ID]
	}
	return nil
}

func (s *Store) ListNoteLabels(ctx context.Context) ([]domain.NoteLabel, error) {
	rows, err := s.p().all(ctx, `SELECT id,name,color,created_at FROM note_labels ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	return collect(rows, noteLabelFrom), nil
}

func (s *Store) SetNoteLabelColor(ctx context.Context, name, color string) error {
	name = strings.TrimSpace(name)
	color = strings.TrimSpace(color)
	if name == "" {
		return fmt.Errorf("%w: note label name is required", domain.ErrInvalid)
	}
	if len(color) != 7 || color[0] != '#' {
		return fmt.Errorf("%w: label color must be a six-digit hex color", domain.ErrInvalid)
	}
	for _, r := range color[1:] {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return fmt.Errorf("%w: label color must be a six-digit hex color", domain.ErrInvalid)
		}
	}
	changed, err := s.p().changes(ctx, `UPDATE note_labels SET color=? WHERE name=? COLLATE NOCASE`, strings.ToLower(color), name)
	if err != nil {
		return err
	}
	if changed == 0 {
		return fmt.Errorf("%w: note label %q", domain.ErrNotFound, name)
	}
	return nil
}

func (s *Store) CreateNote(ctx context.Context, title, content string) (domain.Note, error) {
	return s.CreateNoteWithLabels(ctx, title, content, nil)
}

func (s *Store) CreateNoteWithLabels(ctx context.Context, title, content string, labels []string) (domain.Note, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return domain.Note{}, fmt.Errorf("%w: note title is required", domain.ErrInvalid)
	}
	id, now := domain.NewID(), nowText()
	err := s.run(ctx, func(p provider) error {
		if _, err := p.changes(ctx, `INSERT INTO notes(id,title,content,created_at,updated_at) VALUES(?,?,?,?,?)`, id, title, strings.TrimSpace(content), now, now); err != nil {
			return err
		}
		if err := setNoteLabels(ctx, p, id, labels); err != nil {
			return err
		}
		return activity(ctx, p, "note", id, "note.created", marshalPayload(map[string]any{"title": title, "labels": labels}))
	})
	if err != nil {
		return domain.Note{}, err
	}
	return s.getNote(ctx, id)
}

func (s *Store) UpdateNote(ctx context.Context, id, title, content string) (domain.Note, error) {
	return s.updateNote(ctx, id, title, content, nil, false)
}

func (s *Store) UpdateNoteWithLabels(ctx context.Context, id, title, content string, labels []string) (domain.Note, error) {
	return s.updateNote(ctx, id, title, content, labels, true)
}

func (s *Store) updateNote(ctx context.Context, id, title, content string, labels []string, replaceLabels bool) (domain.Note, error) {
	title = strings.TrimSpace(title)
	if strings.TrimSpace(id) == "" || title == "" {
		return domain.Note{}, fmt.Errorf("%w: note ID and title are required", domain.ErrInvalid)
	}
	err := s.run(ctx, func(p provider) error {
		changed, err := p.changes(ctx, `UPDATE notes SET title=?,content=?,updated_at=? WHERE id=?`, title, strings.TrimSpace(content), nowText(), id)
		if err != nil {
			return err
		}
		if changed == 0 {
			return fmt.Errorf("%w: note %q", domain.ErrNotFound, id)
		}
		if replaceLabels {
			if err := setNoteLabels(ctx, p, id, labels); err != nil {
				return err
			}
		}
		return activity(ctx, p, "note", id, "note.updated", marshalPayload(map[string]any{"title": title, "labels": labels}))
	})
	if err != nil {
		return domain.Note{}, err
	}
	return s.getNote(ctx, id)
}

func normalizeNoteLabels(labels []string) []string {
	out := make([]string, 0, len(labels))
	seen := map[string]bool{}
	for _, label := range labels {
		name := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(label), "#"))
		key := strings.ToLower(name)
		if name == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, name)
	}
	return out
}

func setNoteLabels(ctx context.Context, p provider, noteID string, labels []string) error {
	if _, err := p.changes(ctx, `DELETE FROM note_label_assignments WHERE note_id=?`, noteID); err != nil {
		return err
	}
	for _, name := range normalizeNoteLabels(labels) {
		labelID := domain.NewID()
		if _, err := p.changes(ctx, `INSERT INTO note_labels(id,name,color,created_at) VALUES(?,?,?,?) ON CONFLICT(name) DO NOTHING`, labelID, name, noteLabelColor(name), nowText()); err != nil {
			return err
		}
		row, err := p.must(ctx, "note label", `SELECT id FROM note_labels WHERE name=? COLLATE NOCASE`, name)
		if err != nil {
			return err
		}
		if _, err := p.changes(ctx, `INSERT INTO note_label_assignments(note_id,label_id,created_at) VALUES(?,?,?)`, noteID, str(row, "id"), nowText()); err != nil {
			return err
		}
	}
	return cleanupUnusedNoteLabels(ctx, p)
}

func cleanupUnusedNoteLabels(ctx context.Context, p provider) error {
	_, err := p.changes(ctx, `DELETE FROM note_labels WHERE NOT EXISTS (
		SELECT 1 FROM note_label_assignments a WHERE a.label_id=note_labels.id
	)`)
	return err
}

func noteLabelColor(name string) string {
	// Stable, readable GitHub-like colors make a note label recognizable across restarts.
	colors := []string{"#1f6feb", "#8957e5", "#238636", "#bc4c00", "#bf3989", "#0f8b8d", "#9a6700", "#6e7781"}
	var hash uint32 = 2166136261
	for _, r := range strings.ToLower(name) {
		hash ^= uint32(r)
		hash *= 16777619
	}
	return colors[int(hash)%len(colors)]
}

func (s *Store) DeleteNote(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("%w: note ID is required", domain.ErrInvalid)
	}
	return s.run(ctx, func(p provider) error {
		// Remove polymorphic graph links and detach any other note that points to this one.
		// Notes are intentionally retained by their own delete operation only when they are
		// not the selected target; a dangling note-to-note edge is never left behind.
		if _, err := p.changes(ctx, `UPDATE notes SET linked_entity_type='',linked_entity_id='',graph_visible=0,updated_at=? WHERE linked_entity_type='note' AND linked_entity_id=? AND id<>?`, nowText(), id, id); err != nil {
			return err
		}
		if _, err := p.changes(ctx, `DELETE FROM relations WHERE (source_type='note' AND source_id=?) OR (target_type='note' AND target_id=?)`, id, id); err != nil {
			return err
		}
		if _, err := p.changes(ctx, `DELETE FROM workspace_pins WHERE entity_type='note' AND entity_id=?`, id); err != nil {
			return err
		}
		changed, err := p.changes(ctx, `DELETE FROM notes WHERE id=?`, id)
		if err != nil {
			return err
		}
		if changed == 0 {
			return fmt.Errorf("%w: note %q", domain.ErrNotFound, id)
		}
		if err := cleanupUnusedNoteLabels(ctx, p); err != nil {
			return err
		}
		return activity(ctx, p, "note", id, "note.deleted", "")
	})
}

func (s *Store) getNote(ctx context.Context, id string) (domain.Note, error) {
	r, err := s.p().must(ctx, fmt.Sprintf("note %q", id), noteSelect+` WHERE n.id=?`, id)
	if err != nil {
		return domain.Note{}, err
	}
	notes := []domain.Note{noteFrom(r)}
	if err := s.loadNoteLabels(ctx, notes); err != nil {
		return domain.Note{}, err
	}
	return notes[0], nil
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
	return domain.Quote{
		ID: str(r, "id"), Text: str(r, "text"), Author: str(r, "author"),
		Source: str(r, "source"), SourceURL: str(r, "source_url"), Category: str(r, "category"),
		CreatedAt: parseTime(r["created_at"]),
	}
}

func (s *Store) ListQuotes(ctx context.Context) ([]domain.Quote, error) {
	rows, err := s.p().all(ctx, `SELECT id,text,author,source,source_url,category,created_at FROM quotes ORDER BY created_at,id`)
	if err != nil {
		return nil, err
	}
	return collect(rows, quoteFrom), nil
}

func (s *Store) CreateQuote(ctx context.Context, text, author string) (domain.Quote, error) {
	return s.CreateQuoteWithSource(ctx, domain.Quote{Text: text, Author: author})
}

func (s *Store) CreateQuoteWithSource(ctx context.Context, quote domain.Quote) (domain.Quote, error) {
	quote.Text = strings.TrimSpace(quote.Text)
	quote.Author = strings.TrimSpace(quote.Author)
	quote.Source = strings.TrimSpace(quote.Source)
	quote.SourceURL = strings.TrimSpace(quote.SourceURL)
	quote.Category = strings.TrimSpace(quote.Category)
	if quote.Text == "" {
		return domain.Quote{}, fmt.Errorf("%w: quote text is required", domain.ErrInvalid)
	}
	quote.ID = domain.NewID()
	quote.CreatedAt = time.Now().UTC()
	if _, err := s.p().changes(ctx, `INSERT INTO quotes(id,text,author,source,source_url,category,created_at) VALUES(?,?,?,?,?,?,?)`, quote.ID, quote.Text, quote.Author, quote.Source, quote.SourceURL, quote.Category, timeText(quote.CreatedAt)); err != nil {
		return domain.Quote{}, err
	}
	_ = s.RecordActivity(ctx, "quote", quote.ID, "quote.created", "")
	r, err := s.p().must(ctx, "quote", `SELECT id,text,author,source,source_url,category,created_at FROM quotes WHERE id=?`, quote.ID)
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
		rows, err := p.all(ctx, `SELECT id,text,author,source,source_url,category,created_at FROM quotes ORDER BY created_at,id`)
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
	r, err := s.p().must(ctx, "workspace profile", `SELECT username,full_name,focus,topics,website,bio,readme,updated_at FROM workspace_profile WHERE singleton_id='default'`)
	if err != nil {
		return domain.WorkspaceProfile{}, err
	}
	return domain.WorkspaceProfile{Username: str(r, "username"), FullName: str(r, "full_name"), Focus: str(r, "focus"), Topics: str(r, "topics"), Website: str(r, "website"), Bio: str(r, "bio"), Readme: str(r, "readme"), UpdatedAt: parseTime(r["updated_at"])}, nil
}

func (s *Store) UpdateWorkspaceProfile(ctx context.Context, profile domain.WorkspaceProfile) (domain.WorkspaceProfile, error) {
	profile.Username, profile.FullName = strings.TrimSpace(profile.Username), strings.TrimSpace(profile.FullName)
	if profile.Username == "" || profile.FullName == "" {
		return domain.WorkspaceProfile{}, fmt.Errorf("%w: username and display name are required", domain.ErrInvalid)
	}
	_, err := s.p().changes(ctx, `INSERT INTO workspace_profile(singleton_id,username,full_name,focus,topics,website,bio,readme,updated_at) VALUES('default',?,?,?,?,?,?,?,?) ON CONFLICT(singleton_id) DO UPDATE SET username=excluded.username,full_name=excluded.full_name,focus=excluded.focus,topics=excluded.topics,website=excluded.website,bio=excluded.bio,readme=excluded.readme,updated_at=excluded.updated_at`, profile.Username, profile.FullName, strings.TrimSpace(profile.Focus), strings.TrimSpace(profile.Topics), strings.TrimSpace(profile.Website), strings.TrimSpace(profile.Bio), strings.TrimSpace(profile.Readme), nowText())
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

const scheduledJobColumns = `id,name,description,action_type,schedule_type,interval_minutes,schedule_time,schedule_weekday,schedule_day,enabled,notify,next_run_at,last_run_at,completed_at,branch_id,created_at,updated_at`

func scheduledJobFrom(r Row) domain.ScheduledJob {
	scheduleType := str(r, "schedule_type")
	intervalMinutes := int(i64(r["interval_minutes"]))
	if scheduleType != "" && scheduleType != domain.ScheduleInterval {
		intervalMinutes = 0 // Keep the legacy SQLite column valid without exposing its sentinel value.
	}
	return domain.ScheduledJob{
		ID: str(r, "id"), Name: str(r, "name"), Description: str(r, "description"), ActionType: str(r, "action_type"),
		ScheduleType: scheduleType, IntervalMinutes: intervalMinutes, BranchID: ptrString(r["branch_id"]),
		ScheduleTime: str(r, "schedule_time"), ScheduleWeekday: int(i64(r["schedule_weekday"])), ScheduleDay: int(i64(r["schedule_day"])),
		Enabled: boolRow(r, "enabled"), Notify: boolRow(r, "notify"), NextRunAt: ptrTime(r["next_run_at"]),
		LastRunAt: ptrTime(r["last_run_at"]), CompletedAt: ptrTime(r["completed_at"]),
		CreatedAt: parseTime(r["created_at"]), UpdatedAt: parseTime(r["updated_at"]),
	}
}

func (s *Store) ListScheduledJobs(ctx context.Context) ([]domain.ScheduledJob, error) {
	rows, err := s.p().all(ctx, `SELECT `+scheduledJobColumns+` FROM scheduled_jobs ORDER BY completed_at IS NOT NULL, enabled DESC,next_run_at,name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	return collect(rows, scheduledJobFrom), nil
}

func (s *Store) CreateScheduledJob(ctx context.Context, job domain.ScheduledJob) (domain.ScheduledJob, error) {
	job, err := domain.PrepareScheduledJob(job, domain.Now().Local())
	if err != nil {
		return domain.ScheduledJob{}, err
	}
	if job.NextRunAt == nil {
		return domain.ScheduledJob{}, fmt.Errorf("%w: next run time could not be calculated", domain.ErrInvalid)
	}
	id, now := domain.NewID(), nowText()
	err = s.run(ctx, func(p provider) error {
		_, err := p.changes(ctx, `INSERT INTO scheduled_jobs(id,name,description,action_type,interval_minutes,enabled,notify,next_run_at,created_at,updated_at,schedule_type,schedule_time,schedule_weekday,schedule_day,completed_at,branch_id)
			VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,NULL,?)`, id, job.Name, job.Description, job.ActionType,
			max(1, job.IntervalMinutes), boolInt(job.Enabled), boolInt(job.Notify), timeText(*job.NextRunAt), now, now,
			job.ScheduleType, job.ScheduleTime, job.ScheduleWeekday, job.ScheduleDay, job.BranchID)
		if err != nil {
			return err
		}
		return activity(ctx, p, "scheduled_job", id, "scheduled_job.created", marshalPayload(map[string]any{
			"name": job.Name, "schedule_type": job.ScheduleType, "interval_minutes": job.IntervalMinutes,
			"schedule_time": job.ScheduleTime, "schedule_weekday": job.ScheduleWeekday, "schedule_day": job.ScheduleDay,
		}))
	})
	if err != nil {
		return domain.ScheduledJob{}, err
	}
	return s.getScheduledJob(ctx, id)
}

func (s *Store) UpdateScheduledJob(ctx context.Context, job domain.ScheduledJob) (domain.ScheduledJob, error) {
	if strings.TrimSpace(job.ID) == "" {
		return domain.ScheduledJob{}, fmt.Errorf("%w: job ID is required", domain.ErrInvalid)
	}
	job, err := domain.PrepareScheduledJob(job, domain.Now().Local())
	if err != nil {
		return domain.ScheduledJob{}, err
	}
	if job.NextRunAt == nil {
		return domain.ScheduledJob{}, fmt.Errorf("%w: next run time could not be calculated", domain.ErrInvalid)
	}
	now := nowText()
	err = s.run(ctx, func(p provider) error {
		changed, err := p.changes(ctx, `UPDATE scheduled_jobs SET name=?,description=?,action_type=?,schedule_type=?,interval_minutes=?,schedule_time=?,schedule_weekday=?,schedule_day=?,enabled=?,notify=?,next_run_at=?,completed_at=NULL,branch_id=?,updated_at=? WHERE id=?`,
			job.Name, job.Description, job.ActionType, job.ScheduleType, max(1, job.IntervalMinutes), job.ScheduleTime, job.ScheduleWeekday, job.ScheduleDay,
			boolInt(job.Enabled), boolInt(job.Notify), timeText(*job.NextRunAt), job.BranchID, now, job.ID)
		if err != nil {
			return err
		}
		if changed == 0 {
			return fmt.Errorf("%w: scheduled job %q", domain.ErrNotFound, job.ID)
		}
		return activity(ctx, p, "scheduled_job", job.ID, "scheduled_job.updated", marshalPayload(map[string]any{
			"name": job.Name, "schedule_type": job.ScheduleType, "interval_minutes": job.IntervalMinutes,
			"schedule_time": job.ScheduleTime, "schedule_weekday": job.ScheduleWeekday, "schedule_day": job.ScheduleDay,
		}))
	})
	if err != nil {
		return domain.ScheduledJob{}, err
	}
	return s.getScheduledJob(ctx, job.ID)
}

func (s *Store) DeleteScheduledJob(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("%w: job ID is required", domain.ErrInvalid)
	}
	return s.run(ctx, func(p provider) error {
		changed, err := p.changes(ctx, `DELETE FROM scheduled_jobs WHERE id=?`, id)
		if err != nil {
			return err
		}
		if changed == 0 {
			return fmt.Errorf("%w: scheduled job %q", domain.ErrNotFound, id)
		}
		return activity(ctx, p, "scheduled_job", id, "scheduled_job.deleted", "")
	})
}

func (s *Store) getScheduledJob(ctx context.Context, id string) (domain.ScheduledJob, error) {
	r, err := s.p().must(ctx, "scheduled job", `SELECT `+scheduledJobColumns+` FROM scheduled_jobs WHERE id=?`, id)
	if err != nil {
		return domain.ScheduledJob{}, err
	}
	return scheduledJobFrom(r), nil
}

func (s *Store) ToggleScheduledJob(ctx context.Context, id string) (domain.ScheduledJob, error) {
	if strings.TrimSpace(id) == "" {
		return domain.ScheduledJob{}, fmt.Errorf("%w: job ID is required", domain.ErrInvalid)
	}
	now := domain.Now().Local()
	err := s.run(ctx, func(p provider) error {
		r, err := p.must(ctx, "scheduled job "+id, `SELECT `+scheduledJobColumns+` FROM scheduled_jobs WHERE id=?`, id)
		if err != nil {
			return err
		}
		job := scheduledJobFrom(r)
		if !job.Enabled && job.CompletedAt != nil {
			return fmt.Errorf("%w: this one-time job has completed; edit its date/time to schedule it again", domain.ErrInvalidTransition)
		}
		enabled := !job.Enabled
		if job.NextRunAt == nil || job.NextRunAt.IsZero() {
			if job.ScheduleType == domain.ScheduleOnce {
				return fmt.Errorf("%w: one-time job has no scheduled date/time; edit it to reschedule", domain.ErrInvalid)
			}
			prepared, err := domain.PrepareScheduledJob(job, now)
			if err != nil {
				return err
			}
			job.NextRunAt = prepared.NextRunAt
		} else if enabled && !job.NextRunAt.After(now) && job.ScheduleType != domain.ScheduleOnce {
			next, err := domain.NextScheduledOccurrence(job, now)
			if err != nil {
				return err
			}
			job.NextRunAt = &next
		}
		if job.NextRunAt == nil {
			return fmt.Errorf("%w: next run time could not be calculated", domain.ErrInvalid)
		}
		_, err = p.changes(ctx, `UPDATE scheduled_jobs SET enabled=?,next_run_at=?,updated_at=? WHERE id=?`, boolInt(enabled), timeText(*job.NextRunAt), nowText(), id)
		if err != nil {
			return err
		}
		return activity(ctx, p, "scheduled_job", id, "scheduled_job.toggled", marshalPayload(map[string]bool{"enabled": enabled}))
	})
	if err != nil {
		return domain.ScheduledJob{}, err
	}
	return s.getScheduledJob(ctx, id)
}

func scheduledReminderMessage(job domain.ScheduledJob) string {
	message := job.Description
	switch job.ActionType {
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
	return message
}

func executeScheduledJob(ctx context.Context, p provider, job domain.ScheduledJob, now time.Time, notificationsEnabled bool) error {
	message := scheduledReminderMessage(job)
	if notificationsEnabled && job.Notify {
		if _, err := p.changes(ctx, `INSERT INTO notifications(id,title,body,level,created_at) VALUES(?,?,?,?,?)`, domain.NewID(), job.Name, message, "info", timeText(now)); err != nil {
			return err
		}
	}
	if _, err := p.changes(ctx, `INSERT INTO job_runs(id,job_id,status,message,created_at) VALUES(?,?,?,?,?)`, domain.NewID(), job.ID, "completed", message, timeText(now)); err != nil {
		return err
	}
	if job.ScheduleType == domain.ScheduleOnce {
		_, err := p.changes(ctx, `UPDATE scheduled_jobs SET enabled=0,last_run_at=?,next_run_at=?,completed_at=?,updated_at=? WHERE id=?`, timeText(now), timeText(now), timeText(now), timeText(now), job.ID)
		return err
	}
	next, err := domain.NextScheduledOccurrence(job, now)
	if err != nil {
		return err
	}
	_, err = p.changes(ctx, `UPDATE scheduled_jobs SET last_run_at=?,next_run_at=?,completed_at=NULL,updated_at=? WHERE id=?`, timeText(now), timeText(next), timeText(now), job.ID)
	return err
}

func (s *Store) ProcessDueJobs(ctx context.Context, now time.Time, notificationsEnabled bool) (int, error) {
	if now.IsZero() {
		now = domain.Now()
	}
	now = now.Local()
	processed := 0
	err := s.run(ctx, func(p provider) error {
		rows, err := p.all(ctx, `SELECT `+scheduledJobColumns+` FROM scheduled_jobs WHERE enabled=1 AND completed_at IS NULL AND next_run_at<=? ORDER BY next_run_at,id`, timeText(now))
		if err != nil {
			return err
		}
		for _, row := range rows {
			job := scheduledJobFrom(row)
			if err := executeScheduledJob(ctx, p, job, now, notificationsEnabled); err != nil {
				return err
			}
			processed++
		}
		return nil
	})
	return processed, err
}

func (s *Store) RunScheduledJobNow(ctx context.Context, id string, now time.Time, notificationsEnabled bool) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("%w: job ID is required", domain.ErrInvalid)
	}
	if now.IsZero() {
		now = domain.Now()
	}
	now = now.Local()
	return s.run(ctx, func(p provider) error {
		row, err := p.must(ctx, "scheduled job "+id, `SELECT `+scheduledJobColumns+` FROM scheduled_jobs WHERE id=?`, id)
		if err != nil {
			return err
		}
		job := scheduledJobFrom(row)
		if job.CompletedAt != nil {
			return fmt.Errorf("%w: this one-time job has already completed; edit its date/time to run it again", domain.ErrInvalidTransition)
		}
		return executeScheduledJob(ctx, p, job, now, notificationsEnabled)
	})
}

func notificationFrom(r Row) domain.Notification {
	return domain.Notification{ID: str(r, "id"), Title: str(r, "title"), Body: str(r, "body"), Level: str(r, "level"), RelatedEntityType: str(r, "related_entity_type"), RelatedEntityID: str(r, "related_entity_id"), ReadAt: ptrTime(r["read_at"]), CreatedAt: parseTime(r["created_at"])}
}

func (s *Store) CreateNotification(ctx context.Context, title, body, level string) (domain.Notification, error) {
	title, body, level = strings.TrimSpace(title), strings.TrimSpace(body), strings.TrimSpace(level)
	if title == "" {
		return domain.Notification{}, fmt.Errorf("%w: notification title is required", domain.ErrInvalid)
	}
	if level == "" {
		level = "info"
	}
	n := domain.Notification{ID: domain.NewID(), Title: title, Body: body, Level: level, CreatedAt: time.Now().UTC()}
	if _, err := s.p().changes(ctx, `INSERT INTO notifications(id,title,body,level,created_at) VALUES(?,?,?,?,?)`, n.ID, n.Title, n.Body, n.Level, timeText(n.CreatedAt)); err != nil {
		return domain.Notification{}, err
	}
	_ = s.RecordActivity(ctx, "notification", n.ID, "notification.created", n.Title)
	return n, nil
}

func (s *Store) ListNotifications(ctx context.Context, limit int) ([]domain.Notification, error) {
	if limit < 1 || limit > 500 {
		limit = 50
	}
	rows, err := s.p().all(ctx, `SELECT id,title,body,level,read_at,created_at,related_entity_type,related_entity_id FROM notifications ORDER BY created_at DESC LIMIT ?`, limit)
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
