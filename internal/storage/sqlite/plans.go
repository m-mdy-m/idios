package sqlite

import (
	"context"
	"fmt"
	"strings"
	"time"

	"idios/internal/domain"
)

const planColumns = `p.id,p.title,p.description,p.plan_date,p.linked_entity_type,p.linked_entity_id,p.created_at,p.updated_at,
CASE p.linked_entity_type WHEN 'issue' THEN COALESCE(lr.name||'#'||li.number||' · '||li.title,'') WHEN 'repository' THEN COALESCE(lr.name,'') ELSE '' END AS linked_entity_label`
const planFromSQL = ` FROM plans p
LEFT JOIN issues li ON p.linked_entity_type='issue' AND li.id=p.linked_entity_id
LEFT JOIN repositories lr ON (p.linked_entity_type='repository' AND lr.id=p.linked_entity_id) OR (p.linked_entity_type='issue' AND lr.id=li.repository_id)`

func planFrom(r Row) domain.Plan {
	return domain.Plan{ID: str(r, "id"), Title: str(r, "title"), Description: str(r, "description"), PlanDate: str(r, "plan_date"), LinkedEntityType: str(r, "linked_entity_type"), LinkedEntityID: str(r, "linked_entity_id"), LinkedEntityLabel: str(r, "linked_entity_label"), CreatedAt: parseTime(r["created_at"]), UpdatedAt: parseTime(r["updated_at"]), Tasks: []domain.PlanTask{}}
}

func planTaskFrom(r Row) domain.PlanTask {
	return domain.PlanTask{ID: str(r, "id"), PlanID: str(r, "plan_id"), Title: str(r, "title"), Position: int(i64(r["position"])), Done: boolRow(r, "done"), CreatedAt: parseTime(r["created_at"]), UpdatedAt: parseTime(r["updated_at"]), CompletedAt: ptrTime(r["completed_at"])}
}

func (s *Store) ListPlans(ctx context.Context, from, to string) ([]domain.Plan, error) {
	query := `SELECT ` + planColumns + planFromSQL + ` WHERE 1=1`
	args := []any{}
	if from != "" {
		query += ` AND p.plan_date>=?`
		args = append(args, from)
	}
	if to != "" {
		query += ` AND p.plan_date<=?`
		args = append(args, to)
	}
	query += ` ORDER BY p.plan_date,p.created_at,p.id`
	rows, err := s.p().all(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	plans := collect(rows, planFrom)
	if len(plans) == 0 {
		return plans, nil
	}
	byID := make(map[string]int, len(plans))
	marks := make([]string, 0, len(plans))
	taskArgs := make([]any, 0, len(plans))
	for i := range plans {
		byID[plans[i].ID] = i
		marks = append(marks, "?")
		taskArgs = append(taskArgs, plans[i].ID)
	}
	taskRows, err := s.p().all(ctx, `SELECT id,plan_id,title,position,done,created_at,updated_at,completed_at FROM plan_tasks WHERE plan_id IN (`+strings.Join(marks, ",")+`) ORDER BY plan_id,position,created_at,id`, taskArgs...)
	if err != nil {
		return nil, err
	}
	for _, row := range taskRows {
		if i, ok := byID[str(row, "plan_id")]; ok {
			plans[i].Tasks = append(plans[i].Tasks, planTaskFrom(row))
		}
	}
	return plans, nil
}

func (s *Store) getPlan(ctx context.Context, id string) (domain.Plan, error) {
	row, ok, err := s.p().row(ctx, `SELECT `+planColumns+planFromSQL+` WHERE p.id=?`, id)
	if err != nil {
		return domain.Plan{}, err
	}
	if !ok {
		return domain.Plan{}, fmt.Errorf("%w: plan %q", domain.ErrNotFound, id)
	}
	plan := planFrom(row)
	tasks, err := s.p().all(ctx, `SELECT id,plan_id,title,position,done,created_at,updated_at,completed_at FROM plan_tasks WHERE plan_id=? ORDER BY position,created_at,id`, id)
	if err != nil {
		return domain.Plan{}, err
	}
	plan.Tasks = collect(tasks, planTaskFrom)
	return plan, nil
}

func resolvePlanLink(ctx context.Context, p provider, kind, ref string) (string, error) {
	kind, ref = strings.TrimSpace(kind), strings.TrimSpace(ref)
	if kind == "" {
		if ref != "" {
			return "", fmt.Errorf("%w: linked entity type is required when an entity is selected", domain.ErrInvalid)
		}
		return "", nil
	}
	switch kind {
	case "repository":
		row, ok, err := p.row(ctx, `SELECT id FROM repositories WHERE id=? OR name=? COLLATE NOCASE`, ref, ref)
		if err != nil {
			return "", err
		}
		if !ok {
			return "", fmt.Errorf("%w: linked repository %q", domain.ErrNotFound, ref)
		}
		return str(row, "id"), nil
	case "issue":
		row, ok, err := p.row(ctx, `SELECT i.id FROM issues i JOIN repositories r ON r.id=i.repository_id WHERE i.id=? OR (r.name||'#'||i.number)=? COLLATE NOCASE`, ref, ref)
		if err != nil {
			return "", err
		}
		if !ok {
			return "", fmt.Errorf("%w: linked issue %q", domain.ErrNotFound, ref)
		}
		return str(row, "id"), nil
	default:
		return "", fmt.Errorf("%w: linked entity type must be issue or repository", domain.ErrInvalid)
	}
}

func (s *Store) CreatePlan(ctx context.Context, plan domain.Plan, titles []string) (domain.Plan, error) {
	plan.ID = domain.NewID()
	now := nowText()
	plan.Title = strings.TrimSpace(plan.Title)
	err := s.run(ctx, func(p provider) error {
		linkedID, err := resolvePlanLink(ctx, p, plan.LinkedEntityType, plan.LinkedEntityID)
		if err != nil {
			return err
		}
		plan.LinkedEntityID = linkedID
		if _, err := p.changes(ctx, `INSERT INTO plans(id,title,description,plan_date,linked_entity_type,linked_entity_id,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)`, plan.ID, plan.Title, strings.TrimSpace(plan.Description), plan.PlanDate, plan.LinkedEntityType, linkedID, now, now); err != nil {
			return err
		}
		for position, title := range titles {
			title = strings.TrimSpace(title)
			if title == "" {
				continue
			}
			if _, err := p.changes(ctx, `INSERT INTO plan_tasks(id,plan_id,title,position,done,created_at,updated_at) VALUES(?,?,?,?,0,?,?)`, domain.NewID(), plan.ID, title, position, now, now); err != nil {
				return err
			}
		}
		return activity(ctx, p, "plan", plan.ID, "plan.created", marshalPayload(map[string]any{"title": plan.Title, "date": plan.PlanDate, "linked_entity_type": plan.LinkedEntityType, "linked_entity_id": linkedID, "task_count": len(titles)}))
	})
	if err != nil {
		return domain.Plan{}, err
	}
	return s.getPlan(ctx, plan.ID)
}

func (s *Store) UpdatePlan(ctx context.Context, plan domain.Plan) (domain.Plan, error) {
	plan.Title = strings.TrimSpace(plan.Title)
	now := nowText()
	err := s.run(ctx, func(p provider) error {
		linkedID, err := resolvePlanLink(ctx, p, plan.LinkedEntityType, plan.LinkedEntityID)
		if err != nil {
			return err
		}
		changed, err := p.changes(ctx, `UPDATE plans SET title=?,description=?,plan_date=?,linked_entity_type=?,linked_entity_id=?,updated_at=? WHERE id=?`, plan.Title, strings.TrimSpace(plan.Description), plan.PlanDate, plan.LinkedEntityType, linkedID, now, plan.ID)
		if err != nil {
			return err
		}
		if changed == 0 {
			return fmt.Errorf("%w: plan %q", domain.ErrNotFound, plan.ID)
		}
		return activity(ctx, p, "plan", plan.ID, "plan.updated", marshalPayload(map[string]any{"title": plan.Title, "date": plan.PlanDate}))
	})
	if err != nil {
		return domain.Plan{}, err
	}
	return s.getPlan(ctx, plan.ID)
}

func (s *Store) DeletePlan(ctx context.Context, id string) error {
	return s.run(ctx, func(p provider) error {
		row, ok, err := p.row(ctx, `SELECT title,plan_date FROM plans WHERE id=?`, id)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("%w: plan %q", domain.ErrNotFound, id)
		}
		if _, err := p.changes(ctx, `DELETE FROM plans WHERE id=?`, id); err != nil {
			return err
		}
		return activity(ctx, p, "plan", id, "plan.deleted", marshalPayload(map[string]string{"title": str(row, "title"), "date": str(row, "plan_date")}))
	})
}

func (s *Store) AddPlanTask(ctx context.Context, planID, title string) (domain.PlanTask, error) {
	task := domain.PlanTask{ID: domain.NewID(), PlanID: planID, Title: strings.TrimSpace(title)}
	now := nowText()
	err := s.run(ctx, func(p provider) error {
		if _, err := p.must(ctx, "plan "+planID, `SELECT id FROM plans WHERE id=?`, planID); err != nil {
			return err
		}
		rows, err := p.all(ctx, `SELECT COALESCE(MAX(position),-1)+1 AS next_position FROM plan_tasks WHERE plan_id=?`, planID)
		if err != nil {
			return err
		}
		if len(rows) > 0 {
			task.Position = int(i64(rows[0]["next_position"]))
		}
		if _, err := p.changes(ctx, `INSERT INTO plan_tasks(id,plan_id,title,position,done,created_at,updated_at) VALUES(?,?,?,?,0,?,?)`, task.ID, planID, task.Title, task.Position, now, now); err != nil {
			return err
		}
		if _, err := p.changes(ctx, `UPDATE plans SET updated_at=? WHERE id=?`, now, planID); err != nil {
			return err
		}
		return activity(ctx, p, "plan", planID, "plan.task_added", marshalPayload(map[string]string{"task_id": task.ID, "title": task.Title}))
	})
	if err != nil {
		return domain.PlanTask{}, err
	}
	row, _, err := s.p().row(ctx, `SELECT id,plan_id,title,position,done,created_at,updated_at,completed_at FROM plan_tasks WHERE id=?`, task.ID)
	if err != nil {
		return domain.PlanTask{}, err
	}
	return planTaskFrom(row), nil
}

func (s *Store) SetPlanTaskDone(ctx context.Context, planID, taskID string, done bool) (domain.PlanTask, error) {
	err := s.run(ctx, func(p provider) error {
		row, ok, err := p.row(ctx, `SELECT title,done FROM plan_tasks WHERE id=? AND plan_id=?`, taskID, planID)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("%w: plan task %q", domain.ErrNotFound, taskID)
		}
		if boolRow(row, "done") == done {
			return nil
		}
		var completed any
		if done {
			completed = nowText()
		}
		now := nowText()
		if _, err := p.changes(ctx, `UPDATE plan_tasks SET done=?,completed_at=?,updated_at=? WHERE id=? AND plan_id=?`, boolInt(done), completed, now, taskID, planID); err != nil {
			return err
		}
		if _, err := p.changes(ctx, `UPDATE plans SET updated_at=? WHERE id=?`, now, planID); err != nil {
			return err
		}
		action := "plan.task.uncompleted"
		if done {
			action = "plan.task.completed"
		}
		return activity(ctx, p, "plan", planID, action, marshalPayload(map[string]string{"task_id": taskID, "title": str(row, "title")}))
	})
	if err != nil {
		return domain.PlanTask{}, err
	}
	row, _, err := s.p().row(ctx, `SELECT id,plan_id,title,position,done,created_at,updated_at,completed_at FROM plan_tasks WHERE id=? AND plan_id=?`, taskID, planID)
	if err != nil {
		return domain.PlanTask{}, err
	}
	return planTaskFrom(row), nil
}

func (s *Store) CreatePlanReminderNotification(ctx context.Context, date, period, title, body string, at time.Time) (bool, error) {
	if at.IsZero() {
		at = time.Now()
	}
	at = at.Local()
	deliveredAt := timeText(at)
	sent := false
	err := s.run(ctx, func(p provider) error {
		if _, ok, err := p.row(ctx, `SELECT id FROM plans WHERE plan_date=? LIMIT 1`, date); err != nil {
			return err
		} else if !ok {
			return nil
		}
		changed, err := p.changes(ctx, `INSERT OR IGNORE INTO plan_reminder_deliveries(plan_date,period,delivered_at) VALUES(?,?,?)`, date, period, deliveredAt)
		if err != nil {
			return err
		}
		if changed == 0 {
			return nil
		}
		if _, err := p.changes(ctx, `INSERT INTO notifications(id,title,body,level,created_at,related_entity_type,related_entity_id) VALUES(?,?,?,?,?,?,?)`, domain.NewID(), title, body, "info", deliveredAt, "plan-date", date); err != nil {
			return err
		}
		if err := activity(ctx, p, "plan-date", date, "plan.reminder", marshalPayload(map[string]string{"period": period, "title": title})); err != nil {
			return err
		}
		sent = true
		return nil
	})
	return sent, err
}
