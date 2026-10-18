package sqlite

import (
	"context"
	"time"

	"idios/internal/domain"
)

// SeedDemo creates one connected, reusable sample workspace. It is intentionally atomic and
// idempotent: running it again never duplicates the demo graph or rewrites existing user data.
func (s *Store) SeedDemo(ctx context.Context) (domain.SeedReport, error) {
	report := domain.SeedReport{Repositories: 3, Issues: 8, Projects: 1, Concepts: 2, Notes: 3, Quotes: 4, Jobs: 3}
	already := false
	err := s.run(ctx, func(p provider) error {
		if _, found, err := p.row(ctx, `SELECT value FROM app_settings WHERE key='seed.demo.v1'`); err != nil {
			return err
		} else if found {
			already = true
			return nil
		}
		now := domain.Now()
		stamp := timeText(now)
		exec := func(q string, args ...any) error { _, err := p.changes(ctx, q, args...); return err }

		// Repositories point at their default branch through a deferred FK, so each pair is
		// created inside this single transaction before the FK check runs at commit time.
		repos := []struct{ id, name, desc, branch string }{
			{"seed-repo-idios", "IDIOS", "Personal workspace engine and terminal experience.", "seed-branch-idios-main"},
			{"seed-repo-cdin", "CDIN", "A small systems project for event-driven architecture.", "seed-branch-cdin-main"},
			{"seed-repo-zinx", "ZINX", "Language design notes and runtime experiments.", "seed-branch-zinx-main"},
		}
		for _, r := range repos {
			if err := exec(`INSERT INTO repositories(id,name,description,default_branch_id,created_at,updated_at) VALUES(?,?,?,?,?,?)`, r.id, r.name, r.desc, r.branch, stamp, stamp); err != nil {
				return err
			}
			if err := exec(`INSERT INTO branches(id,repository_id,name,is_default,created_at,updated_at) VALUES(?,?, 'main',1,?,?)`, r.branch, r.id, stamp, stamp); err != nil {
				return err
			}
		}

		issues := []struct {
			id                                  string
			num                                 int
			repo, title, desc, status, priority string
			parent                              any
		}{
			{"seed-issue-idios-1", 1, "seed-repo-idios", "Move theme tokens into configuration", "Load the active theme from the user config and keep styling independent from page rendering.", "in_progress", "high", nil},
			{"seed-issue-idios-2", 2, "seed-repo-idios", "Build a useful startup intro", "Show a compact wordmark, rotating quote, and obvious first actions.", "open", "normal", nil},
			{"seed-issue-idios-3", 3, "seed-repo-idios", "Improve keyboard navigation", "Make every main destination reachable without a mouse.", "open", "normal", nil},
			{"seed-issue-idios-4", 4, "seed-repo-idios", "Design a keyboard shortcut map", "Document and render the shortcuts users can actually use.", "open", "low", "seed-issue-idios-3"},
			{"seed-issue-idios-5", 5, "seed-repo-idios", "Polish graph empty states", "Explain blockers and relationships without making users read protocol docs.", "open", "high", nil},
			{"seed-issue-cdin-1", 1, "seed-repo-cdin", "Document event-loop contracts", "Write down ownership, scheduling and lifecycle invariants.", "done", "normal", nil},
			{"seed-issue-cdin-2", 2, "seed-repo-cdin", "Split adapter and engine responsibilities", "Keep terminal concerns outside the application core.", "open", "high", nil},
			{"seed-issue-zinx-1", 1, "seed-repo-zinx", "Sketch the module import graph", "Define which packages can depend on which other packages.", "open", "normal", nil},
		}
		for _, i := range issues {
			if err := exec(`INSERT INTO issues(id,number,repository_id,title,description,status,priority,parent_issue_id,created_at,updated_at,closed_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, i.id, i.num, i.repo, i.title, i.desc, i.status, i.priority, i.parent, stamp, stamp, nullableStatusTime(i.status, stamp)); err != nil {
				return err
			}
		}

		// Open and completed PR examples; accepted PRs have a completion record on main.
		if err := exec(`INSERT INTO pull_requests(id,number,repository_id,issue_id,title,description,status,target_branch_id,created_at,updated_at,accepted_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
			"seed-pr-idios-1", 1, "seed-repo-idios", "seed-issue-idios-1", "Configurable theme tokens", "Record of the implementation plan and review notes.", "open", "seed-branch-idios-main", stamp, stamp, nil); err != nil {
			return err
		}
		acceptedAt := timeText(now.Add(-48 * time.Hour))
		if err := exec(`INSERT INTO pull_requests(id,number,repository_id,issue_id,title,description,status,target_branch_id,created_at,updated_at,accepted_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
			"seed-pr-cdin-1", 1, "seed-repo-cdin", "seed-issue-cdin-1", "Document event-loop contracts", "Reviewed design notes and lifecycle invariants.", "accepted", "seed-branch-cdin-main", acceptedAt, acceptedAt, acceptedAt); err != nil {
			return err
		}
		if err := exec(`INSERT INTO completions(id,repository_id,branch_id,issue_id,pull_request_id,summary,sequence,parent_completion_id,created_at) VALUES(?,?,?,?,?,?,1,NULL,?)`,
			"seed-completion-cdin-1", "seed-repo-cdin", "seed-branch-cdin-main", "seed-issue-cdin-1", "seed-pr-cdin-1", "Event loop contract documented and reviewed.", acceptedAt); err != nil {
			return err
		}

		if err := exec(`INSERT INTO issue_dependencies(issue_id,prerequisite_issue_id,created_at) VALUES(?,?,?)`, "seed-issue-idios-2", "seed-issue-cdin-1", stamp); err != nil {
			return err
		}
		if err := exec(`INSERT INTO issue_dependencies(issue_id,prerequisite_issue_id,created_at) VALUES(?,?,?)`, "seed-issue-idios-5", "seed-issue-zinx-1", stamp); err != nil {
			return err
		}

		if err := exec(`INSERT INTO projects(id,name,description,created_at,updated_at) VALUES(?,?,?,?,?)`, "seed-project-workspace", "Workspace experience", "A sample cross-repository project for improving the IDIOS experience.", stamp, stamp); err != nil {
			return err
		}
		for _, issueID := range []string{"seed-issue-idios-1", "seed-issue-idios-2", "seed-issue-cdin-2", "seed-issue-zinx-1"} {
			if err := exec(`INSERT INTO project_issues(project_id,issue_id,created_at) VALUES(?,?,?)`, "seed-project-workspace", issueID, stamp); err != nil {
				return err
			}
		}

		for _, c := range []struct{ id, name, desc string }{
			{"seed-concept-config", "Config-driven UI", "User-owned configuration controls colors and presentation without editing Go source."},
			{"seed-concept-graph", "Knowledge graph", "Typed edges make relationships and prerequisites inspectable."},
		} {
			if err := exec(`INSERT INTO concepts(id,name,description,created_at,updated_at) VALUES(?,?,?,?,?)`, c.id, c.name, c.desc, stamp, stamp); err != nil {
				return err
			}
		}
		for _, rel := range []struct{ st, sid, tt, tid, kind string }{
			{"issue", "seed-issue-idios-1", "concept", "seed-concept-config", "related_to"},
			{"issue", "seed-issue-idios-5", "concept", "seed-concept-graph", "related_to"},
			{"concept", "seed-concept-config", "concept", "seed-concept-graph", "supports"},
		} {
			if err := exec(`INSERT INTO relations(id,source_type,source_id,target_type,target_id,relation_type,created_at) VALUES(?,?,?,?,?,?,?)`, domain.NewID(), rel.st, rel.sid, rel.tt, rel.tid, rel.kind, stamp); err != nil {
				return err
			}
		}

		if err := exec(`INSERT INTO labels(id,name,description,color,scope_type,repository_id,created_at) VALUES(?,?,?,?,?,?,?)`, "seed-label-important", "important", "Needs a little extra attention.", "#e3b341", "global", nil, stamp); err != nil {
			return err
		}
		if err := exec(`INSERT INTO labels(id,name,description,color,scope_type,repository_id,created_at) VALUES(?,?,?,?,?,?,?)`, "seed-label-design", "design", "UX and architecture work.", "#bc8cff", "repository", "seed-repo-idios", stamp); err != nil {
			return err
		}
		for _, pair := range []struct{ i, l string }{{"seed-issue-idios-1", "seed-label-design"}, {"seed-issue-idios-5", "seed-label-important"}, {"seed-issue-zinx-1", "seed-label-important"}} {
			if err := exec(`INSERT INTO issue_labels(issue_id,label_id,created_at) VALUES(?,?,?)`, pair.i, pair.l, stamp); err != nil {
				return err
			}
		}
		if err := exec(`INSERT INTO milestones(id,title,description,scope_type,scope_id,due_date,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)`, "seed-milestone-v1", "First experience pass", "Configurable appearance, intro and user guide.", "repository", "seed-repo-idios", now.Add(14*24*time.Hour).Format("2006-01-02"), stamp, stamp); err != nil {
			return err
		}
		if err := exec(`INSERT INTO issue_milestones(issue_id,milestone_id,created_at) VALUES(?,?,?)`, "seed-issue-idios-1", "seed-milestone-v1", stamp); err != nil {
			return err
		}

		notes := []struct {
			id, title, content, typ, eid string
			visible                      bool
		}{
			{"seed-note-theme", "Theme token idea", "Put all theme colors in config.json. Keep color names semantic (accent, surface, danger) instead of scattering hex codes in pages.", "issue", "seed-issue-idios-1", true},
			{"seed-note-graph", "Graph reading tip", "A parent/sub-issue edge describes structure. A depends_on edge is a blocker. A concept relation is semantic and should not block work.", "concept", "seed-concept-graph", true},
			{"seed-note-home", "Workspace README draft", "This local workspace connects implementation tasks, system design notes and learning projects.", "repository", "seed-repo-idios", false},
		}
		for _, n := range notes {
			if err := exec(`INSERT INTO notes(id,title,content,linked_entity_type,linked_entity_id,graph_visible,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)`, n.id, n.title, n.content, n.typ, n.eid, boolInt(n.visible), stamp, stamp); err != nil {
				return err
			}
		}
		quotes := []struct{ id, text, author string }{
			{"seed-quote-01", "Make the important thing obvious, and the complicated thing explorable.", "IDIOS notes"},
			{"seed-quote-02", "A graph is useful when its edges explain what to do next.", "Workspace principle"},
			{"seed-quote-03", "Small, reviewable steps beat a heroic rewrite with no map.", "Engineering note"},
			{"seed-quote-04", "Good tools make progress visible without turning work into noise.", "IDIOS notes"},
		}
		for _, q := range quotes {
			if err := exec(`INSERT INTO quotes(id,text,author,created_at) VALUES(?,?,?,?)`, q.id, q.text, q.author, stamp); err != nil {
				return err
			}
		}

		jobs := []struct {
			id, name, desc, action string
			interval               int
		}{
			{"seed-job-daily", "Daily planning", "Review your important items and choose a small next step.", "daily-plan", 1440},
			{"seed-job-blockers", "Check blocked issues", "Review the graph for unresolved prerequisites.", "blocked-issues", 360},
			{"seed-job-weekly", "Weekly workspace review", "Review activity, completions and upcoming priorities.", "weekly-review", 10080},
		}
		for _, j := range jobs {
			next := timeText(now.Add(time.Duration(j.interval) * time.Minute))
			if err := exec(`INSERT INTO scheduled_jobs(id,name,description,action_type,interval_minutes,enabled,notify,next_run_at,created_at,updated_at) VALUES(?,?,?,?,?,1,1,?,?,?)`, j.id, j.name, j.desc, j.action, j.interval, next, stamp, stamp); err != nil {
				return err
			}
		}

		if err := exec(`INSERT INTO notifications(id,title,body,level,created_at) VALUES(?,?,?,?,?)`, "seed-notification-welcome", "Explore your demo workspace", "Open Graph to inspect dependencies, sub-issues, semantic Concept links, a PR completion and Notes connected to the graph.", "info", stamp); err != nil {
			return err
		}
		if err := exec(`INSERT INTO workspace_pins(entity_type,entity_id,label,color,position,created_at) VALUES(?,?,?,?,?,?)`, "repository", "seed-repo-idios", "Important repository", "accent", 0, stamp); err != nil {
			return err
		}
		if err := exec(`INSERT INTO workspace_pins(entity_type,entity_id,label,color,position,created_at) VALUES(?,?,?,?,?,?)`, "issue", "seed-issue-idios-5", "Blocked graph polish", "warning", 1, stamp); err != nil {
			return err
		}

		activity := []struct{ typ, id, action, payload string }{
			{"repository", "seed-repo-idios", "repository.created", "IDIOS demo repository created"},
			{"issue", "seed-issue-idios-1", "issue.started", "Theme token work is in progress"},
			{"pull_request", "seed-pr-cdin-1", "pull_request.accepted", "Accepted PR produced a completion on main"},
			{"note", "seed-note-theme", "note.created", "Idea linked to an Issue and made visible in the graph"},
		}
		for _, a := range activity {
			if err := exec(`INSERT INTO activity_events(id,entity_type,entity_id,action,payload,created_at) VALUES(?,?,?,?,?,?)`, domain.NewID(), a.typ, a.id, a.action, a.payload, stamp); err != nil {
				return err
			}
		}
		return exec(`INSERT INTO app_settings(key,value,updated_at) VALUES('seed.demo.v1','complete',?)`, stamp)
	})
	if err != nil {
		return domain.SeedReport{}, err
	}
	if already {
		report.AlreadySeeded = true
		return report, nil
	}
	_ = s.RecordActivity(ctx, "workspace", "default", "workspace.demo_seeded", "Created the IDIOS sample workspace")
	return report, nil
}

func nullableStatusTime(status, stamp string) any {
	if status == string(domain.IssueDone) || status == string(domain.IssueCancelled) {
		return stamp
	}
	return nil
}
