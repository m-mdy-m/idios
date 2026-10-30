// Package features declares web navigation and presentation metadata.
// Commands and workflow rules live in internal/protocol and internal/application respectively.
package features

import "sort"

type Feature struct {
	ID                  string `json:"id"`
	Label               string `json:"label"`
	Group               string `json:"group"`
	Description         string `json:"description"`
	WebPath             string `json:"web_path,omitempty"`
	Icon                string `json:"icon"`
	WebNavigation       bool   `json:"web_navigation,omitempty"`
	WebBottomNavigation bool   `json:"web_bottom_navigation,omitempty"`
	Metric              string `json:"metric,omitempty"`
	WebGroupOrder       int    `json:"web_group_order,omitempty"`
	WebOrder            int    `json:"web_order,omitempty"`
}

var catalog = []Feature{
	{ID: "overview", Label: "Overview", Group: "Workspace", Description: "Workspace health, progress, and recent activity.", WebPath: "/", Icon: "home", WebNavigation: true, WebGroupOrder: 1, WebOrder: 1},
	{ID: "issues", Label: "Issues", Group: "Workspace", Description: "Create, prioritize, start, cancel, and inspect issues and their blocking relationships.", WebPath: "/issues", Icon: "check-square", WebNavigation: true, Metric: "open_issues", WebGroupOrder: 1, WebOrder: 2},
	{ID: "repositories", Label: "Repositories", Group: "Workspace", Description: "Repository overview, issues, pull requests, branches, labels, milestones, settings, graph, and activity.", WebPath: "/repositories", Icon: "blocks", WebNavigation: true, Metric: "repositories", WebGroupOrder: 1, WebOrder: 3},
	{ID: "projects", Label: "Projects", Group: "Workspace", Description: "Group issues across repositories and track project progress.", WebPath: "/projects", Icon: "folder-kanban", WebNavigation: true, Metric: "projects", WebGroupOrder: 1, WebOrder: 4},
	{ID: "pull-requests", Label: "Pull requests", Group: "Workspace", Description: "Review and accept work records; acceptance completes the issue atomically.", WebPath: "/pull-requests", Icon: "git-pull-request", WebNavigation: true, WebGroupOrder: 1, WebOrder: 5},
	{ID: "graph", Label: "Knowledge graph", Group: "Knowledge", Description: "Explore typed relations without treating semantic links as blocking dependencies.", WebPath: "/graph", Icon: "git-branch", WebNavigation: true, WebGroupOrder: 2, WebOrder: 1},
	{ID: "concepts", Label: "Concepts", Group: "Knowledge", Description: "Manage reusable concepts and their semantic relationships.", WebPath: "/concepts", Icon: "brain-circuit", WebNavigation: true, WebGroupOrder: 2, WebOrder: 2},
	{ID: "shelf", Label: "Shelf", Group: "Knowledge", Description: "Browse a local book library, read PDFs and Markdown, connect notes and questions, and link books to projects.", WebPath: "/shelf", Icon: "library-big", WebNavigation: true, WebGroupOrder: 2, WebOrder: 3},
	{ID: "notes", Label: "Notes", Group: "Knowledge", Description: "Capture notes, labels, and links to workspace entities.", WebPath: "/notes", Icon: "book-open", WebNavigation: true, WebGroupOrder: 2, WebOrder: 4},
	{ID: "quotes", Label: "Quotes", Group: "Knowledge", Description: "Manage sourced quotations and quote reminders.", WebPath: "/quotes", Icon: "quote", WebNavigation: true, WebGroupOrder: 2, WebOrder: 5},
	{ID: "activity", Label: "Activity", Group: "Knowledge", Description: "Inspect recent workspace activity and history.", WebPath: "/activity", Icon: "activity", WebNavigation: true, WebGroupOrder: 2, WebOrder: 6},
	{ID: "plans", Label: "Daily plans", Group: "Organize", Description: "Schedule dated plans, check off tasks, and review completion over time.", WebPath: "/plans", Icon: "calendar-check", WebNavigation: true, WebGroupOrder: 3, WebOrder: 1},
	{ID: "planning", Label: "Labels & milestones", Group: "Organize", Description: "Classify work and track repository- or project-scoped milestones.", WebPath: "/planning", Icon: "tags", WebNavigation: true, WebGroupOrder: 3, WebOrder: 2},
	{ID: "jobs", Label: "Scheduled jobs", Group: "Organize", Description: "Create, reschedule, pause, run, and remove scheduled jobs.", WebPath: "/jobs", Icon: "calendar-clock", WebNavigation: true, Metric: "jobs", WebGroupOrder: 3, WebOrder: 3},
	{ID: "commands", Label: "Action center", Group: "Organize", Description: "Run any operation published in the shared Go protocol catalog.", WebPath: "/commands", Icon: "sparkles", WebNavigation: true, WebGroupOrder: 3, WebOrder: 4},
	{ID: "profile", Label: "Profile & pins", Group: "Workspace", Description: "Edit the workspace profile and keep important records pinned for quick access.", WebPath: "/profile", Icon: "circle-user-round", WebNavigation: true, WebGroupOrder: 1, WebOrder: 6},
	{ID: "remove", Label: "Remove items", Group: "Organize", Description: "Review and safely remove records with explicit destructive-action confirmation.", WebPath: "/remove", Icon: "trash-2", WebNavigation: true, WebGroupOrder: 3, WebOrder: 5},
	{ID: "logs", Label: "Logs & diagnostics", Group: "Organize", Description: "Inspect recent UI and service diagnostics.", WebPath: "/logs", Icon: "scroll-text", WebNavigation: true, WebGroupOrder: 3, WebOrder: 6},
	{ID: "help", Label: "Help & shortcuts", Group: "Organize", Description: "Learn workflow semantics, navigation, keyboard shortcuts, and client architecture.", WebPath: "/help", Icon: "circle-help", WebNavigation: true, WebGroupOrder: 3, WebOrder: 7},
	{ID: "notifications", Label: "Notifications", Group: "", Description: "Review reminders and mark notifications read.", WebPath: "/notifications", Icon: "bell", WebNavigation: true, WebBottomNavigation: true, WebGroupOrder: 4, WebOrder: 1},
	{ID: "settings", Label: "Settings", Group: "", Description: "Configure appearance, templates, reminders, startup, diagnostics, and local server behavior.", WebPath: "/settings", Icon: "settings-2", WebNavigation: true, WebBottomNavigation: true, WebGroupOrder: 4, WebOrder: 2},

	// Screens and contextual features are part of the same inventory but are intentionally not
	// global navigation items. Their parent pages provide context (for example, a selected repo).
	{ID: "intro", Label: "Introduction", Group: "Onboarding", Description: "First-run guide with live workspace progress.", WebPath: "/intro", Icon: "compass"},
	{ID: "notification-settings", Label: "Notification settings", Group: "Settings", Description: "Notification intervals, quote reminders, desktop delivery, and test delivery.", WebPath: "/settings/notifications", Icon: "bell-ring"},
	{ID: "repository-overview", Label: "Repository overview", Group: "Repository workspace", Description: "Inspect the selected repository summary and quick actions.", WebPath: "/repositories?tab=Overview", Icon: "blocks"},
	{ID: "repository-issues", Label: "Repository issues", Group: "Repository workspace", Description: "Inspect issues scoped to the selected repository.", WebPath: "/repositories?tab=Issues", Icon: "check-square"},
	{ID: "repository-pull-requests", Label: "Repository pull requests", Group: "Repository workspace", Description: "Inspect review records scoped to the selected repository.", WebPath: "/repositories?tab=Pull%20requests", Icon: "git-pull-request"},
	{ID: "job-queue", Label: "Scheduled job queue", Group: "Automation", Description: "Inspect upcoming scheduled runs, prior run times, and completed one-time jobs.", WebPath: "/jobs#queue", Icon: "calendar-clock"},
	{ID: "bulk-issues", Label: "Bulk issue creation", Group: "Issues", Description: "Preview, create, and retry issue batch items without repeating successful writes.", WebPath: "/issues?bulk=1", Icon: "list-checks"},
	{ID: "bulk-quotes", Label: "Bulk quote creation", Group: "Quotes", Description: "Preview and save multiple quotes using the shared command protocol.", WebPath: "/quotes?bulk=1", Icon: "list-checks"},
	{ID: "bulk-jobs", Label: "Bulk job creation", Group: "Automation", Description: "Preview and create multiple scheduled jobs using the shared command protocol.", WebPath: "/jobs?bulk=1", Icon: "list-checks"},
	{ID: "updates", Label: "Updates", Group: "Settings", Description: "Check release notes and install a verified application update.", WebPath: "/settings/updates", Icon: "download"},
	{ID: "repository-branches", Label: "Repository branches", Group: "Repository workspace", Description: "Inspect the default main branch and branch records for the selected repository.", WebPath: "/repositories?tab=Branches", Icon: "git-branch"},
	{ID: "repository-labels", Label: "Repository labels", Group: "Repository workspace", Description: "Create and remove labels for the selected repository.", WebPath: "/repositories?tab=Labels", Icon: "tags"},
	{ID: "repository-milestones", Label: "Repository milestones", Group: "Repository workspace", Description: "Manage milestones and due dates for the selected repository.", WebPath: "/repositories?tab=Milestones", Icon: "layers"},
	{ID: "repository-settings", Label: "Repository settings", Group: "Repository workspace", Description: "View repository details and workflow defaults.", WebPath: "/repositories?tab=Settings", Icon: "settings-2"},
	{ID: "repository-activity", Label: "Repository activity", Group: "Repository workspace", Description: "Review activity and accepted completion history for the selected repository.", WebPath: "/repositories?tab=Activity", Icon: "activity"},
	{ID: "repository-graph", Label: "Repository graph", Group: "Repository workspace", Description: "Focus the knowledge graph on the selected repository.", WebPath: "/repositories?tab=Graph", Icon: "git-branch"},
	{ID: "batch-results", Label: "Batch results", Group: "Transient", Description: "Per-item outcomes with retry for failed items only.", WebPath: "/commands", Icon: "list-checks"},
	{ID: "batch-progress", Label: "Batch progress", Group: "Transient", Description: "Progress and safe cancellation for a running batch operation.", WebPath: "/commands", Icon: "list-checks"},
	{ID: "issue-detail", Label: "Issue details", Group: "Details", Description: "Dependencies, labels, milestones, concept links, actions, and graph context.", WebPath: "/issues", Icon: "check-square"},
	{ID: "pull-request-detail", Label: "Pull request details", Group: "Details", Description: "Edit, accept, close, and navigate to the linked issue.", WebPath: "/pull-requests", Icon: "git-pull-request"},
	{ID: "project-detail", Label: "Project details", Group: "Details", Description: "View linked issues and add work to a project.", WebPath: "/projects", Icon: "folder-kanban"},
	{ID: "concept-detail", Label: "Concept details", Group: "Details", Description: "Inspect concept context and open the connected graph.", WebPath: "/concepts", Icon: "brain-circuit"},
	{ID: "new-item", Label: "Create menu", Group: "Actions", Description: "Quick-create common entities from anywhere in the workspace.", WebPath: "/commands", Icon: "plus"},
	{ID: "generic-form", Label: "Generic form", Group: "Transient", Description: "Render an action form from the shared protocol schema.", WebPath: "/commands", Icon: "list-checks"},
	{ID: "delete-confirmation", Label: "Delete confirmation", Group: "Transient", Description: "Review the exact record and warnings before a destructive operation.", WebPath: "/remove", Icon: "trash-2"},
}

// Catalog returns a defensive copy to keep callers from mutating package metadata.
func Catalog() []Feature {
	out := make([]Feature, len(catalog))
	copy(out, catalog)
	return out
}

func WebNavigation() []Feature {
	var out []Feature
	for _, item := range catalog {
		if item.WebNavigation {
			out = append(out, item)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].WebGroupOrder == out[j].WebGroupOrder {
			return out[i].WebOrder < out[j].WebOrder
		}
		return out[i].WebGroupOrder < out[j].WebGroupOrder
	})
	return out
}

func WebBottomNavigation() []Feature {
	var out []Feature
	for _, item := range catalog {
		if item.WebNavigation && item.WebBottomNavigation {
			out = append(out, item)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].WebOrder < out[j].WebOrder })
	return out
}
