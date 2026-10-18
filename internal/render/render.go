// Package render turns Application results into the human-readable text and the
// machine-readable JSON that adapters show. It owns presentation only: no rules,
// no storage access, and no logging.
package render

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"text/tabwriter"

	"idios/internal/domain"
)

// Value writes v as indented JSON when asJSON is set, otherwise as human text.
func Value(w io.Writer, v any, asJSON bool) {
	if asJSON {
		JSON(w, v)
		return
	}
	switch x := v.(type) {
	case domain.Repository:
		fmt.Fprintf(w, "Repository %s\n  id: %s\n  default branch: main\n  description: %s\n", x.Name, x.ID, x.Description)
	case []domain.Repository:
		tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
		fmt.Fprintln(tw, "NAME\tDESCRIPTION\tID")
		for _, v := range x {
			fmt.Fprintf(tw, "%s\t%s\t%s\n", v.Name, v.Description, v.ID)
		}
		tw.Flush()
	case domain.Project:
		fmt.Fprintf(w, "Project %s\n  id: %s\n  %s\n", x.Name, x.ID, x.Description)
	case []domain.Project:
		for _, v := range x {
			fmt.Fprintf(w, "%-24s %s\n", v.Name, v.ID)
		}
	case domain.Issue:
		Issue(w, x)
	case []domain.Issue:
		tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
		fmt.Fprintln(tw, "REF\tSTATUS\tREADY\tTITLE")
		for _, v := range x {
			ready := "ready"
			if v.Blocked {
				ready = "blocked"
			}
			fmt.Fprintf(tw, "%s#%d\t%s\t%s\t%s\n", v.RepositoryName, v.Number, v.Status, ready, v.Title)
		}
		tw.Flush()
	case domain.PullRequest:
		PullRequest(w, x)
	case []domain.PullRequest:
		tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
		fmt.Fprintln(tw, "REF\tSTATUS\tISSUE\tTITLE")
		for _, v := range x {
			fmt.Fprintf(tw, "%s!%d\t%s\t%s#%d\t%s\n", v.RepositoryName, v.Number, v.Status, v.RepositoryName, v.IssueNumber, v.Title)
		}
		tw.Flush()
	case domain.CompletionRecord:
		Completion(w, x)
	case []domain.CompletionRecord:
		for _, v := range x {
			fmt.Fprintf(w, "%s/%s #%d · %s (Issue #%d, PR %s)\n", v.RepositoryName, v.BranchName, v.Sequence, v.Summary, v.IssueNumber, v.PullRequestID)
		}
	case domain.Concept:
		fmt.Fprintf(w, "Concept %s\n  id: %s\n  %s\n", x.Name, x.ID, x.Description)
	case []domain.Concept:
		for _, v := range x {
			fmt.Fprintf(w, "%-28s %s\n", v.Name, v.ID)
		}
	case domain.Graph:
		Graph(w, x)
	case []domain.Branch:
		for _, v := range x {
			mark := " "
			if v.IsDefault {
				mark = "*"
			}
			fmt.Fprintf(w, "%s %s  (%s)\n", mark, v.Name, v.ID)
		}
	case domain.Label:
		fmt.Fprintf(w, "Label %s (%s)\n  id: %s\n", x.Name, x.ScopeType, x.ID)
	case []domain.Label:
		for _, v := range x {
			scope := v.ScopeType
			if v.RepositoryName != "" {
				scope += " / " + v.RepositoryName
			}
			fmt.Fprintf(w, "%-22s %-18s %s\n", v.Name, scope, v.ID)
		}
	case []domain.Milestone:
		for _, v := range x {
			due := "no due date"
			if v.DueDate != nil {
				due = *v.DueDate
			}
			fmt.Fprintf(w, "%-24s %-12s due %s  (%s)\n", v.Title, v.ScopeName, due, v.ID)
		}
	case []domain.ActivityEvent:
		for _, v := range x {
			fmt.Fprintf(w, "%s  %-25s %s/%s %s\n", v.CreatedAt.Format("2006-01-02 15:04"), v.Action, v.EntityType, v.EntityID, v.Payload)
		}
	case []domain.RepositorySetting:
		for _, v := range x {
			fmt.Fprintf(w, "%s = %s\n", v.Key, v.Value)
		}
	case map[string]any:
		if pr, ok := x["pull_request"].(domain.PullRequest); ok {
			PullRequest(w, pr)
			if completion, ok := x["completion"].(domain.CompletionRecord); ok {
				Completion(w, completion)
			}
			if issue, ok := x["issue"].(domain.Issue); ok {
				fmt.Fprintf(w, "Issue status: %s\n", issue.Status)
			}
			return
		}
		Fields(w, x)
	case map[string]string:
		fields := make(map[string]any, len(x))
		for k, v := range x {
			fields[k] = v
		}
		Fields(w, fields)
	default:
		JSON(w, v)
	}
}

// Fields prints a plain key/value result in a stable order.
func Fields(w io.Writer, m map[string]any) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(w, "%s: %v\n", k, m[k])
	}
}

// JSON writes v as indented JSON. It is also the fallback for values without a
// human form, where a correct machine-readable dump beats an empty line.
func JSON(w io.Writer, v any) {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

// Issue writes the detail block for one Issue.
func Issue(w io.Writer, x domain.Issue) {
	fmt.Fprintf(w, "%s#%d — %s\n", x.RepositoryName, x.Number, x.Title)
	fmt.Fprintf(w, "  status: %s", x.Status)
	if x.Blocked {
		fmt.Fprint(w, " (blocked)")
	}
	fmt.Fprintf(w, "\n  id: %s\n", x.ID)
	if x.Description != "" {
		fmt.Fprintf(w, "  description: %s\n", x.Description)
	}
	if x.ParentReference != "" {
		fmt.Fprintf(w, "  parent: %s\n", x.ParentReference)
	}
	if x.Priority != "" {
		fmt.Fprintf(w, "  priority: %s\n", x.Priority)
	}
	if x.DueDate != nil {
		fmt.Fprintf(w, "  due: %s\n", *x.DueDate)
	}
	for _, b := range x.Blockers {
		fmt.Fprintf(w, "  blocker: %s#%d %q (%s)\n", b.RepositoryName, b.Number, b.Title, b.Status)
	}
}

// PullRequest writes the detail block for one Pull Request.
func PullRequest(w io.Writer, x domain.PullRequest) {
	fmt.Fprintf(w, "%s!%d — %s\n  status: %s\n  issue: %s#%d\n  target: %s\n  id: %s\n", x.RepositoryName, x.Number, x.Title, x.Status, x.RepositoryName, x.IssueNumber, x.TargetBranchName, x.ID)
	if x.Description != "" {
		fmt.Fprintf(w, "  description: %s\n", x.Description)
	}
}

// Completion writes one logical completion record.
func Completion(w io.Writer, x domain.CompletionRecord) {
	fmt.Fprintf(w, "Completion %s/%s/%d\n  issue: %s#%d\n  summary: %s\n", x.RepositoryName, x.BranchName, x.Sequence, x.RepositoryName, x.IssueNumber, x.Summary)
}

// Graph writes nodes and edges, resolving edge endpoints to their references.
func Graph(w io.Writer, g domain.Graph) {
	fmt.Fprintf(w, "Graph: %d nodes · %d edges\n", len(g.Nodes), len(g.Edges))
	byID := map[string]domain.GraphNode{}
	for _, n := range g.Nodes {
		byID[n.ID] = n
		fmt.Fprintf(w, "  [%s] %s  <%s>\n", n.Type, n.Reference, n.Status)
	}
	fmt.Fprintln(w, "Edges:")
	for _, e := range g.Edges {
		source, target := e.Source, e.Target
		if a, ok := byID[e.Source]; ok {
			source = a.Reference
		}
		if b, ok := byID[e.Target]; ok {
			target = b.Reference
		}
		fmt.Fprintf(w, "  %s --%s--> %s\n", source, e.Type, target)
	}
}
