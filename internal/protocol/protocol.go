// Package protocol defines IDIOS's versioned, client-neutral workspace action contract.
// The Web client, TUI command palette, and future adapters consume the same catalog and
// dispatch mutations through Application so workflow rules are never reimplemented in UI code.
package protocol

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"idios/internal/application"
	"idios/internal/domain"
	"idios/internal/reminders"
	"idios/internal/validation"
)

const Version = 1

type Field struct {
	Name        string   `json:"name"`
	Label       string   `json:"label"`
	Type        string   `json:"type"` // text, textarea, date, datetime, number, select, boolean
	Required    bool     `json:"required,omitempty"`
	Placeholder string   `json:"placeholder,omitempty"`
	Help        string   `json:"help,omitempty"`
	Default     any      `json:"default,omitempty"`
	Options     []string `json:"options,omitempty"`
	Source      string   `json:"source,omitempty"` // dynamic options source, resolved by this protocol
}

type Command struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Category    string  `json:"category"`
	Description string  `json:"description"`
	Fields      []Field `json:"fields,omitempty"`
}

type handler func(context.Context, *application.Service, map[string]any) (any, error)
type commandSpec struct {
	Command
	run handler
}

func field(name, label, typ string, required bool, placeholder, help string, def any, options ...string) Field {
	return Field{Name: name, Label: label, Type: typ, Required: required, Placeholder: placeholder, Help: help, Default: def, Options: options}
}

var specs = []commandSpec{
	{Command: Command{ID: "repository.create", Title: "Create repository", Category: "Repositories", Description: "Create a local repository workspace and its initial workflow branch.", Fields: []Field{field("name", "Name", "text", true, "XORON5", "Used in references such as XORON5#4.", nil), field("description", "Description", "textarea", false, "What is this repository for?", "Short workspace overview.", nil), field("default_branch", "Initial branch", "text", false, "main", "Name of the initial IDIOS workflow branch.", "main")}}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		return s.CreateRepositoryWithDefaultBranch(c, str(v, "name"), str(v, "description"), str(v, "default_branch"))
	}},
	{Command: Command{ID: "branch.create", Title: "Create branch", Category: "Repositories", Description: "Create an IDIOS workflow branch based on another branch in the same repository. This does not run Git or create filesystem refs.", Fields: []Field{field("repository", "Repository", "select", true, "", "Repository that owns this branch.", nil), field("name", "Branch name", "text", true, "feature/short-description", "A unique name within this repository.", nil), field("base_branch", "Base branch", "select", false, "", "Branch this branch is based on.", nil)}}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		return s.CreateBranch(c, str(v, "repository"), str(v, "name"), str(v, "base_branch"))
	}},
	{Command: Command{ID: "repository.delete", Title: "Delete repository", Category: "Repositories", Description: "Delete a local repository and its stored records. Choose the intended workspace by name or ID.", Fields: []Field{field("ref", "Repository name or ID", "text", true, "XORON5", "Select the local workspace to remove.", nil)}}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		err := s.DeleteRepository(c, str(v, "ref"))
		return map[string]any{"deleted": err == nil, "ref": str(v, "ref")}, err
	}},
	{Command: Command{ID: "repository.description.update", Title: "Update repository description", Category: "Repositories", Description: "Change the short description shown on this local repository page.", Fields: []Field{field("repository", "Repository", "select", true, "", "Choose a repository.", nil), field("description", "Description", "textarea", false, "What is this repository for?", "Short repository overview.", nil)}}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		return s.UpdateRepositoryDescription(c, str(v, "repository"), str(v, "description"))
	}},
	{Command: Command{ID: "project.create", Title: "Create project", Category: "Projects", Description: "Create a project to group related issues.", Fields: []Field{field("name", "Name", "text", true, "Release planning", "Project name.", nil), field("description", "Description", "textarea", false, "Project goal…", "What does success look like?", nil)}}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		return s.CreateProject(c, str(v, "name"), str(v, "description"))
	}},
	{Command: Command{ID: "project.delete", Title: "Delete project", Category: "Projects", Description: "Delete a project record.", Fields: []Field{field("ref", "Project name or ID", "text", true, "Release planning", "", nil)}}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		err := s.DeleteProject(c, str(v, "ref"))
		return map[string]any{"deleted": err == nil}, err
	}},
	{Command: Command{ID: "project.add_issue", Title: "Add issue to project", Category: "Projects", Description: "Link an existing issue to a project.", Fields: []Field{field("project", "Project", "text", true, "Release planning", "Project name or ID.", nil), field("issue", "Issue reference", "text", true, "XORON5#12", "Issue reference or stable ID.", nil)}}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		err := s.AddIssueToProject(c, str(v, "project"), str(v, "issue"))
		return map[string]any{"added": err == nil}, err
	}},
	{Command: Command{ID: "project.repository.link", Title: "Link repository to project", Category: "Projects", Description: "Associate a repository with a project, describe its role, and optionally mark it as primary.", Fields: []Field{field("project", "Project", "text", true, "XORON5", "Project name or ID.", nil), field("repository", "Repository", "text", true, "386bsd", "Repository name or ID.", nil), field("relationship", "Role in project", "text", false, "Upstream reference", "Short note explaining how this repository relates to the project.", nil), field("primary", "Primary repository", "boolean", false, "", "Mark this as the main implementation repository.", false)}}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		err := s.LinkRepositoryToProject(c, str(v, "project"), str(v, "repository"), str(v, "relationship"), boolean(v, "primary"))
		return map[string]any{"linked": err == nil}, err
	}},
	{Command: Command{ID: "project.repository.unlink", Title: "Unlink repository from project", Category: "Projects", Description: "Remove a repository association without deleting either record.", Fields: []Field{field("project", "Project", "text", true, "XORON5", "Project name or ID.", nil), field("repository", "Repository", "text", true, "386bsd", "Repository name or ID.", nil)}}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		err := s.UnlinkRepositoryFromProject(c, str(v, "project"), str(v, "repository"))
		return map[string]any{"unlinked": err == nil}, err
	}},
	{Command: Command{ID: "project.repository.primary", Title: "Set primary project repository", Category: "Projects", Description: "Choose the linked repository that contains the main implementation.", Fields: []Field{field("project", "Project", "text", true, "XORON5", "Project name or ID.", nil), field("repository", "Repository", "text", true, "xoron5", "The repository must already be linked to the project.", nil)}}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		err := s.SetPrimaryProjectRepository(c, str(v, "project"), str(v, "repository"))
		return map[string]any{"primary": err == nil}, err
	}},
	{Command: Command{ID: "issue.create", Title: "Create issue", Category: "Issues", Description: "Create an issue with optional parent, due date, and priority.", Fields: []Field{field("repo", "Repository", "text", true, "XORON5", "Repository name or ID.", nil), field("title", "Title", "text", true, "Implement settings page", "", nil), field("description", "Description", "textarea", false, "Context, acceptance criteria…", "", nil), field("type", "Issue type", "select", false, "task", "Task, bug, feature or research; empty defaults to task.", "task", "task", "bug", "feature", "research"), field("parent", "Parent issue", "text", false, "XORON5#4", "Parent issues must belong to the same repository.", nil), field("due", "Due date", "date", false, "", "YYYY-MM-DD.", nil), field("priority", "Priority", "select", false, "normal", "", "normal", "low", "normal", "high", "urgent"), field("branch", "Branch", "select", false, "", "Optional branch to track this issue on.", nil)}}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		return s.CreateIssueWithTypeAndBranch(c, str(v, "repo"), str(v, "title"), str(v, "description"), str(v, "parent"), str(v, "due"), str(v, "priority"), str(v, "branch"), str(v, "type"))
	}},
	{Command: Command{ID: "issue.start", Title: "Start issue", Category: "Issues", Description: "Start an issue when all blocking prerequisites are complete.", Fields: []Field{field("ref", "Issue reference", "text", true, "XORON5#4", "The Engine will reject blocked work.", nil)}}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		return s.StartIssue(c, str(v, "ref"))
	}},
	{Command: Command{ID: "issue.cancel", Title: "Cancel issue", Category: "Issues", Description: "Mark an issue cancelled.", Fields: []Field{field("ref", "Issue reference", "text", true, "XORON5#4", "", nil)}}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		return s.CancelIssue(c, str(v, "ref"))
	}},
	{Command: Command{ID: "issue.branch.assign", Title: "Assign issue to branch", Category: "Issues", Description: "Associate an issue with a workflow branch from the same repository. Leave branch empty to clear it.", Fields: []Field{field("issue", "Issue", "select", true, "", "Issue to update.", nil), field("branch", "Branch", "select", false, "", "Only branches from the issue's repository are accepted.", nil)}}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		err := s.AssignIssueBranch(c, str(v, "issue"), str(v, "branch"))
		return map[string]any{"assigned": err == nil}, err
	}},
	{Command: Command{ID: "issue.parent.assign", Title: "Set parent issue", Category: "Issues", Description: "Change an issue's parent/sub-issue relationship. The parent must be open and in the same repository; cycles are rejected.", Fields: []Field{field("issue", "Sub-issue", "select", true, "", "Issue whose parent will change.", nil), field("parent", "Parent issue", "select", false, "", "Leave empty to remove the parent.", nil)}}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		err := s.AssignIssueParent(c, str(v, "issue"), str(v, "parent"))
		return map[string]any{"assigned": err == nil}, err
	}},
	{Command: Command{ID: "issue.delete", Title: "Delete issue", Category: "Issues", Description: "Delete an issue. The Engine validates relationships and constraints.", Fields: []Field{field("ref", "Issue reference", "text", true, "XORON5#4", "", nil)}}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		err := s.DeleteIssue(c, str(v, "ref"))
		return map[string]any{"deleted": err == nil}, err
	}},
	{Command: Command{ID: "issue.dependency.add", Title: "Add blocking dependency", Category: "Issues", Description: "Make one issue wait for a prerequisite issue.", Fields: []Field{field("issue", "Blocked issue", "select", true, "", "The issue that depends on the prerequisite.", nil), field("prerequisite", "Prerequisite issue", "select", true, "", "Only depends_on blocks work.", nil)}}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		err := s.AddDependency(c, str(v, "issue"), str(v, "prerequisite"))
		return map[string]any{"added": err == nil}, err
	}},
	{Command: Command{ID: "issue.dependency.remove", Title: "Remove blocking dependency", Category: "Issues", Description: "Remove a prerequisite relationship.", Fields: []Field{field("issue", "Issue", "select", true, "", "", nil), field("prerequisite", "Prerequisite issue", "select", true, "", "", nil)}}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		err := s.RemoveDependency(c, str(v, "issue"), str(v, "prerequisite"))
		return map[string]any{"removed": err == nil}, err
	}},
	{Command: Command{ID: "issue.concept.link", Title: "Link issue to concept", Category: "Issues", Description: "Add a semantic concept link; this does not block work.", Fields: []Field{field("issue", "Issue", "select", true, "", "", nil), field("concept", "Concept", "select", true, "", "Choose a concept.", nil)}}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		err := s.LinkIssueConcept(c, str(v, "issue"), str(v, "concept"))
		return map[string]any{"linked": err == nil}, err
	}},
	{Command: Command{ID: "issue.label.assign", Title: "Assign label", Category: "Issues", Description: "Assign an existing label to an issue.", Fields: []Field{field("issue", "Issue", "select", true, "", "Choose an issue.", nil), field("label", "Label", "select", true, "", "Choose a label.", nil)}}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		err := s.AssignLabel(c, str(v, "issue"), str(v, "label"))
		return map[string]any{"assigned": err == nil}, err
	}},
	{Command: Command{ID: "issue.milestone.assign", Title: "Assign milestone", Category: "Issues", Description: "Assign an existing milestone to an issue.", Fields: []Field{field("issue", "Issue", "select", true, "", "Choose an issue.", nil), field("milestone", "Milestone", "select", true, "", "Choose a milestone.", nil)}}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		err := s.AssignMilestone(c, str(v, "issue"), str(v, "milestone"))
		return map[string]any{"assigned": err == nil}, err
	}},
	{Command: Command{ID: "pull_request.create", Title: "Create pull request", Category: "Pull requests", Description: "Create a simulated review record covering one or more in-progress issues. Accepting it creates one IDIOS commit and completes all included issues; it does not run Git.", Fields: []Field{field("issue", "Primary issue", "select", true, "", "Primary issue shown in the repository history; use Additional issues to include more work in the same simulated merge.", nil), field("additional_issues", "Additional issues", "text", false, "XORON#4, XORON#9", "Comma-separated issue references included in the same simulated merge.", nil), field("title", "Title", "text", false, "", "Defaults from the selected issue(s).", nil), field("description", "Work summary", "textarea", true, "What changed and how was it verified?", "A clear completion explanation is required.", nil), field("source_branch", "Source branch", "select", false, "", "Optional branch containing the work.", nil), field("target_branch", "Target branch", "select", false, "", "Defaults to the repository default branch.", nil), field("draft", "Draft pull request", "boolean", false, "", "Keep this as a draft while work is still being prepared.", false)}}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		issues := []string{str(v, "issue")}
		issues = append(issues, split(str(v, "additional_issues"))...)
		return s.CreatePullRequestWithIssues(c, issues, str(v, "title"), str(v, "description"), str(v, "source_branch"), str(v, "target_branch"), boolean(v, "draft"))
	}},
	{Command: Command{ID: "pull_request.edit", Title: "Edit pull request", Category: "Pull requests", Description: "Update an open pull request title or description.", Fields: []Field{field("ref", "Pull request reference", "text", true, "XORON5!2", "", nil), field("title", "Title", "text", false, "", "Leave empty to preserve where supported.", nil), field("description", "Description", "textarea", false, "", "", nil)}}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		return s.UpdatePullRequest(c, str(v, "ref"), str(v, "title"), str(v, "description"))
	}},
	{Command: Command{ID: "pull_request.accept", Title: "Accept pull request", Category: "Pull requests", Description: "Merge a simulated PR into its target branch. Records one commit and atomically completes every linked issue; no Git command is run.", Fields: []Field{field("ref", "Pull request reference", "text", true, "XORON5!2", "Merge completes every issue linked to this PR and adds exactly one simulated commit.", nil), field("summary", "Completion summary", "textarea", false, "Implemented and verified…", "", nil)}}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		p, completion, issue, err := s.AcceptPullRequest(c, str(v, "ref"), str(v, "summary"))
		return map[string]any{"pull_request": p, "completion": completion, "issue": issue}, err
	}},
	{Command: Command{ID: "pull_request.mark_ready", Title: "Mark pull request ready", Category: "Pull requests", Description: "Convert an open draft pull request into a normal review request.", Fields: []Field{field("ref", "Pull request reference", "select", true, "", "Choose the draft pull request.", nil)}}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		return s.MarkPullRequestReady(c, str(v, "ref"))
	}},
	{Command: Command{ID: "pull_request.close", Title: "Close pull request", Category: "Pull requests", Description: "Close an open pull request without accepting it.", Fields: []Field{field("ref", "Pull request reference", "text", true, "XORON5!2", "", nil)}}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		return s.ClosePullRequest(c, str(v, "ref"))
	}},
	{Command: Command{ID: "pull_request.delete", Title: "Delete pull request", Category: "Pull requests", Description: "Delete a pull request record subject to Engine constraints.", Fields: []Field{field("ref", "Pull request reference", "text", true, "XORON5!2", "", nil)}}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		err := s.DeletePullRequest(c, str(v, "ref"))
		return map[string]any{"deleted": err == nil}, err
	}},
	{Command: Command{ID: "concept.create", Title: "Create concept", Category: "Knowledge", Description: "Create a reusable knowledge concept.", Fields: []Field{field("name", "Name", "text", true, "Concurrency", "", nil), field("description", "Description", "textarea", false, "A short explanation…", "", nil)}}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		return s.CreateConcept(c, str(v, "name"), str(v, "description"))
	}},
	{Command: Command{ID: "concept.delete", Title: "Delete concept", Category: "Knowledge", Description: "Delete a concept record.", Fields: []Field{field("ref", "Concept name or ID", "text", true, "Concurrency", "", nil)}}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		err := s.DeleteConcept(c, str(v, "ref"))
		return map[string]any{"deleted": err == nil}, err
	}},
	{Command: Command{ID: "relation.create", Title: "Create knowledge relation", Category: "Knowledge", Description: "Connect two graph entities with a typed semantic relationship.", Fields: []Field{field("source_type", "Source type", "select", true, "concept", "", "concept", "repository", "issue", "pull_request", "concept", "project", "note", "label", "milestone", "branch", "completion"), field("source", "Source reference", "text", true, "concept:Concurrency", "", nil), field("target_type", "Target type", "select", true, "issue", "", "issue", "repository", "issue", "pull_request", "concept", "project", "note", "label", "milestone", "branch", "completion"), field("target", "Target reference", "text", true, "XORON5#4", "", nil), field("kind", "Relation type", "text", true, "related_to", "Relationship type.", "related_to")}}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		return s.CreateRelation(c, str(v, "source_type"), str(v, "source"), str(v, "target_type"), str(v, "target"), str(v, "kind"))
	}},
	{Command: Command{ID: "label.create", Title: "Create label", Category: "Planning", Description: "Create a global or repository-scoped label.", Fields: []Field{field("name", "Name", "text", true, "bug", "", nil), field("scope", "Scope", "select", true, "global", "", "global", "global", "repository"), field("repository", "Repository", "text", false, "XORON5", "Required for repository scope.", nil), field("description", "Description", "textarea", false, "", "", nil), field("color", "Color", "color", false, "#7c9cff", "Label color.", "#7c9cff")}}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		return s.CreateLabel(c, str(v, "name"), str(v, "scope"), str(v, "repository"), str(v, "description"), str(v, "color"))
	}},
	{Command: Command{ID: "label.delete", Title: "Delete label", Category: "Planning", Description: "Delete a label.", Fields: []Field{field("ref", "Label name or ID", "text", true, "bug", "", nil)}}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		err := s.DeleteLabel(c, str(v, "ref"))
		return map[string]any{"deleted": err == nil}, err
	}},
	{Command: Command{ID: "milestone.create", Title: "Create milestone", Category: "Planning", Description: "Create a repository- or project-scoped milestone.", Fields: []Field{field("scope", "Scope", "select", true, "repository", "", "repository", "repository", "project"), field("owner", "Repository or project", "text", true, "XORON5", "Owner reference.", nil), field("title", "Title", "text", true, "v1.0", "", nil), field("description", "Description", "textarea", false, "", "", nil), field("due", "Due date", "date", false, "", "YYYY-MM-DD.", nil)}}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		return s.CreateMilestone(c, str(v, "scope"), str(v, "owner"), str(v, "title"), str(v, "description"), str(v, "due"))
	}},
	{Command: Command{ID: "milestone.delete", Title: "Delete milestone", Category: "Planning", Description: "Delete a milestone.", Fields: []Field{field("ref", "Milestone title or ID", "text", true, "v1.0", "", nil)}}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		err := s.DeleteMilestone(c, str(v, "ref"))
		return map[string]any{"deleted": err == nil}, err
	}},
	{Command: Command{ID: "note.label.color", Title: "Set note label color", Category: "Notes", Description: "Choose a color for a reusable note label.", Fields: []Field{field("name", "Label", "text", true, "research", "Exact label name.", nil), field("color", "Color", "color", true, "#1f6feb", "Six-digit hexadecimal color.", nil)}}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		err := s.SetNoteLabelColor(c, str(v, "name"), str(v, "color"))
		return map[string]any{"updated": err == nil}, err
	}},
	{Command: Command{ID: "note.create", Title: "Create note", Category: "Notes", Description: "Capture a note and optional comma-separated labels.", Fields: []Field{field("title", "Title", "text", true, "Architecture decisions", "", nil), field("content", "Content", "textarea", false, "Write a note…", "", nil), field("labels", "Labels", "text", false, "design, backend", "Comma-separated labels.", nil)}}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		return s.CreateNoteWithLabels(c, str(v, "title"), str(v, "content"), split(str(v, "labels")))
	}},
	{Command: Command{ID: "note.update", Title: "Update note", Category: "Notes", Description: "Edit a note and replace its labels.", Fields: []Field{field("id", "Note ID", "text", true, "note-id", "", nil), field("title", "Title", "text", true, "Architecture decisions", "", nil), field("content", "Content", "textarea", false, "", "", nil), field("labels", "Labels", "text", false, "design, backend", "Comma-separated labels.", nil)}}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		return s.UpdateNoteWithLabels(c, str(v, "id"), str(v, "title"), str(v, "content"), split(str(v, "labels")))
	}},
	{Command: Command{ID: "note.delete", Title: "Delete note", Category: "Notes", Description: "Delete a note.", Fields: []Field{field("id", "Note ID", "text", true, "note-id", "", nil)}}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		err := s.DeleteNote(c, str(v, "id"))
		return map[string]any{"deleted": err == nil}, err
	}},
	{Command: Command{ID: "note.link", Title: "Link note to entity", Category: "Notes", Description: "Connect a note to an entity; optionally include it in the graph.", Fields: []Field{field("id", "Note ID", "text", true, "note-id", "", nil), field("entity_type", "Entity type", "select", true, "issue", "", "issue", "repository", "issue", "pull_request", "concept", "project", "label", "milestone", "branch", "completion"), field("entity_ref", "Entity reference", "text", true, "XORON5#4", "", nil), field("graph_visible", "Visible in graph", "boolean", false, "", "", false)}}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		err := s.LinkNote(c, str(v, "id"), str(v, "entity_type"), str(v, "entity_ref"), boolean(v, "graph_visible"))
		return map[string]any{"linked": err == nil}, err
	}},
	{Command: Command{ID: "quote.create", Title: "Save quote", Category: "Quotes", Description: "Save a quote with an optional author.", Fields: []Field{field("text", "Quote", "textarea", true, "A memorable line…", "", nil), field("author", "Author", "text", false, "Author name", "", nil), field("source", "Source", "text", false, "Book, article, or talk", "", nil), field("source_url", "Source URL", "text", false, "https://…", "", nil), field("category", "Category", "text", false, "Reading", "", nil)}}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		return s.CreateQuoteWithSource(c, domain.Quote{Text: str(v, "text"), Author: str(v, "author"), Source: str(v, "source"), SourceURL: str(v, "source_url"), Category: str(v, "category")})
	}},
	{Command: Command{ID: "quote.delete", Title: "Delete quote", Category: "Quotes", Description: "Delete a saved quote.", Fields: []Field{field("id", "Quote ID", "text", true, "quote-id", "", nil)}}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		err := s.DeleteQuote(c, str(v, "id"))
		return map[string]any{"deleted": err == nil}, err
	}},
	{Command: Command{ID: "quote.reminder.test", Title: "Send test quote reminder", Category: "Quotes", Description: "Choose a quote and save a test reminder to the shared inbox.", Fields: nil}, run: func(c context.Context, s *application.Service, _ map[string]any) (any, error) {
		items, err := s.ListQuotes(c)
		if err != nil {
			return nil, err
		}
		if len(items) == 0 {
			return nil, fmt.Errorf("the quote library is empty; add a quote before testing reminders")
		}
		item := reminders.Choose(items, "")
		notice, err := s.CreateNotification(c, "Quote reminder test", reminders.FormatText(item), "quote")
		if err != nil {
			return nil, err
		}
		return map[string]any{"quote": item, "notification": notice}, nil
	}},
	{Command: Command{ID: "job.create", Title: "Create scheduled job", Category: "Automation", Description: "Schedule a built-in workspace reminder. Configure recurrence and notification preference.", Fields: []Field{field("name", "Name", "text", true, "Weekly review", "", nil), field("description", "Description", "textarea", false, "", "", nil), field("action_type", "Action", "select", true, "weekly-review", "", "weekly-review", "daily-plan", "weekly-review", "blocked-issues", "open-issues", "custom-reminder"), field("schedule_type", "Schedule", "select", true, "weekly", "", "weekly", "interval", "once", "daily", "weekly", "monthly"), field("interval_minutes", "Interval in minutes", "number", false, "60", "Used for interval schedules.", 60), field("schedule_at", "One-time date and time", "datetime", false, "", "Used for once schedules; local time.", nil), field("schedule_time", "Time", "text", false, "09:00", "Local time HH:MM.", "09:00"), field("schedule_weekday", "Weekday (0–6)", "number", false, "1", "0=Sunday … 6=Saturday.", 1), field("schedule_day", "Day of month", "number", false, "1", "Used for monthly schedules.", 1), field("enabled", "Enabled", "boolean", false, "", "", true), field("notify", "Create notification", "boolean", false, "", "", true), field("branch", "Related branch", "select", false, "", "Optional branch this scheduled job supports.", nil)}}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		job, err := jobFromInput(v, "")
		if err != nil {
			return nil, err
		}
		return s.CreateScheduledJob(c, job)
	}},
	{Command: Command{ID: "job.update", Title: "Update scheduled job", Category: "Automation", Description: "Update an existing scheduled job.", Fields: []Field{field("id", "Job ID", "text", true, "job-id", "", nil), field("name", "Name", "text", true, "Weekly review", "", nil), field("description", "Description", "textarea", false, "", "", nil), field("action_type", "Action", "select", true, "weekly-review", "", "weekly-review", "daily-plan", "weekly-review", "blocked-issues", "open-issues", "custom-reminder"), field("schedule_type", "Schedule", "select", true, "weekly", "", "weekly", "interval", "once", "daily", "weekly", "monthly"), field("interval_minutes", "Interval in minutes", "number", false, "60", "", 60), field("schedule_at", "One-time date and time", "datetime", false, "", "Used for once schedules; local time.", nil), field("schedule_time", "Time", "text", false, "09:00", "Local time HH:MM.", "09:00"), field("schedule_weekday", "Weekday (0–6)", "number", false, "1", "", 1), field("schedule_day", "Day of month", "number", false, "1", "", 1), field("enabled", "Enabled", "boolean", false, "", "", true), field("notify", "Create notification", "boolean", false, "", "", true), field("branch", "Related branch", "select", false, "", "Optional branch this scheduled job supports.", nil)}}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		job, err := jobFromInput(v, str(v, "id"))
		if err != nil {
			return nil, err
		}
		return s.UpdateScheduledJob(c, job)
	}},
	{Command: Command{ID: "job.toggle", Title: "Toggle scheduled job", Category: "Automation", Description: "Enable or pause a scheduled job.", Fields: []Field{field("id", "Job ID", "text", true, "job-id", "", nil)}}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		return s.ToggleScheduledJob(c, str(v, "id"))
	}},
	{Command: Command{ID: "job.run", Title: "Run scheduled job now", Category: "Automation", Description: "Execute a job immediately and optionally create a notification.", Fields: []Field{field("id", "Job ID", "text", true, "job-id", "", nil), field("notify", "Send a notification", "boolean", false, "", "Honor the caller's notification preference.", true)}}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		notify := true // preserve the original protocol behavior for older clients
		if _, exists := v["notify"]; exists {
			notify = boolean(v, "notify")
		}
		return map[string]any{"ran": true}, s.RunScheduledJobNow(c, str(v, "id"), time.Now(), notify)
	}},
	{Command: Command{ID: "job.delete", Title: "Delete scheduled job", Category: "Automation", Description: "Delete a scheduled job.", Fields: []Field{field("id", "Job ID", "text", true, "job-id", "", nil)}}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		err := s.DeleteScheduledJob(c, str(v, "id"))
		return map[string]any{"deleted": err == nil}, err
	}},
	{Command: Command{ID: "workspace.profile.update", Title: "Update workspace profile", Category: "Workspace", Description: "Update the workspace name, bio, and README content.", Fields: []Field{field("username", "Username", "text", false, "you", "", nil), field("full_name", "Display name", "text", false, "Personal workspace", "", nil), field("focus", "Current focus", "text", false, "Kernel design", "What are you working on right now?", nil), field("topics", "Topics and tools", "text", false, "Go, C, systems", "A short comma-separated list that helps summarize this workspace.", nil), field("website", "Project or profile URL", "text", false, "https://…", "Optional link related to your work.", nil), field("bio", "Bio", "textarea", false, "", "", nil), field("readme", "Workspace README", "textarea", false, "", "", nil)}}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		return s.UpdateWorkspaceProfile(c, domain.WorkspaceProfile{Username: str(v, "username"), FullName: str(v, "full_name"), Focus: str(v, "focus"), Topics: str(v, "topics"), Website: str(v, "website"), Bio: str(v, "bio"), Readme: str(v, "readme")})
	}},
	{Command: Command{ID: "workspace.pin.toggle", Title: "Toggle workspace pin", Category: "Workspace", Description: "Pin or unpin an issue, project, repository, concept, or note.", Fields: []Field{field("entity_type", "Entity type", "select", true, "issue", "", "issue", "repository", "issue", "pull_request", "project", "concept", "note", "milestone", "label"), field("entity_ref", "Entity ID or reference", "text", true, "XORON5#4", "", nil), field("label", "Display label", "text", false, "Important", "", nil), field("color", "Color token", "text", false, "accent", "", "accent")}}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		pinned, err := s.ToggleWorkspacePin(c, str(v, "entity_type"), str(v, "entity_ref"), str(v, "label"), str(v, "color"))
		return map[string]any{"pinned": pinned}, err
	}},
	{Command: Command{ID: "notification.read_all", Title: "Mark notifications read", Category: "Notifications", Description: "Mark all notifications as read.", Fields: nil}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		err := s.MarkNotificationsRead(c)
		return map[string]any{"read": err == nil}, err
	}},
	{Command: Command{ID: "notification.create", Title: "Create notification", Category: "Notifications", Description: "Create a local inbox notification.", Fields: []Field{field("title", "Title", "text", true, "Reminder", "", nil), field("body", "Message", "textarea", false, "", "", nil), field("level", "Level", "select", true, "info", "", "info", "info", "warning", "error")}}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		return s.CreateNotification(c, str(v, "title"), str(v, "body"), str(v, "level"))
	}},
	{Command: Command{ID: "notification.delete", Title: "Delete notification", Category: "Notifications", Description: "Delete a notification from the inbox.", Fields: []Field{field("id", "Notification ID", "text", true, "notification-id", "", nil)}}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		err := s.DeleteNotification(c, str(v, "id"))
		return map[string]any{"deleted": err == nil}, err
	}},
	{Command: Command{ID: "issue.bulk_create", Title: "Bulk create issues", Category: "Issues", Description: "Create up to 100 issues with a shared description and priority. One non-empty title per line.", Fields: []Field{field("repo", "Repository", "text", true, "XORON5", "Repository name or ID.", nil), field("titles", "Issue titles", "textarea", true, "First issue\nSecond issue", "One issue title per line; up to 100.", nil), field("description", "Shared description", "textarea", false, "Shared context…", "Applied to every issue.", nil), field("priority", "Priority", "select", true, "normal", "Applied to every issue.", "normal", "low", "normal", "high", "urgent")}}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		items, err := lines(str(v, "titles"), 100)
		if err != nil {
			return nil, err
		}
		if len(items) == 0 {
			return nil, fmt.Errorf("enter at least one issue title")
		}
		if _, err := s.GetRepository(c, str(v, "repo")); err != nil {
			return nil, err
		}
		created := make([]domain.Issue, 0, len(items))
		failed := make([]map[string]string, 0)
		for _, title := range items {
			item, createErr := s.CreateIssue(c, str(v, "repo"), title, str(v, "description"), "", "", str(v, "priority"))
			if createErr != nil {
				failed = append(failed, map[string]string{"item": title, "error": createErr.Error()})
				continue
			}
			created = append(created, item)
		}
		return map[string]any{"created": len(created), "total": len(items), "failed": failed, "items": created}, nil
	}},
	{Command: Command{ID: "quote.bulk_create", Title: "Bulk save quotes", Category: "Quotes", Description: "Save up to 100 quotes. One non-empty quote per line with optional shared attribution.", Fields: []Field{field("quotes", "Quotes", "textarea", true, "One quote per line", "Up to 100 lines.", nil), field("author", "Shared author", "text", false, "Author name", "Applied to every quote.", nil), field("source", "Shared source", "text", false, "Book or article", "Optional citation.", nil)}}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		items, err := lines(str(v, "quotes"), 100)
		if err != nil {
			return nil, err
		}
		if len(items) == 0 {
			return nil, fmt.Errorf("enter at least one quote")
		}
		created := make([]domain.Quote, 0, len(items))
		failed := make([]map[string]string, 0)
		for _, quote := range items {
			item, createErr := s.CreateQuoteWithSource(c, domain.Quote{Text: quote, Author: str(v, "author"), Source: str(v, "source")})
			if createErr != nil {
				failed = append(failed, map[string]string{"item": quote, "error": createErr.Error()})
				continue
			}
			created = append(created, item)
		}
		return map[string]any{"created": len(created), "total": len(items), "failed": failed, "items": created}, nil
	}},
	{Command: Command{ID: "job.bulk_create", Title: "Bulk create scheduled jobs", Category: "Automation", Description: "Create up to 100 schedules with shared recurrence and reminder type.", Fields: []Field{field("names", "Job names", "textarea", true, "Weekly review\nPlan tomorrow", "One job name per line; up to 100.", nil), field("description", "Shared reminder note", "textarea", false, "", "Applied to every job.", nil), field("action_type", "Reminder kind", "select", true, "custom-reminder", "Applied to every job.", "custom-reminder", "daily-plan", "weekly-review", "blocked-issues", "open-issues", "custom-reminder"), field("schedule_type", "Schedule", "select", true, "weekly", "Shared schedule.", "weekly", "interval", "once", "daily", "weekly", "monthly"), field("interval_minutes", "Interval in minutes", "number", false, "60", "For interval schedules.", 60), field("schedule_at", "One-time date and time", "datetime", false, "", "For once schedules, local time.", nil), field("schedule_time", "Time", "text", false, "09:00", "For daily, weekly, and monthly schedules.", "09:00"), field("schedule_weekday", "Weekday (0–6)", "number", false, "1", "0=Sunday … 6=Saturday.", 1), field("schedule_day", "Day of month", "number", false, "1", "For monthly schedules.", 1), field("notify", "Send notifications", "boolean", false, "", "Global notifications must also be enabled.", true)}}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) {
		items, err := lines(str(v, "names"), 100)
		if err != nil {
			return nil, err
		}
		if len(items) == 0 {
			return nil, fmt.Errorf("enter at least one job name")
		}
		created := make([]domain.ScheduledJob, 0, len(items))
		failed := make([]map[string]string, 0)
		for _, name := range items {
			input := map[string]any{"id": "", "name": name, "description": str(v, "description"), "action_type": str(v, "action_type"), "schedule_type": str(v, "schedule_type"), "interval_minutes": v["interval_minutes"], "schedule_at": str(v, "schedule_at"), "schedule_time": str(v, "schedule_time"), "schedule_weekday": v["schedule_weekday"], "schedule_day": v["schedule_day"], "notify": boolean(v, "notify"), "enabled": true}
			job, inputErr := jobFromInput(input, "")
			if inputErr != nil {
				return nil, inputErr
			}
			item, createErr := s.CreateScheduledJob(c, job)
			if createErr != nil {
				failed = append(failed, map[string]string{"item": name, "error": createErr.Error()})
				continue
			}
			created = append(created, item)
		}
		return map[string]any{"created": len(created), "total": len(items), "failed": failed, "items": created}, nil
	}},
	{Command: Command{ID: "demo.seed", Title: "Load demo workspace", Category: "Workspace", Description: "Create sample records for exploring IDIOS.", Fields: nil}, run: func(c context.Context, s *application.Service, v map[string]any) (any, error) { return s.SeedDemo(c) }},
}

type Option struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

func metadata(command Command) Command {
	out := command
	out.Fields = append([]Field(nil), command.Fields...)
	for i := range out.Fields {
		out.Fields[i].Options = append([]string(nil), out.Fields[i].Options...)
		if out.Fields[i].Source == "" {
			out.Fields[i].Source = fieldSource(command.ID, out.Fields[i].Name)
		}
	}
	return out
}

func fieldSource(commandID, name string) string {
	if commandID == "pull_request.create" && name == "issue" {
		return "pr_issues"
	}
	switch name {
	case "repo":
		return "repositories"
	case "branch", "base_branch", "source_branch", "target_branch":
		return "branches"
	case "repository":
		if commandID == "branch.create" || commandID == "repository.description.update" || commandID == "label.create" {
			return "repositories"
		}
	case "issue", "parent", "prerequisite":
		return "issues"
	case "project":
		return "projects"
	case "concept":
		return "concepts"
	case "label":
		return "labels"
	case "milestone":
		return "milestones"
	case "owner":
		if commandID == "milestone.create" {
			return "owners"
		}
	case "source":
		if commandID == "relation.create" {
			return "source_entities"
		}
	case "target":
		if commandID == "relation.create" {
			return "target_entities"
		}
	case "entity_ref":
		if commandID == "workspace.pin.toggle" || commandID == "note.link" {
			return "entities"
		}
	case "ref":
		switch {
		case strings.HasPrefix(commandID, "repository."):
			return "repositories"
		case strings.HasPrefix(commandID, "issue."):
			return "issues"
		case strings.HasPrefix(commandID, "project."):
			return "projects"
		case strings.HasPrefix(commandID, "pull_request."):
			return "pull_requests"
		case strings.HasPrefix(commandID, "concept."):
			return "concepts"
		case strings.HasPrefix(commandID, "label."):
			return "labels"
		case strings.HasPrefix(commandID, "milestone."):
			return "milestones"
		}
	case "id":
		switch {
		case strings.HasPrefix(commandID, "note."):
			return "notes"
		case strings.HasPrefix(commandID, "quote."):
			return "quotes"
		case strings.HasPrefix(commandID, "job."):
			return "jobs"
		case strings.HasPrefix(commandID, "notification."):
			return "notifications"
		}
	}
	return ""
}

func Catalog() []Command {
	out := make([]Command, 0, len(specs))
	for _, spec := range specs {
		out = append(out, metadata(spec.Command))
	}
	return out
}

func Find(id string) (Command, bool) {
	for _, spec := range specs {
		if spec.ID == id {
			return metadata(spec.Command), true
		}
	}
	return Command{}, false
}

// Options resolves the dynamic selector values exposed by the protocol schema. They are built
// from the same Application service as commands, so browser forms never invent their own data.
func Options(ctx context.Context, app *application.Service, source, entityType string) ([]Option, error) {
	if app == nil {
		return nil, fmt.Errorf("Application is unavailable")
	}
	switch source {
	case "repositories":
		items, err := app.ListRepositories(ctx)
		if err != nil {
			return nil, err
		}
		out := make([]Option, 0, len(items))
		for _, item := range items {
			out = append(out, Option{Label: item.Name, Value: item.ID})
		}
		return out, nil
	case "pr_issues":
		items, err := app.ListIssues(ctx, "", "in_progress")
		if err != nil {
			return nil, err
		}
		out := make([]Option, 0, len(items))
		for _, item := range items {
			if item.Blocked {
				continue
			}
			out = append(out, Option{Label: fmt.Sprintf("%s#%d · %s", item.RepositoryName, item.Number, item.Title), Value: item.ID})
		}
		return out, nil
	case "issues":
		items, err := app.ListIssues(ctx, "", "")
		if err != nil {
			return nil, err
		}
		out := make([]Option, 0, len(items))
		for _, item := range items {
			out = append(out, Option{Label: fmt.Sprintf("%s#%d · %s", item.RepositoryName, item.Number, item.Title), Value: item.ID})
		}
		return out, nil
	case "projects":
		items, err := app.ListProjects(ctx)
		if err != nil {
			return nil, err
		}
		out := make([]Option, 0, len(items))
		for _, item := range items {
			out = append(out, Option{Label: item.Name, Value: item.ID})
		}
		return out, nil
	case "pull_requests":
		items, err := app.ListPullRequests(ctx, "", "")
		if err != nil {
			return nil, err
		}
		out := make([]Option, 0, len(items))
		for _, item := range items {
			out = append(out, Option{Label: fmt.Sprintf("%s!%d · %s", item.RepositoryName, item.Number, item.Title), Value: item.ID})
		}
		return out, nil
	case "concepts":
		items, err := app.ListConcepts(ctx)
		if err != nil {
			return nil, err
		}
		out := make([]Option, 0, len(items))
		for _, item := range items {
			out = append(out, Option{Label: item.Name, Value: item.ID})
		}
		return out, nil
	case "labels":
		items, err := app.ListLabels(ctx, "")
		if err != nil {
			return nil, err
		}
		out := make([]Option, 0, len(items))
		for _, item := range items {
			suffix := item.ScopeType
			if item.RepositoryName != "" {
				suffix += " · " + item.RepositoryName
			}
			out = append(out, Option{Label: item.Name + " (" + suffix + ")", Value: item.ID})
		}
		return out, nil
	case "milestones":
		items, err := app.ListMilestones(ctx, "", "")
		if err != nil {
			return nil, err
		}
		out := make([]Option, 0, len(items))
		for _, item := range items {
			label := item.Title
			if item.ScopeName != "" {
				label += " · " + item.ScopeName
			}
			out = append(out, Option{Label: label, Value: item.ID})
		}
		return out, nil
	case "notes":
		items, err := app.ListNotes(ctx)
		if err != nil {
			return nil, err
		}
		out := make([]Option, 0, len(items))
		for _, item := range items {
			out = append(out, Option{Label: item.Title, Value: item.ID})
		}
		return out, nil
	case "quotes":
		items, err := app.ListQuotes(ctx)
		if err != nil {
			return nil, err
		}
		out := make([]Option, 0, len(items))
		for _, item := range items {
			label := []rune(item.Text)
			if len(label) > 72 {
				label = append(label[:69], '…')
			}
			out = append(out, Option{Label: string(label), Value: item.ID})
		}
		return out, nil
	case "jobs":
		items, err := app.ListScheduledJobs(ctx)
		if err != nil {
			return nil, err
		}
		out := make([]Option, 0, len(items))
		for _, item := range items {
			out = append(out, Option{Label: item.Name, Value: item.ID})
		}
		return out, nil
	case "notifications":
		items, err := app.ListNotifications(ctx, 100)
		if err != nil {
			return nil, err
		}
		out := make([]Option, 0, len(items))
		for _, item := range items {
			out = append(out, Option{Label: item.Title + " · " + item.CreatedAt.Format("2006-01-02 15:04"), Value: item.ID})
		}
		return out, nil
	case "branches":
		repositories, err := app.ListRepositories(ctx)
		if err != nil {
			return nil, err
		}
		out := make([]Option, 0, len(repositories))
		for _, repository := range repositories {
			branches, listErr := app.ListBranches(ctx, repository.ID)
			if listErr != nil {
				return nil, listErr
			}
			for _, branch := range branches {
				out = append(out, Option{Label: repository.Name + " · " + branch.Name, Value: branch.ID})
			}
		}
		return out, nil
	case "completions":
		items, err := app.ListCompletions(ctx, "")
		if err != nil {
			return nil, err
		}
		out := make([]Option, 0, len(items))
		for _, item := range items {
			out = append(out, Option{Label: fmt.Sprintf("%s#%d · %s", item.RepositoryName, item.IssueNumber, item.Summary), Value: item.ID})
		}
		return out, nil
	case "owners":
		switch entityType {
		case "repository":
			return Options(ctx, app, "repositories", "")
		case "project":
			return Options(ctx, app, "projects", "")
		default:
			return nil, fmt.Errorf("unsupported owner scope %q", entityType)
		}
	case "source_entities", "target_entities":
		return Options(ctx, app, "entities", entityType)
	case "entities":
		switch entityType {
		case "repository":
			return Options(ctx, app, "repositories", "")
		case "issue":
			return Options(ctx, app, "issues", "")
		case "pull_request":
			return Options(ctx, app, "pull_requests", "")
		case "project":
			return Options(ctx, app, "projects", "")
		case "concept":
			return Options(ctx, app, "concepts", "")
		case "note":
			return Options(ctx, app, "notes", "")
		case "label":
			return Options(ctx, app, "labels", "")
		case "milestone":
			return Options(ctx, app, "milestones", "")
		case "branch":
			return Options(ctx, app, "branches", "")
		case "completion":
			return Options(ctx, app, "completions", "")
		default:
			return nil, fmt.Errorf("unsupported entity type %q", entityType)
		}
	default:
		return nil, fmt.Errorf("unknown protocol options source %q", source)
	}
}

func Execute(ctx context.Context, app *application.Service, id string, input map[string]any) (any, error) {
	var selected *commandSpec
	for i := range specs {
		if specs[i].ID == id {
			selected = &specs[i]
			break
		}
	}
	if selected == nil {
		return nil, fmt.Errorf("unknown protocol command %q", id)
	}

	// Every adapter enters through this validation boundary. Reject unsupported keys and
	// normalize all supplied values before checking application readiness or looking up data.
	fields := make([]validation.Field, 0, len(selected.Fields))
	for _, field := range selected.Fields {
		source := field.Source
		if source == "" {
			source = fieldSource(selected.ID, field.Name)
		}
		fields = append(fields, validation.Field{
			Name: field.Name, Label: field.Label, Type: field.Type,
			Required: field.Required, Options: field.Options, Dynamic: source != "",
		})
	}
	validated, err := validation.ValidateCommandInput(selected.ID, fields, input)
	if err != nil {
		return nil, err
	}
	if app == nil {
		return nil, fmt.Errorf("IDIOS Application is unavailable")
	}
	return selected.run(ctx, app, validated)
}

func str(v map[string]any, key string) string {
	if value, ok := v[key]; ok && value != nil {
		return strings.TrimSpace(fmt.Sprint(value))
	}
	return ""
}
func boolean(v map[string]any, key string) bool {
	value, ok := v[key]
	if !ok || value == nil {
		return false
	}
	switch x := value.(type) {
	case bool:
		return x
	case string:
		return strings.EqualFold(x, "true") || strings.EqualFold(x, "yes") || x == "1"
	default:
		return false
	}
}
func integer(v map[string]any, key string, fallback int) int {
	value, ok := v[key]
	if !ok || value == nil || fmt.Sprint(value) == "" {
		return fallback
	}
	switch x := value.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	default:
		n, err := strconv.Atoi(fmt.Sprint(value))
		if err != nil {
			return fallback
		}
		return n
	}
}
func split(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, part := range parts {
		p := strings.TrimSpace(part)
		if p != "" && !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}
func jobFromInput(v map[string]any, id string) (domain.ScheduledJob, error) {
	job := domain.ScheduledJob{ID: id, Name: str(v, "name"), Description: str(v, "description"), ActionType: str(v, "action_type"), ScheduleType: str(v, "schedule_type"), IntervalMinutes: integer(v, "interval_minutes", 60), ScheduleTime: str(v, "schedule_time"), ScheduleWeekday: integer(v, "schedule_weekday", 1), ScheduleDay: integer(v, "schedule_day", 1), Enabled: boolean(v, "enabled"), Notify: boolean(v, "notify")}
	if branch := strings.TrimSpace(str(v, "branch")); branch != "" {
		job.BranchID = &branch
	}
	if job.ScheduleType == domain.ScheduleOnce {
		raw := strings.ReplaceAll(strings.TrimSpace(str(v, "schedule_at")), "T", " ")
		parsed, err := time.ParseInLocation("2006-01-02 15:04", raw, time.Local)
		if err != nil {
			return domain.ScheduledJob{}, fmt.Errorf("one-time schedule must use YYYY-MM-DD HH:MM")
		}
		job.NextRunAt = &parsed
	}
	return job, nil
}

func lines(raw string, limit int) ([]string, error) {
	raw = strings.ReplaceAll(strings.ReplaceAll(raw, "\r\n", "\n"), "\r", "\n")
	parts := strings.Split(raw, "\n")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	if len(out) > limit {
		return nil, fmt.Errorf("a batch can contain at most %d items", limit)
	}
	return out, nil
}
