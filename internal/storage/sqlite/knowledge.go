package sqlite

import (
	"context"
	"fmt"
	"strings"

	"idios/internal/domain"
)

func conceptFrom(r Row) domain.Concept {
	return domain.Concept{
		ID:          str(r, "id"),
		Name:        str(r, "name"),
		Description: str(r, "description"),
		CreatedAt:   parseTime(r["created_at"]),
		UpdatedAt:   parseTime(r["updated_at"]),
	}
}

func (s *Store) CreateConcept(ctx context.Context, name, description string) (domain.Concept, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.Concept{}, fmt.Errorf("%w: concept name is required", domain.ErrInvalid)
	}
	id, now := domain.NewID(), nowText()
	if _, err := s.p().changes(ctx, `INSERT INTO concepts(id,name,description,created_at,updated_at) VALUES(?,?,?,?,?)`,
		id, name, strings.TrimSpace(description), now, now); err != nil {
		return domain.Concept{}, err
	}
	_ = s.RecordActivity(ctx, "concept", id, "concept.created", marshalPayload(map[string]string{"name": name}))
	return s.GetConcept(ctx, id)
}

func (s *Store) GetConcept(ctx context.Context, ref string) (domain.Concept, error) {
	row, err := s.p().must(ctx, fmt.Sprintf("concept %q", ref),
		`SELECT id,name,description,created_at,updated_at FROM concepts WHERE id=? OR name=? COLLATE NOCASE LIMIT 1`, ref, ref)
	if err != nil {
		return domain.Concept{}, err
	}
	return conceptFrom(row), nil
}

func (s *Store) ListConcepts(ctx context.Context) ([]domain.Concept, error) {
	rows, err := s.p().all(ctx, `SELECT id,name,description,created_at,updated_at FROM concepts ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	return collect(rows, conceptFrom), nil
}

// CreateRelation records one typed semantic edge after resolving both endpoints. Workflow
// edges are rejected here so `depends_on` exists in exactly one place.
func (s *Store) CreateRelation(ctx context.Context, sourceType, sourceID, targetType, targetID, relationType string) (domain.Relation, error) {
	sourceType, targetType, relationType = strings.TrimSpace(sourceType), strings.TrimSpace(targetType), strings.TrimSpace(relationType)
	if sourceID == "" || targetID == "" || relationType == "" {
		return domain.Relation{}, fmt.Errorf("%w: source, target and relation type are required", domain.ErrInvalid)
	}
	if workflowRelation(relationType) {
		return domain.Relation{}, fmt.Errorf("%w: workflow edges must use the dedicated dependency or parent-issue fields", domain.ErrInvalid)
	}
	if !semanticRelation(relationType) {
		return domain.Relation{}, fmt.Errorf("%w: unsupported semantic relation type %q", domain.ErrInvalid, relationType)
	}
	resolvedSource, err := s.resolveEntityID(ctx, sourceType, sourceID)
	if err != nil {
		return domain.Relation{}, fmt.Errorf("source: %w", err)
	}
	resolvedTarget, err := s.resolveEntityID(ctx, targetType, targetID)
	if err != nil {
		return domain.Relation{}, fmt.Errorf("target: %w", err)
	}
	if sourceType == targetType && resolvedSource == resolvedTarget {
		return domain.Relation{}, fmt.Errorf("%w: self-relations are not allowed", domain.ErrInvalid)
	}
	id, created := domain.NewID(), nowText()
	if _, err := s.p().changes(ctx, `INSERT INTO relations(id,source_type,source_id,target_type,target_id,relation_type,created_at) VALUES(?,?,?,?,?,?,?)`,
		id, sourceType, resolvedSource, targetType, resolvedTarget, relationType, created); err != nil {
		return domain.Relation{}, err
	}
	_ = s.RecordActivity(ctx, sourceType, resolvedSource, "graph.relation_created", marshalPayload(map[string]string{"target_type": targetType, "target_id": resolvedTarget, "relation_type": relationType}))
	return domain.Relation{ID: id, SourceType: sourceType, SourceID: resolvedSource, TargetType: targetType, TargetID: resolvedTarget, Type: relationType, CreatedAt: parseTime(created)}, nil
}

func workflowRelation(kind string) bool {
	switch kind {
	case "depends_on", "prerequisite_of", "parent_of", "blocks":
		return true
	}
	return false
}

func semanticRelation(kind string) bool {
	switch kind {
	case "related_to", "uses", "contains", "part_of", "explains", "contrasts_with", "uses_concept", "references", "supports", "contradicts", "implements", "implemented_by":
		return true
	}
	return false
}

func (s *Store) LinkIssueConcept(ctx context.Context, issueRef, conceptRef string) error {
	i, err := s.GetIssue(ctx, issueRef)
	if err != nil {
		return err
	}
	c, err := s.GetConcept(ctx, conceptRef)
	if err != nil {
		return err
	}
	_, err = s.CreateRelation(ctx, "issue", i.ID, "concept", c.ID, "uses_concept")
	return err
}

// resolveEntityID turns any accepted entity reference into its stable ID. Short spellings
// such as `label:global/lang` stay unambiguous because the scope is part of the reference.
func (s *Store) resolveEntityID(ctx context.Context, typ, ref string) (string, error) {
	switch typ {
	case "issue":
		x, err := s.GetIssue(ctx, ref)
		return x.ID, err
	case "repository":
		x, err := s.GetRepository(ctx, ref)
		return x.ID, err
	case "project":
		x, err := s.GetProject(ctx, ref)
		return x.ID, err
	case "concept":
		x, err := s.GetConcept(ctx, ref)
		return x.ID, err
	case "pull_request":
		x, err := s.GetPullRequest(ctx, ref)
		return x.ID, err
	case "branch":
		return s.resolveBranchID(ctx, ref)
	case "label":
		return s.resolveLabelID(ctx, ref)
	case "milestone":
		x, err := s.getMilestone(ctx, ref)
		return x.ID, err
	case "completion":
		return s.resolveCompletionID(ctx, ref)
	case "note":
		row, err := s.p().must(ctx, fmt.Sprintf("note %q", ref), `SELECT id FROM notes WHERE id=? OR title=? COLLATE NOCASE LIMIT 1`, ref, ref)
		if err != nil {
			return "", err
		}
		return str(row, "id"), nil
	case "shelf_book":
		row, err := s.p().must(ctx, fmt.Sprintf("Shelf book %q", ref), `SELECT id FROM shelf_books WHERE id=?`, ref)
		if err != nil {
			return "", err
		}
		return str(row, "id"), nil
	default:
		return "", fmt.Errorf("%w: unknown graph entity type %q", domain.ErrInvalid, typ)
	}
}

func (s *Store) resolveBranchID(ctx context.Context, ref string) (string, error) {
	raw := strings.TrimPrefix(ref, "branch:")
	if raw != ref {
		if slash := strings.Index(raw, "/"); slash > 0 {
			row, err := s.p().must(ctx, fmt.Sprintf("branch %q", ref),
				`SELECT b.id FROM branches b JOIN repositories r ON r.id=b.repository_id WHERE r.name=? COLLATE NOCASE AND b.name=? COLLATE NOCASE LIMIT 1`,
				raw[:slash], raw[slash+1:])
			if err != nil {
				return "", err
			}
			return str(row, "id"), nil
		}
	}
	rows, err := s.p().all(ctx, `SELECT b.id FROM branches b WHERE b.id=? OR b.name=? COLLATE NOCASE ORDER BY CASE WHEN b.id=? THEN 0 ELSE 1 END LIMIT 2`, raw, raw, raw)
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "", fmt.Errorf("%w: branch %q", domain.ErrNotFound, ref)
	}
	if len(rows) > 1 {
		return "", fmt.Errorf("%w: branch name %q is ambiguous; use branch:Repository/name", domain.ErrInvalid, ref)
	}
	return str(rows[0], "id"), nil
}

func (s *Store) resolveLabelID(ctx context.Context, ref string) (string, error) {
	raw := strings.TrimPrefix(ref, "label:")
	if raw != ref {
		if id, err := s.labelFromScopedRef(ctx, ref, raw); err != nil || id != "" {
			return id, err
		}
	}
	rows, err := s.p().all(ctx, `SELECT id FROM labels WHERE id=? OR name=? COLLATE NOCASE ORDER BY CASE WHEN id=? THEN 0 ELSE 1 END LIMIT 2`, raw, raw, raw)
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "", fmt.Errorf("%w: label %q", domain.ErrNotFound, ref)
	}
	if len(rows) > 1 {
		return "", fmt.Errorf("%w: label name %q is ambiguous; use label:global/name or label:Repository/name", domain.ErrInvalid, ref)
	}
	return str(rows[0], "id"), nil
}

// labelFromScopedRef resolves `label:global/name`, `label:repository/Repo/name` and the
// shorter `label:Repo/name`; an empty ID means "not written in that scope, try the bare name".
func (s *Store) labelFromScopedRef(ctx context.Context, ref, raw string) (string, error) {
	parts := strings.SplitN(raw, "/", 3)
	notFound := func() error { return fmt.Errorf("%w: label %q", domain.ErrNotFound, ref) }
	switch {
	case len(parts) >= 2 && strings.EqualFold(parts[0], "global"):
		row, ok, err := s.p().row(ctx, `SELECT id FROM labels WHERE scope_type='global' AND name=? COLLATE NOCASE LIMIT 1`, parts[1])
		if err != nil {
			return "", err
		}
		if !ok {
			return "", notFound()
		}
		return str(row, "id"), nil
	case len(parts) == 3 && strings.EqualFold(parts[0], "repository"):
		row, ok, err := s.p().row(ctx, `SELECT l.id FROM labels l JOIN repositories r ON r.id=l.repository_id
 WHERE l.scope_type='repository' AND r.name=? COLLATE NOCASE AND l.name=? COLLATE NOCASE LIMIT 1`, parts[1], parts[2])
		if err != nil {
			return "", err
		}
		if !ok {
			return "", notFound()
		}
		return str(row, "id"), nil
	case len(parts) == 2:
		row, ok, err := s.p().row(ctx, `SELECT l.id FROM labels l JOIN repositories r ON r.id=l.repository_id
 WHERE l.scope_type='repository' AND r.name=? COLLATE NOCASE AND l.name=? COLLATE NOCASE LIMIT 1`, parts[0], parts[1])
		if err != nil || !ok {
			return "", err
		}
		return str(row, "id"), nil
	}
	return "", nil
}

func (s *Store) resolveCompletionID(ctx context.Context, ref string) (string, error) {
	raw := strings.TrimPrefix(ref, "completion:")
	if raw != ref {
		if parts := strings.SplitN(raw, "/", 3); len(parts) == 3 {
			row, err := s.p().must(ctx, fmt.Sprintf("completion %q", ref),
				`SELECT c.id FROM completions c JOIN repositories r ON r.id=c.repository_id JOIN branches b ON b.id=c.branch_id
 WHERE r.name=? COLLATE NOCASE AND b.name=? COLLATE NOCASE AND c.sequence=? LIMIT 1`, parts[0], parts[1], parts[2])
			if err != nil {
				return "", err
			}
			return str(row, "id"), nil
		}
	}
	row, err := s.p().must(ctx, fmt.Sprintf("completion %q", ref), `SELECT id FROM completions WHERE id=? LIMIT 1`, raw)
	if err != nil {
		return "", err
	}
	return str(row, "id"), nil
}
