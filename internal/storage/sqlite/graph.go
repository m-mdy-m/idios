package sqlite

import (
	"context"
	"fmt"
	"sort"

	"idios/internal/domain"
)

// ListGraph projects every entity and edge into one navigable view. Structural, workflow
// and semantic edges are stored separately and only meet here.
func (s *Store) ListGraph(ctx context.Context) (domain.Graph, error) {
	g := domain.Graph{Nodes: []domain.GraphNode{}, Edges: []domain.GraphEdge{}}

	repos, err := s.ListRepositories(ctx)
	if err != nil {
		return g, err
	}
	for _, r := range repos {
		g.Nodes = append(g.Nodes, domain.GraphNode{ID: graphID("repository", r.ID), Type: "repository", Label: r.Name, Reference: "repo:" + r.Name})
	}

	branches, err := s.p().all(ctx, `SELECT b.id,b.repository_id,b.name,r.name AS repository_name FROM branches b JOIN repositories r ON r.id=b.repository_id`)
	if err != nil {
		return g, err
	}
	for _, b := range branches {
		id, repoID, name := str(b, "id"), str(b, "repository_id"), str(b, "name")
		g.Nodes = append(g.Nodes, domain.GraphNode{ID: graphID("branch", id), Type: "branch", Label: str(b, "repository_name") + " / " + name, Reference: "branch:" + str(b, "repository_name") + "/" + name})
		g.Edges = append(g.Edges, domain.GraphEdge{Source: graphID("repository", repoID), Target: graphID("branch", id), Type: "has_branch"})
	}

	projects, err := s.ListProjects(ctx)
	if err != nil {
		return g, err
	}
	for _, p := range projects {
		g.Nodes = append(g.Nodes, domain.GraphNode{ID: graphID("project", p.ID), Type: "project", Label: p.Name, Reference: "project:" + p.Name})
	}

	concepts, err := s.ListConcepts(ctx)
	if err != nil {
		return g, err
	}
	for _, c := range concepts {
		g.Nodes = append(g.Nodes, domain.GraphNode{ID: graphID("concept", c.ID), Type: "concept", Label: c.Name, Reference: "concept:" + c.Name})
	}

	issues, err := s.ListIssues(ctx, "", "")
	if err != nil {
		return g, err
	}
	for _, i := range issues {
		ref := reference(i.RepositoryName, "#", i.Number)
		g.Nodes = append(g.Nodes, domain.GraphNode{ID: graphID("issue", i.ID), Type: "issue", Label: ref + " · " + i.Title, Status: string(i.Status), Reference: "issue:" + ref})
		g.Edges = append(g.Edges, domain.GraphEdge{Source: graphID("repository", i.RepositoryID), Target: graphID("issue", i.ID), Type: "owns_issue"})
		if i.ParentIssueID != nil {
			g.Edges = append(g.Edges, domain.GraphEdge{Source: graphID("issue", *i.ParentIssueID), Target: graphID("issue", i.ID), Type: "has_sub_issue"})
		}
	}

	dependencies, err := s.ListDependencyEdges(ctx)
	if err != nil {
		return g, err
	}
	for _, e := range dependencies {
		g.Edges = append(g.Edges, domain.GraphEdge{Source: graphID("issue", e.Source), Target: graphID("issue", e.Target), Type: "depends_on"})
	}

	prs, err := s.ListPullRequests(ctx, "", "")
	if err != nil {
		return g, err
	}
	for _, p := range prs {
		ref := reference(p.RepositoryName, "!", p.Number)
		g.Nodes = append(g.Nodes, domain.GraphNode{ID: graphID("pull_request", p.ID), Type: "pull_request", Label: ref + " · " + p.Title, Status: string(p.Status), Reference: "pr:" + ref})
		g.Edges = append(g.Edges,
			domain.GraphEdge{Source: graphID("issue", p.IssueID), Target: graphID("pull_request", p.ID), Type: "has_pull_request"},
			domain.GraphEdge{Source: graphID("pull_request", p.ID), Target: graphID("branch", p.TargetBranchID), Type: "targets_branch"})
	}

	completions, err := s.ListCompletions(ctx, "")
	if err != nil {
		return g, err
	}
	for _, c := range completions {
		g.Nodes = append(g.Nodes, domain.GraphNode{ID: graphID("completion", c.ID), Type: "completion", Label: fmt.Sprintf("%s completion %d · %s", c.BranchName, c.Sequence, c.Summary), Reference: completionReference(c)})
		g.Edges = append(g.Edges,
			domain.GraphEdge{Source: graphID("completion", c.ID), Target: graphID("issue", c.IssueID), Type: "completes_issue"},
			domain.GraphEdge{Source: graphID("completion", c.ID), Target: graphID("branch", c.BranchID), Type: "recorded_on"},
			domain.GraphEdge{Source: graphID("pull_request", c.PullRequestID), Target: graphID("completion", c.ID), Type: "produces_completion"})
	}

	// Notes only enter the graph when the user explicitly marks them as graph-visible.
	notes, err := s.ListNotes(ctx)
	if err != nil {
		return g, err
	}
	for _, n := range notes {
		if !n.GraphVisible {
			continue
		}
		g.Nodes = append(g.Nodes, domain.GraphNode{ID: graphID("note", n.ID), Type: "note", Label: n.Title, Reference: "note:" + n.ID})
		if n.LinkedEntityType != "" && n.LinkedEntityID != "" {
			g.Edges = append(g.Edges, domain.GraphEdge{Source: graphID("note", n.ID), Target: graphID(n.LinkedEntityType, n.LinkedEntityID), Type: "note_about"})
		}
	}

	labels, err := s.ListLabels(ctx, "")
	if err != nil {
		return g, err
	}
	for _, l := range labels {
		g.Nodes = append(g.Nodes, domain.GraphNode{ID: graphID("label", l.ID), Type: "label", Label: l.Name, Reference: labelReference(l)})
		if l.RepositoryID != nil {
			g.Edges = append(g.Edges, domain.GraphEdge{Source: graphID("repository", *l.RepositoryID), Target: graphID("label", l.ID), Type: "defines_label"})
		}
	}

	milestones, err := s.ListMilestones(ctx, "", "")
	if err != nil {
		return g, err
	}
	for _, m := range milestones {
		g.Nodes = append(g.Nodes, domain.GraphNode{ID: graphID("milestone", m.ID), Type: "milestone", Label: m.Title, Reference: milestoneReference(m)})
		g.Edges = append(g.Edges, domain.GraphEdge{Source: graphID(m.ScopeType, m.ScopeID), Target: graphID("milestone", m.ID), Type: "has_milestone"})
	}

	for _, assoc := range []struct{ sql, sourceType, sourceColumn, targetType, targetColumn, edgeType string }{
		{`SELECT project_id,issue_id FROM project_issues`, "project", "project_id", "issue", "issue_id", "contains_issue"},
		{`SELECT issue_id,label_id FROM issue_labels`, "issue", "issue_id", "label", "label_id", "labelled_with"},
		{`SELECT issue_id,milestone_id FROM issue_milestones`, "issue", "issue_id", "milestone", "milestone_id", "in_milestone"},
		{`SELECT source_type,source_id,target_type,target_id,relation_type FROM relations`, "auto", "source_id", "auto", "target_id", "relation_type"},
	} {
		rows, err := s.p().all(ctx, assoc.sql)
		if err != nil {
			return g, err
		}
		for _, r := range rows {
			sourceType, targetType, edgeType := assoc.sourceType, assoc.targetType, assoc.edgeType
			if sourceType == "auto" {
				// Semantic relations carry both endpoint types and the actual edge type in the row.
				sourceType, targetType = str(r, "source_type"), str(r, "target_type")
				edgeType = str(r, "relation_type")
			}
			g.Edges = append(g.Edges, domain.GraphEdge{
				Source: graphID(sourceType, str(r, assoc.sourceColumn)),
				Target: graphID(targetType, str(r, assoc.targetColumn)),
				Type:   edgeType,
			})
		}
	}

	// Stable ordering makes CLI output, JSON snapshots and tests reproducible.
	sort.Slice(g.Nodes, func(i, j int) bool {
		if g.Nodes[i].Type == g.Nodes[j].Type {
			return g.Nodes[i].Reference < g.Nodes[j].Reference
		}
		return g.Nodes[i].Type < g.Nodes[j].Type
	})
	sort.Slice(g.Edges, func(i, j int) bool {
		a, b := g.Edges[i], g.Edges[j]
		if a.Source == b.Source {
			if a.Type == b.Type {
				return a.Target < b.Target
			}
			return a.Type < b.Type
		}
		return a.Source < b.Source
	})
	return g, nil
}

func graphID(typ, id string) string { return typ + ":" + id }

func completionReference(c domain.CompletionRecord) string {
	return fmt.Sprintf("completion:%s/%s/%d", c.RepositoryName, c.BranchName, c.Sequence)
}

func labelReference(l domain.Label) string {
	if l.ScopeType == "global" {
		return "label:global/" + l.Name
	}
	return "label:repository/" + l.RepositoryName + "/" + l.Name
}

func milestoneReference(m domain.Milestone) string {
	return "milestone:" + m.ScopeType + "/" + m.ScopeName + "/" + m.Title
}
