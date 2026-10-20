package sqlite

import (
	"context"
	"fmt"
	"strings"

	"idios/internal/domain"
)

// DeleteRepository removes a local repository and its repository-owned work in one
// transaction. Notes and external completion records are preserved but detached from
// records that no longer exist. This is intentionally a hard delete, so the UI must
// require the user to type the repository name before calling it.
func (s *Store) DeleteRepository(ctx context.Context, ref string) error {
	repo, err := s.GetRepository(ctx, ref)
	if err != nil {
		return err
	}
	return s.run(ctx, func(p provider) error {
		if _, err := p.must(ctx, "repository", `SELECT id FROM repositories WHERE id=?`, repo.ID); err != nil {
			return err
		}

		// Preserve completions in other repositories that happened to branch from this
		// repository's history. Their provenance becomes an explicit missing parent.
		if _, err := p.changes(ctx, `UPDATE completions SET parent_completion_id=NULL WHERE repository_id<>? AND parent_completion_id IN (SELECT id FROM completions WHERE repository_id=?)`, repo.ID, repo.ID); err != nil {
			return err
		}

		// Preserve notes but remove dead links before their targets disappear.
		if _, err := p.changes(ctx, `UPDATE notes SET linked_entity_type='',linked_entity_id='',graph_visible=0,updated_at=? WHERE
			(linked_entity_type IN ('repository','repo') AND linked_entity_id=?) OR
			(linked_entity_type='issue' AND linked_entity_id IN (SELECT id FROM issues WHERE repository_id=?)) OR
			(linked_entity_type='pull_request' AND linked_entity_id IN (SELECT id FROM pull_requests WHERE repository_id=?)) OR
			(linked_entity_type='branch' AND linked_entity_id IN (SELECT id FROM branches WHERE repository_id=?)) OR
			(linked_entity_type='completion' AND linked_entity_id IN (SELECT id FROM completions WHERE repository_id=?)) OR
			(linked_entity_type='label' AND linked_entity_id IN (SELECT id FROM labels WHERE repository_id=?)) OR
			(linked_entity_type='milestone' AND linked_entity_id IN (SELECT id FROM milestones WHERE scope_type='repository' AND scope_id=?))`,
			nowText(), repo.ID, repo.ID, repo.ID, repo.ID, repo.ID, repo.ID, repo.ID); err != nil {
			return err
		}

		// Relations and pins are polymorphic (without SQL foreign keys), so remove every
		// edge/pin attached to one of the repository's owned entities explicitly.
		if _, err := p.changes(ctx, `DELETE FROM relations WHERE
			(source_type IN ('repository','repo') AND source_id=?) OR (target_type IN ('repository','repo') AND target_id=?) OR
			source_id IN (SELECT id FROM issues WHERE repository_id=?) OR target_id IN (SELECT id FROM issues WHERE repository_id=?) OR
			source_id IN (SELECT id FROM pull_requests WHERE repository_id=?) OR target_id IN (SELECT id FROM pull_requests WHERE repository_id=?) OR
			source_id IN (SELECT id FROM branches WHERE repository_id=?) OR target_id IN (SELECT id FROM branches WHERE repository_id=?) OR
			source_id IN (SELECT id FROM completions WHERE repository_id=?) OR target_id IN (SELECT id FROM completions WHERE repository_id=?) OR
			source_id IN (SELECT id FROM labels WHERE repository_id=?) OR target_id IN (SELECT id FROM labels WHERE repository_id=?) OR
			source_id IN (SELECT id FROM milestones WHERE scope_type='repository' AND scope_id=?) OR target_id IN (SELECT id FROM milestones WHERE scope_type='repository' AND scope_id=?)`,
			repo.ID, repo.ID, repo.ID, repo.ID, repo.ID, repo.ID, repo.ID, repo.ID, repo.ID, repo.ID, repo.ID, repo.ID, repo.ID, repo.ID); err != nil {
			return err
		}
		if _, err := p.changes(ctx, `DELETE FROM workspace_pins WHERE entity_id=? OR entity_id IN (
			SELECT id FROM issues WHERE repository_id=? UNION SELECT id FROM pull_requests WHERE repository_id=?
			UNION SELECT id FROM branches WHERE repository_id=? UNION SELECT id FROM completions WHERE repository_id=?
			UNION SELECT id FROM labels WHERE repository_id=? UNION SELECT id FROM milestones WHERE scope_type='repository' AND scope_id=?)`,
			repo.ID, repo.ID, repo.ID, repo.ID, repo.ID, repo.ID, repo.ID); err != nil {
			return err
		}

		// Incoming dependencies can cross repository boundaries; remove both ends so a
		// remaining issue never points at an issue that is about to be deleted.
		if _, err := p.changes(ctx, `DELETE FROM issue_dependencies WHERE issue_id IN (SELECT id FROM issues WHERE repository_id=?) OR prerequisite_issue_id IN (SELECT id FROM issues WHERE repository_id=?)`, repo.ID, repo.ID); err != nil {
			return err
		}
		// Child issues are retained as standalone issues if the parent repository is removed
		// through a legacy/corrupt database. Normal writes only allow same-repository parents.
		if _, err := p.changes(ctx, `UPDATE issues SET parent_issue_id=NULL WHERE parent_issue_id IN (SELECT id FROM issues WHERE repository_id=?)`, repo.ID); err != nil {
			return err
		}

		// Delete completion leaves before parents to respect the self-referencing RESTRICT FK.
		completionRows, err := p.all(ctx, `SELECT id,parent_completion_id FROM completions WHERE repository_id=?`, repo.ID)
		if err != nil {
			return err
		}
		completionIDs := make(map[string]bool, len(completionRows))
		children := make(map[string][]string, len(completionRows))
		for _, row := range completionRows {
			id := str(row, "id")
			completionIDs[id] = true
			if parent := str(row, "parent_completion_id"); parent != "" {
				children[parent] = append(children[parent], id)
			}
		}
		state := make(map[string]uint8, len(completionRows))
		var deleteCompletion func(string) error
		deleteCompletion = func(id string) error {
			if state[id] == 2 {
				return nil
			}
			if state[id] == 1 {
				return fmt.Errorf("%w: completion history contains a parent cycle; repository was not deleted", domain.ErrConflict)
			}
			state[id] = 1
			for _, child := range children[id] {
				if completionIDs[child] {
					if err := deleteCompletion(child); err != nil {
						return err
					}
				}
			}
			if _, err := p.changes(ctx, `DELETE FROM completions WHERE id=?`, id); err != nil {
				return err
			}
			state[id] = 2
			return nil
		}
		for id := range completionIDs {
			if err := deleteCompletion(id); err != nil {
				return err
			}
		}
		if _, err := p.changes(ctx, `DELETE FROM pull_requests WHERE repository_id=?`, repo.ID); err != nil {
			return err
		}
		if _, err := p.changes(ctx, `DELETE FROM issues WHERE repository_id=?`, repo.ID); err != nil {
			return err
		}
		if _, err := p.changes(ctx, `DELETE FROM milestones WHERE scope_type='repository' AND scope_id=?`, repo.ID); err != nil {
			return err
		}
		// Branch deletion is now safe: PRs and completions have already gone, and cross-repo
		// branch/completion references use SET NULL semantics.
		if _, err := p.changes(ctx, `DELETE FROM branches WHERE repository_id=?`, repo.ID); err != nil {
			return err
		}
		changed, err := p.changes(ctx, `DELETE FROM repositories WHERE id=?`, repo.ID)
		if err != nil {
			return err
		}
		if changed == 0 {
			return fmt.Errorf("%w: repository %q", domain.ErrNotFound, repo.Name)
		}
		return activity(ctx, p, "repository", repo.ID, "repository.deleted", marshalPayload(map[string]string{"name": repo.Name, "cascade": "issues,pull_requests,completions,branches,local_labels,milestones,settings"}))
	})
}

// DeleteIssue is deliberately guarded. An Issue with child Issues, incoming blockers,
// PRs or completion history must be resolved first; deleting it must not silently rewrite
// other work or erase accepted-work history.
func (s *Store) DeleteIssue(ctx context.Context, ref string) error {
	issue, err := s.GetIssue(ctx, ref)
	if err != nil {
		return err
	}
	return s.run(ctx, func(p provider) error {
		checks := []struct {
			label string
			sql   string
		}{
			{"child Issue(s)", `SELECT COUNT(*) AS n FROM issues WHERE parent_issue_id=?`},
			{"other Issue(s) depending on this Issue", `SELECT COUNT(*) AS n FROM issue_dependencies WHERE prerequisite_issue_id=?`},
			{"Pull Request(s)", `SELECT COUNT(*) AS n FROM pull_requests WHERE issue_id=?`},
			{"completion record(s)", `SELECT COUNT(*) AS n FROM completions WHERE issue_id=?`},
		}
		var blockers []string
		for _, check := range checks {
			row, ok, err := p.row(ctx, check.sql, issue.ID)
			if err != nil {
				return err
			}
			if ok && i64(row["n"]) > 0 {
				blockers = append(blockers, fmt.Sprintf("%d %s", i64(row["n"]), check.label))
			}
		}
		if len(blockers) > 0 {
			return fmt.Errorf("%w: cannot delete %s#%d while it has %s; remove or resolve those references first", domain.ErrConflict, issue.RepositoryName, issue.Number, strings.Join(blockers, ", "))
		}
		if _, err := p.changes(ctx, `DELETE FROM relations WHERE source_id=? OR target_id=?`, issue.ID, issue.ID); err != nil {
			return err
		}
		if _, err := p.changes(ctx, `DELETE FROM workspace_pins WHERE entity_id=?`, issue.ID); err != nil {
			return err
		}
		if _, err := p.changes(ctx, `UPDATE notes SET linked_entity_type='',linked_entity_id='',graph_visible=0,updated_at=? WHERE linked_entity_type='issue' AND linked_entity_id=?`, nowText(), issue.ID); err != nil {
			return err
		}
		changed, err := p.changes(ctx, `DELETE FROM issues WHERE id=?`, issue.ID)
		if err != nil {
			return err
		}
		if changed == 0 {
			return fmt.Errorf("%w: issue %q", domain.ErrNotFound, ref)
		}
		return activity(ctx, p, "issue", issue.ID, "issue.deleted", marshalPayload(map[string]any{"repository": issue.RepositoryName, "number": issue.Number, "title": issue.Title}))
	})
}

func (s *Store) DeletePullRequest(ctx context.Context, ref string) error {
	pr, err := s.GetPullRequest(ctx, ref)
	if err != nil {
		return err
	}
	return s.run(ctx, func(p provider) error {
		row, ok, err := p.row(ctx, `SELECT COUNT(*) AS n FROM completions WHERE pull_request_id=?`, pr.ID)
		if err != nil {
			return err
		}
		if ok && i64(row["n"]) > 0 {
			return fmt.Errorf("%w: %s!%d has completion history; accepted Pull Requests are kept as the record of finished work", domain.ErrConflict, pr.RepositoryName, pr.Number)
		}
		if _, err := p.changes(ctx, `DELETE FROM relations WHERE source_id=? OR target_id=?`, pr.ID, pr.ID); err != nil {
			return err
		}
		if _, err := p.changes(ctx, `DELETE FROM workspace_pins WHERE entity_id=?`, pr.ID); err != nil {
			return err
		}
		if _, err := p.changes(ctx, `UPDATE notes SET linked_entity_type='',linked_entity_id='',graph_visible=0,updated_at=? WHERE linked_entity_type='pull_request' AND linked_entity_id=?`, nowText(), pr.ID); err != nil {
			return err
		}
		changed, err := p.changes(ctx, `DELETE FROM pull_requests WHERE id=?`, pr.ID)
		if err != nil {
			return err
		}
		if changed == 0 {
			return fmt.Errorf("%w: pull request %q", domain.ErrNotFound, ref)
		}
		return activity(ctx, p, "pull_request", pr.ID, "pull_request.deleted", marshalPayload(map[string]any{"repository": pr.RepositoryName, "number": pr.Number, "title": pr.Title}))
	})
}

func (s *Store) DeleteProject(ctx context.Context, ref string) error {
	project, err := s.GetProject(ctx, ref)
	if err != nil {
		return err
	}
	return s.run(ctx, func(p provider) error {
		if _, err := p.changes(ctx, `DELETE FROM relations WHERE (source_type='project' AND source_id=?) OR (target_type='project' AND target_id=?) OR source_id IN (SELECT id FROM milestones WHERE scope_type='project' AND scope_id=?) OR target_id IN (SELECT id FROM milestones WHERE scope_type='project' AND scope_id=?)`, project.ID, project.ID, project.ID, project.ID); err != nil {
			return err
		}
		if _, err := p.changes(ctx, `DELETE FROM workspace_pins WHERE entity_id=? OR entity_id IN (SELECT id FROM milestones WHERE scope_type='project' AND scope_id=?)`, project.ID, project.ID); err != nil {
			return err
		}
		if _, err := p.changes(ctx, `UPDATE notes SET linked_entity_type='',linked_entity_id='',graph_visible=0,updated_at=? WHERE
			(linked_entity_type='project' AND linked_entity_id=?) OR
			(linked_entity_type='milestone' AND linked_entity_id IN (SELECT id FROM milestones WHERE scope_type='project' AND scope_id=?))`, nowText(), project.ID, project.ID); err != nil {
			return err
		}
		if _, err := p.changes(ctx, `DELETE FROM milestones WHERE scope_type='project' AND scope_id=?`, project.ID); err != nil {
			return err
		}
		changed, err := p.changes(ctx, `DELETE FROM projects WHERE id=?`, project.ID)
		if err != nil {
			return err
		}
		if changed == 0 {
			return fmt.Errorf("%w: project %q", domain.ErrNotFound, ref)
		}
		return activity(ctx, p, "project", project.ID, "project.deleted", marshalPayload(map[string]string{"name": project.Name}))
	})
}

func (s *Store) DeleteConcept(ctx context.Context, ref string) error {
	concept, err := s.GetConcept(ctx, ref)
	if err != nil {
		return err
	}
	return s.run(ctx, func(p provider) error {
		if _, err := p.changes(ctx, `DELETE FROM relations WHERE (source_type='concept' AND source_id=?) OR (target_type='concept' AND target_id=?)`, concept.ID, concept.ID); err != nil {
			return err
		}
		if _, err := p.changes(ctx, `DELETE FROM workspace_pins WHERE entity_id=?`, concept.ID); err != nil {
			return err
		}
		if _, err := p.changes(ctx, `UPDATE notes SET linked_entity_type='',linked_entity_id='',graph_visible=0,updated_at=? WHERE linked_entity_type='concept' AND linked_entity_id=?`, nowText(), concept.ID); err != nil {
			return err
		}
		changed, err := p.changes(ctx, `DELETE FROM concepts WHERE id=?`, concept.ID)
		if err != nil {
			return err
		}
		if changed == 0 {
			return fmt.Errorf("%w: concept %q", domain.ErrNotFound, ref)
		}
		return activity(ctx, p, "concept", concept.ID, "concept.deleted", marshalPayload(map[string]string{"name": concept.Name}))
	})
}

func (s *Store) DeleteQuote(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("%w: quote ID is required", domain.ErrInvalid)
	}
	return s.run(ctx, func(p provider) error {
		row, err := p.must(ctx, "quote", `SELECT text,author FROM quotes WHERE id=?`, id)
		if err != nil {
			return err
		}
		if _, err := p.changes(ctx, `DELETE FROM workspace_pins WHERE entity_id=?`, id); err != nil {
			return err
		}
		if _, err := p.changes(ctx, `DELETE FROM relations WHERE source_id=? OR target_id=?`, id, id); err != nil {
			return err
		}
		if _, err := p.changes(ctx, `DELETE FROM quotes WHERE id=?`, id); err != nil {
			return err
		}
		return activity(ctx, p, "quote", id, "quote.deleted", marshalPayload(map[string]string{"text": str(row, "text"), "author": str(row, "author")}))
	})
}

func (s *Store) DeleteLabel(ctx context.Context, ref string) error {
	labelID := strings.TrimSpace(ref)
	if labelID == "" {
		return fmt.Errorf("%w: label reference is required", domain.ErrInvalid)
	}
	// Prefer an exact ID before resolving a user-facing label name. Names may be
	// duplicated across global and repository scopes, but an ID is always unambiguous.
	if row, ok, lookupErr := s.p().row(ctx, `SELECT id FROM labels WHERE id=?`, labelID); lookupErr != nil {
		return lookupErr
	} else if ok {
		labelID = str(row, "id")
	} else {
		var resolveErr error
		labelID, resolveErr = s.resolveLabelID(ctx, ref)
		if resolveErr != nil {
			return resolveErr
		}
	}
	label, err := s.getLabel(ctx, labelID)
	if err != nil {
		return err
	}
	return s.run(ctx, func(p provider) error {
		if _, err := p.changes(ctx, `DELETE FROM relations WHERE (source_type='label' AND source_id=?) OR (target_type='label' AND target_id=?)`, label.ID, label.ID); err != nil {
			return err
		}
		if _, err := p.changes(ctx, `DELETE FROM workspace_pins WHERE entity_id=?`, label.ID); err != nil {
			return err
		}
		if _, err := p.changes(ctx, `UPDATE notes SET linked_entity_type='',linked_entity_id='',graph_visible=0,updated_at=? WHERE linked_entity_type='label' AND linked_entity_id=?`, nowText(), label.ID); err != nil {
			return err
		}
		changed, err := p.changes(ctx, `DELETE FROM labels WHERE id=?`, label.ID)
		if err != nil {
			return err
		}
		if changed == 0 {
			return fmt.Errorf("%w: label %q", domain.ErrNotFound, ref)
		}
		return activity(ctx, p, "label", label.ID, "label.deleted", marshalPayload(map[string]any{"name": label.Name, "scope": label.ScopeType, "repository": label.RepositoryName}))
	})
}

func (s *Store) DeleteMilestone(ctx context.Context, ref string) error {
	milestone, err := s.getMilestone(ctx, ref)
	if err != nil {
		return err
	}
	return s.run(ctx, func(p provider) error {
		if _, err := p.changes(ctx, `DELETE FROM relations WHERE (source_type='milestone' AND source_id=?) OR (target_type='milestone' AND target_id=?)`, milestone.ID, milestone.ID); err != nil {
			return err
		}
		if _, err := p.changes(ctx, `DELETE FROM workspace_pins WHERE entity_id=?`, milestone.ID); err != nil {
			return err
		}
		if _, err := p.changes(ctx, `UPDATE notes SET linked_entity_type='',linked_entity_id='',graph_visible=0,updated_at=? WHERE linked_entity_type='milestone' AND linked_entity_id=?`, nowText(), milestone.ID); err != nil {
			return err
		}
		changed, err := p.changes(ctx, `DELETE FROM milestones WHERE id=?`, milestone.ID)
		if err != nil {
			return err
		}
		if changed == 0 {
			return fmt.Errorf("%w: milestone %q", domain.ErrNotFound, ref)
		}
		return activity(ctx, p, "milestone", milestone.ID, "milestone.deleted", marshalPayload(map[string]string{"title": milestone.Title, "scope": milestone.ScopeType, "owner": milestone.ScopeName}))
	})
}

func (s *Store) DeleteNotification(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("%w: notification ID is required", domain.ErrInvalid)
	}
	return s.run(ctx, func(p provider) error {
		changed, err := p.changes(ctx, `DELETE FROM notifications WHERE id=?`, id)
		if err != nil {
			return err
		}
		if changed == 0 {
			return fmt.Errorf("%w: notification %q", domain.ErrNotFound, id)
		}
		return activity(ctx, p, "notification", id, "notification.deleted", "")
	})
}
