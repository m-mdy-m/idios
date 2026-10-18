package sqlite

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"idios/internal/domain"
	"idios/internal/ports"
)

// ImportLegacy reads (never mutates) an IDIOS 0.4.x SQLite database. Tasks become Issues.
// Historical done tasks receive explicitly-labelled synthetic PR/Completion records so the
// new engine's invariant (done requires accepted PR) remains true without claiming Git history.
func (s *Store) ImportLegacy(ctx context.Context, path string) (ports.LegacyImportResult, error) {
	if path == "" {
		return ports.LegacyImportResult{}, fmt.Errorf("%w: source database path required", domain.ErrInvalid)
	}
	if s.path != memoryPath {
		oldAbs, e1 := filepath.Abs(path)
		targetAbs, e2 := filepath.Abs(s.path)
		if e1 == nil && e2 == nil && filepath.Clean(oldAbs) == filepath.Clean(targetAbs) {
			return ports.LegacyImportResult{}, fmt.Errorf("%w: legacy source and Engine database must be different files", domain.ErrInvalid)
		}
	}
	old, err := openDatabase(path, true)
	if err != nil {
		return ports.LegacyImportResult{}, err
	}
	defer old.Close()
	legacy := provider{q: old}
	conceptRows, err := legacy.all(ctx, `SELECT id,name,description,created_at,updated_at FROM concepts ORDER BY seq`)
	if err != nil {
		return ports.LegacyImportResult{}, fmt.Errorf("not a supported IDIOS legacy SQLite database: %w", err)
	}
	tasks, err := legacy.all(ctx, `SELECT id,text,due_date,status,done_at,goal_id,source_id,created_at,updated_at FROM tasks ORDER BY seq`)
	if err != nil {
		return ports.LegacyImportResult{}, err
	}
	result := ports.LegacyImportResult{Repository: "Legacy Tasks"}
	repo, err := s.GetRepository(ctx, result.Repository)
	if err != nil {
		repo, err = s.CreateRepository(ctx, result.Repository, "Imported from a previous IDIOS SQLite database; original database remains unchanged.")
	}
	if err != nil {
		return result, err
	}
	conceptMap, err := importConcepts(ctx, s, conceptRows, &result)
	if err != nil {
		return result, err
	}
	issueMap, err := importTasks(ctx, s, repo, tasks, &result)
	if err != nil {
		return result, err
	}
	// Retain semantic relations among concepts and old tasks. Workflow dependencies are
	// deliberately not guessed/imported: their direction may have different legacy meaning.
	rels, relErr := legacy.all(ctx, `SELECT source_type,source_id,target_type,target_id,type FROM relations ORDER BY created_at`)
	if relErr == nil {
		result.ImportedRelations = importRelations(ctx, s, rels, conceptMap, issueMap)
	}
	_ = s.RecordActivity(ctx, "repository", repo.ID, "legacy.imported", marshalPayload(result))
	return result, nil
}

func importConcepts(ctx context.Context, s *Store, rows []Row, result *ports.LegacyImportResult) (map[string]string, error) {
	conceptMap := map[string]string{}
	for _, r := range rows {
		name := strings.TrimSpace(str(r, "name"))
		if name == "" {
			continue
		}
		c, err := s.CreateConcept(ctx, name, str(r, "description"))
		if err != nil {
			existing, lookErr := s.GetConcept(ctx, name)
			if lookErr != nil {
				return conceptMap, err
			}
			c = existing
		}
		conceptMap[str(r, "id")] = c.ID
		result.ImportedConcepts++
	}
	return conceptMap, nil
}

// importTasks recreates Tasks as Issues. A historical `done` Task moves to `in_progress`
// and is completed through a synthetic PR whose text states the migration provenance.
func importTasks(ctx context.Context, s *Store, repo domain.Repository, rows []Row, result *ports.LegacyImportResult) (map[string]string, error) {
	issueMap := map[string]string{}
	for _, t := range rows {
		oldID, title := str(t, "id"), strings.TrimSpace(str(t, "text"))
		if oldID == "" || title == "" {
			result.SkippedTasks++
			continue
		}
		if existing, err := s.GetIssue(ctx, oldID); err == nil && existing.ID != "" {
			issueMap[oldID] = existing.ID
			result.SkippedTasks++
			continue
		}
		legacyStatus := str(t, "status")
		description := fmt.Sprintf("Migrated from legacy Task %s. Legacy status: %s. Legacy done_at: %s. Legacy due_date: %s. Legacy goal_id: %s. Legacy source_id: %s.",
			oldID, legacyStatus, str(t, "done_at"), str(t, "due_date"), str(t, "goal_id"), str(t, "source_id"))
		status := domain.IssueOpen
		switch legacyStatus {
		case "skipped":
			status = domain.IssueCancelled
		case "done":
			status = domain.IssueInProgress
		}
		i, err := s.CreateIssue(ctx, domain.Issue{ID: oldID, RepositoryID: repo.ID, Title: title, Description: description, Status: status, DueDate: ptrString(t["due_date"])})
		if err != nil {
			result.SkippedTasks++
			continue
		}
		issueMap[oldID] = i.ID
		result.ImportedTasks++
		if legacyStatus != "done" {
			continue
		}
		pr, err := s.CreatePullRequest(ctx, i.ID, "Imported completion: "+title,
			fmt.Sprintf("Synthetic migration record. The legacy Task was marked done at %s. The original database did not contain a Pull Request for this task; no Git commit or merge is claimed.", str(t, "done_at")))
		if err != nil {
			return issueMap, err
		}
		if _, _, _, err := s.AcceptPullRequest(ctx, pr.ID, "Historical task completion imported from IDIOS 0.4.x; see PR description for migration provenance."); err != nil {
			return issueMap, err
		}
		result.ImportedDoneTasks++
	}
	return issueMap, nil
}

// importRelations keeps only semantic edges between concepts and issues; workflow edges
// (depends_on/prerequisite_of/blocks) are dropped because their legacy direction is unclear.
func importRelations(ctx context.Context, s *Store, rows []Row, conceptMap, issueMap map[string]string) int {
	imported := 0
	for _, r := range rows {
		sourceType, targetType := str(r, "source_type"), str(r, "target_type")
		sourceID, targetID := str(r, "source_id"), str(r, "target_id")
		if sourceType == "task" {
			sourceType, sourceID = "issue", issueMap[sourceID]
		}
		if targetType == "task" {
			targetType, targetID = "issue", issueMap[targetID]
		}
		if sourceType == "concept" {
			sourceID = conceptMap[sourceID]
		}
		if targetType == "concept" {
			targetID = conceptMap[targetID]
		}
		if sourceID == "" || targetID == "" || !importableEntity(sourceType) || !importableEntity(targetType) {
			continue
		}
		kind := str(r, "type")
		if kind == "depends_on" || kind == "prerequisite_of" || kind == "blocks" {
			continue
		}
		if _, err := s.CreateRelation(ctx, sourceType, sourceID, targetType, targetID, kind); err == nil {
			imported++
		}
	}
	return imported
}

func importableEntity(typ string) bool { return typ == "issue" || typ == "concept" }
