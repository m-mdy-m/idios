// Package features declares the user-facing IDIOS feature catalog shared by every client.
// It contains navigation and presentation metadata only; commands and workflow rules live in
// internal/protocol and internal/application respectively.
package features

import "sort"

type Feature struct {
	ID                  string `json:"id"`
	Label               string `json:"label"`
	Group               string `json:"group"`
	Description         string `json:"description"`
	WebPath             string `json:"web_path,omitempty"`
	Icon                string `json:"icon"`
	TUIPage             string `json:"tui_page,omitempty"`
	TUIAction           string `json:"tui_action,omitempty"`
	WebNavigation       bool   `json:"web_navigation,omitempty"`
	WebBottomNavigation bool   `json:"web_bottom_navigation,omitempty"`
	TUITopNavigation    bool   `json:"tui_top_navigation,omitempty"`
	TUISideNavigation   bool   `json:"tui_side_navigation,omitempty"`
	TUICycleNavigation  bool   `json:"tui_cycle_navigation,omitempty"`
	Metric              string `json:"metric,omitempty"`
	WebGroupOrder       int    `json:"web_group_order,omitempty"`
	WebOrder            int    `json:"web_order,omitempty"`
	TUITopOrder         int    `json:"-"`
	TUISideOrder        int    `json:"-"`
	TUICycleOrder       int    `json:"-"`
}

var catalog = []Feature{
	{ID: "overview", Label: "Overview", Group: "Workspace", Description: "Workspace health, progress, and recent activity.", WebPath: "/", Icon: "home", TUIPage: "overview", WebNavigation: true, TUITopNavigation: true, TUICycleNavigation: true, TUITopOrder: 1, TUICycleOrder: 1, WebGroupOrder: 1, WebOrder: 1},
	{ID: "issues", Label: "Issues", Group: "Workspace", Description: "Create, prioritize, start, cancel, and inspect issues and their blocking relationships.", WebPath: "/issues", Icon: "check-square", TUIPage: "issues", WebNavigation: true, TUITopNavigation: true, TUICycleNavigation: true, TUITopOrder: 3, TUICycleOrder: 4, Metric: "open_issues", WebGroupOrder: 1, WebOrder: 2},
	{ID: "repositories", Label: "Repositories", Group: "Workspace", Description: "Repository overview, issues, pull requests, branches, labels, milestones, settings, graph, and activity.", WebPath: "/repositories", Icon: "blocks", TUIPage: "repositories", WebNavigation: true, TUITopNavigation: true, TUICycleNavigation: true, TUITopOrder: 2, TUICycleOrder: 2, Metric: "repositories", WebGroupOrder: 1, WebOrder: 3},
	{ID: "projects", Label: "Projects", Group: "Workspace", Description: "Group issues across repositories and track project progress.", WebPath: "/projects", Icon: "folder-kanban", TUIPage: "projects", WebNavigation: true, TUISideNavigation: true, TUICycleNavigation: true, TUISideOrder: 1, TUICycleOrder: 3, Metric: "projects", WebGroupOrder: 1, WebOrder: 4},
	{ID: "pull-requests", Label: "Pull requests", Group: "Workspace", Description: "Review and accept work records; acceptance completes the issue atomically.", WebPath: "/pull-requests", Icon: "git-pull-request", TUIPage: "pull-requests", WebNavigation: true, TUISideNavigation: true, TUICycleNavigation: true, TUISideOrder: 2, TUICycleOrder: 5, WebGroupOrder: 1, WebOrder: 5},
	{ID: "graph", Label: "Knowledge graph", Group: "Knowledge", Description: "Explore typed relations without treating semantic links as blocking dependencies.", WebPath: "/graph", Icon: "git-branch", TUIPage: "graph", WebNavigation: true, TUISideNavigation: true, TUICycleNavigation: true, TUISideOrder: 3, TUICycleOrder: 6, WebGroupOrder: 2, WebOrder: 1},
	{ID: "concepts", Label: "Concepts", Group: "Knowledge", Description: "Manage reusable concepts and their semantic relationships.", WebPath: "/concepts", Icon: "brain-circuit", TUIPage: "concepts", WebNavigation: true, TUISideNavigation: true, TUICycleNavigation: true, TUISideOrder: 4, TUICycleOrder: 7, WebGroupOrder: 2, WebOrder: 2},
	{ID: "notes", Label: "Notes", Group: "Knowledge", Description: "Capture notes, labels, and links to workspace entities.", WebPath: "/notes", Icon: "book-open", TUIPage: "notes", WebNavigation: true, TUISideNavigation: true, TUICycleNavigation: true, TUISideOrder: 5, TUICycleOrder: 8, WebGroupOrder: 2, WebOrder: 3},
	{ID: "quotes", Label: "Quotes", Group: "Knowledge", Description: "Manage sourced quotations and quote reminders.", WebPath: "/quotes", Icon: "quote", TUIPage: "quotes", WebNavigation: true, TUISideNavigation: true, TUICycleNavigation: true, TUISideOrder: 6, TUICycleOrder: 9, WebGroupOrder: 2, WebOrder: 4},
	{ID: "activity", Label: "Activity", Group: "Knowledge", Description: "Inspect recent workspace activity and history.", WebPath: "/activity", Icon: "activity", WebNavigation: true, WebGroupOrder: 2, WebOrder: 5},
	{ID: "plans", Label: "Daily plans", Group: "Organize", Description: "Schedule dated plans, check off tasks, and review completion over time.", WebPath: "/plans", Icon: "calendar-check", WebNavigation: true, WebGroupOrder: 3, WebOrder: 1},
	{ID: "planning", Label: "Labels & milestones", Group: "Organize", Description: "Classify work and track repository- or project-scoped milestones.", WebPath: "/planning", Icon: "tags", WebNavigation: true, WebGroupOrder: 3, WebOrder: 2},
	{ID: "jobs", Label: "Scheduled jobs", Group: "Organize", Description: "Create, reschedule, pause, run, and remove scheduled jobs.", WebPath: "/jobs", Icon: "calendar-clock", TUIPage: "jobs", WebNavigation: true, Metric: "jobs", TUITopNavigation: true, TUICycleNavigation: true, TUITopOrder: 4, TUICycleOrder: 10, WebGroupOrder: 3, WebOrder: 3},
	{ID: "commands", Label: "Action center", Group: "Organize", Description: "Run any operation published in the shared Go protocol catalog.", WebPath: "/commands", Icon: "sparkles", TUIPage: "command-palette", WebNavigation: true, TUISideNavigation: true, WebGroupOrder: 3, WebOrder: 4},
	{ID: "profile", Label: "Profile & pins", Group: "Workspace", Description: "Edit the workspace profile and keep important records pinned for quick access.", WebPath: "/profile", Icon: "circle-user-round", TUIAction: "settings-action:profile", WebNavigation: true, TUISideNavigation: true, TUISideOrder: 7, WebGroupOrder: 1, WebOrder: 6},
	{ID: "remove", Label: "Remove items", Group: "Organize", Description: "Review and safely remove records with explicit destructive-action confirmation.", WebPath: "/remove", Icon: "trash-2", TUIPage: "remove-center", WebNavigation: true, TUISideNavigation: true, TUICycleNavigation: true, TUISideOrder: 8, TUICycleOrder: 11, WebGroupOrder: 3, WebOrder: 5},
	{ID: "logs", Label: "Logs & diagnostics", Group: "Organize", Description: "Inspect recent UI and service diagnostics.", WebPath: "/logs", Icon: "scroll-text", TUIPage: "logs", WebNavigation: true, TUISideNavigation: true, TUICycleNavigation: true, TUISideOrder: 10, TUICycleOrder: 13, WebGroupOrder: 3, WebOrder: 6},
	{ID: "help", Label: "Help & shortcuts", Group: "Organize", Description: "Learn workflow semantics, navigation, keyboard shortcuts, and client architecture.", WebPath: "/help", Icon: "circle-help", TUIPage: "help", WebNavigation: true, TUISideNavigation: true, TUISideOrder: 12, WebGroupOrder: 3, WebOrder: 7},
	{ID: "notifications", Label: "Notifications", Group: "", Description: "Review reminders and mark notifications read.", WebPath: "/notifications", Icon: "bell", TUIPage: "notifications", WebNavigation: true, WebBottomNavigation: true, TUISideNavigation: true, TUICycleNavigation: true, TUISideOrder: 9, TUICycleOrder: 12, WebGroupOrder: 4, WebOrder: 1},
	{ID: "settings", Label: "Settings", Group: "", Description: "Configure appearance, templates, reminders, startup, diagnostics, and local server behavior.", WebPath: "/settings", Icon: "settings-2", TUIPage: "settings", WebNavigation: true, WebBottomNavigation: true, TUISideNavigation: true, TUICycleNavigation: true, TUISideOrder: 11, TUICycleOrder: 14, WebGroupOrder: 4, WebOrder: 2},

	// Screens and contextual features are part of the same inventory but are intentionally not
	// global navigation items. Their parent pages provide context (for example, a selected repo).
	{ID: "intro", Label: "Introduction", Group: "Onboarding", Description: "First-run guide with live workspace progress.", WebPath: "/intro", Icon: "compass", TUIPage: "intro"},
	{ID: "notification-settings", Label: "Notification settings", Group: "Settings", Description: "Notification intervals, quote reminders, desktop delivery, and test delivery.", WebPath: "/settings/notifications", Icon: "bell-ring", TUIPage: "settings-notifications", TUIAction: "settings-action:notifications-page"},
	{ID: "keyboard-shortcuts", Label: "Keyboard shortcuts", Group: "Settings", Description: "Edit the shared TUI key map while browsing from Web.", WebPath: "/settings/keyboard", Icon: "keyboard", TUIPage: "settings"},
	{ID: "repository-overview", Label: "Repository overview", Group: "Repository workspace", Description: "Inspect the selected repository summary and quick actions.", WebPath: "/repositories?tab=Overview", Icon: "blocks", TUIPage: "repository-overview"},
	{ID: "repository-issues", Label: "Repository issues", Group: "Repository workspace", Description: "Inspect issues scoped to the selected repository.", WebPath: "/repositories?tab=Issues", Icon: "check-square", TUIPage: "repository-issues"},
	{ID: "repository-pull-requests", Label: "Repository pull requests", Group: "Repository workspace", Description: "Inspect review records scoped to the selected repository.", WebPath: "/repositories?tab=Pull%20requests", Icon: "git-pull-request", TUIPage: "repository-pull-requests"},
	{ID: "job-queue", Label: "Scheduled job queue", Group: "Automation", Description: "Inspect upcoming scheduled runs, prior run times, and completed one-time jobs.", WebPath: "/jobs#queue", Icon: "calendar-clock", TUIPage: "jobs"},
	{ID: "bulk-issues", Label: "Bulk issue creation", Group: "Issues", Description: "Preview, create, and retry issue batch items without repeating successful writes.", WebPath: "/issues?bulk=1", Icon: "list-checks", TUIAction: "protocol:issue.bulk_create"},
	{ID: "bulk-quotes", Label: "Bulk quote creation", Group: "Quotes", Description: "Preview and save multiple quotes using the shared command protocol.", WebPath: "/quotes?bulk=1", Icon: "list-checks", TUIAction: "protocol:quote.bulk_create"},
	{ID: "bulk-jobs", Label: "Bulk job creation", Group: "Automation", Description: "Preview and create multiple scheduled jobs using the shared command protocol.", WebPath: "/jobs?bulk=1", Icon: "list-checks", TUIAction: "protocol:job.bulk_create"},
	{ID: "updates", Label: "Updates", Group: "Settings", Description: "Check release notes and install a verified application update.", WebPath: "/settings/updates", Icon: "download", TUIPage: "upgrade", TUIAction: "upgrade:open"},
	{ID: "repository-branches", Label: "Repository branches", Group: "Repository workspace", Description: "Inspect the default main branch and branch records for the selected repository.", WebPath: "/repositories?tab=Branches", Icon: "git-branch", TUIPage: "repository-branches"},
	{ID: "repository-labels", Label: "Repository labels", Group: "Repository workspace", Description: "Create and remove labels for the selected repository.", WebPath: "/repositories?tab=Labels", Icon: "tags", TUIPage: "repository-labels"},
	{ID: "repository-milestones", Label: "Repository milestones", Group: "Repository workspace", Description: "Manage milestones and due dates for the selected repository.", WebPath: "/repositories?tab=Milestones", Icon: "layers", TUIPage: "repository-milestones"},
	{ID: "repository-settings", Label: "Repository settings", Group: "Repository workspace", Description: "View repository details and workflow defaults.", WebPath: "/repositories?tab=Settings", Icon: "settings-2", TUIPage: "repository-settings"},
	{ID: "repository-activity", Label: "Repository activity", Group: "Repository workspace", Description: "Review activity and accepted completion history for the selected repository.", WebPath: "/repositories?tab=Activity", Icon: "activity", TUIPage: "repository-activity"},
	{ID: "repository-graph", Label: "Repository graph", Group: "Repository workspace", Description: "Focus the knowledge graph on the selected repository.", WebPath: "/repositories?tab=Graph", Icon: "git-branch", TUIPage: "repository-graph"},
	{ID: "batch-results", Label: "Batch results", Group: "Transient", Description: "Per-item outcomes with retry for failed items only.", WebPath: "/commands", Icon: "list-checks", TUIPage: "batch-results"},
	{ID: "batch-progress", Label: "Batch progress", Group: "Transient", Description: "Progress and safe cancellation for the running TUI batch.", WebPath: "/commands", Icon: "list-checks", TUIPage: "batch-progress"},
	{ID: "issue-detail", Label: "Issue details", Group: "Details", Description: "Dependencies, labels, milestones, concept links, actions, and graph context.", WebPath: "/issues", Icon: "check-square", TUIPage: "issue-detail"},
	{ID: "pull-request-detail", Label: "Pull request details", Group: "Details", Description: "Edit, accept, close, and navigate to the linked issue.", WebPath: "/pull-requests", Icon: "git-pull-request", TUIPage: "pr-detail"},
	{ID: "project-detail", Label: "Project details", Group: "Details", Description: "View linked issues and add work to a project.", WebPath: "/projects", Icon: "folder-kanban", TUIPage: "project-detail"},
	{ID: "concept-detail", Label: "Concept details", Group: "Details", Description: "Inspect concept context and open the connected graph.", WebPath: "/concepts", Icon: "brain-circuit", TUIPage: "concept-detail"},
	{ID: "new-item", Label: "Create menu", Group: "Actions", Description: "Quick-create common entities from anywhere in the workspace.", WebPath: "/commands", Icon: "plus", TUIPage: "new"},
	{ID: "generic-form", Label: "Generic form", Group: "Transient", Description: "Render an action form from the shared protocol schema.", WebPath: "/commands", Icon: "list-checks", TUIPage: "form"},
	{ID: "delete-confirmation", Label: "Delete confirmation", Group: "Transient", Description: "Review the exact record and warnings before a destructive operation.", WebPath: "/remove", Icon: "trash-2", TUIPage: "delete-confirmation"},
	{ID: "quit-confirmation", Label: "Quit confirmation", Group: "Terminal", Description: "Confirm a graceful TUI exit without terminating an active batch unexpectedly.", Icon: "log-out", TUIPage: "quit-confirmation", TUIAction: "quit:confirm"},
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

func TUITopNavigation() []Feature {
	var out []Feature
	for _, item := range catalog {
		if item.TUITopNavigation {
			out = append(out, item)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].TUITopOrder < out[j].TUITopOrder })
	return out
}

func TUISideNavigation() []Feature {
	var out []Feature
	for _, item := range catalog {
		if item.TUISideNavigation {
			out = append(out, item)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].TUISideOrder < out[j].TUISideOrder })
	return out
}

func TUICycleNavigation() []Feature {
	var out []Feature
	for _, item := range catalog {
		if item.TUICycleNavigation {
			out = append(out, item)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].TUICycleOrder < out[j].TUICycleOrder })
	return out
}
