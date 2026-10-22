package tui

import (
	"context"
	"fmt"

	tea "charm.land/bubbletea/v2"
	"idios/internal/application"
	"idios/internal/protocol"
)

type removableItem struct {
	ID      string
	Label   string
	Detail  string
	Warning string
}

type deleteTarget struct {
	Kind             string
	ID               string
	Label            string
	Detail           string
	Warning          string
	TypeNameRequired bool
}

var removeCategories = []struct {
	kind  string
	label string
}{
	{"repository", "Repositories"}, {"issue", "Issues"}, {"pull_request", "Pull requests"},
	{"project", "Projects"}, {"concept", "Concepts"}, {"note", "Notes"}, {"quote", "Quotes"},
	{"job", "Scheduled jobs"}, {"label", "Labels"}, {"milestone", "Milestones"}, {"notification", "Notifications"},
}

func (m *model) renderRemoveCenter(c *canvas, p palette) {
	c.title("Remove items")
	if m.removeKind == "" {
		c.muted("One place to remove items across your workspace. Choose a category, review the exact target, then confirm.")
		c.panel("BE CAREFUL WITH PERMANENT DATA", "Repository deletion removes its Issues, Pull Requests, completions, branches, local labels, milestones and settings.", "Issues that have child/dependency references and accepted Pull Requests with completion history are protected from individual deletion.")
		counts := m.removeCounts()
		for i, category := range m.filteredRemoveCategories() {
			m.selectLines = append(m.selectLines, len(c.lines))
			c.buttonRow(buttonSpec{fmt.Sprintf("%s · %d", category.label, counts[category.kind]), "remove-kind:" + category.kind, i == m.cursor})
		}
		c.blank()
		c.muted("Tip: / filters categories or item details. Repository removal requires typing its exact name.")
		return
	}

	categoryLabel := removeCategoryLabel(m.removeKind)
	items := m.removableItems(m.removeKind)
	c.buttonRow(buttonSpec{"← All removal categories", "remove:categories", false}, buttonSpec{"Refresh", "refresh:now", false})
	c.muted(fmt.Sprintf("%s · %d item(s) · / filters by name and details", categoryLabel, len(items)))
	if len(items) == 0 {
		c.panel("NOTHING TO REMOVE", "There are no items in this category, or no items match the current filter.")
		return
	}
	for idx, item := range items {
		m.selectLines = append(m.selectLines, len(c.lines))
		c.title(truncateVisible(item.Label, max(10, c.width-4)))
		if item.Detail != "" {
			c.muted(truncateVisible(item.Detail, max(10, c.width-4)))
		}
		c.buttonRow(buttonSpec{"Remove…", "remove:select:" + m.removeKind + "|" + item.ID, idx == m.cursor})
		c.rule()
	}
}

func (m *model) filteredRemoveCategories() []struct {
	kind  string
	label string
} {
	items := make([]struct {
		kind  string
		label string
	}, 0, len(removeCategories))
	for _, category := range removeCategories {
		if containsFold(category.label, m.search) {
			items = append(items, category)
		}
	}
	return items
}

func (m *model) removeCounts() map[string]int {
	return map[string]int{
		"repository": len(m.repositories), "issue": len(m.issues), "pull_request": len(m.pullRequests),
		"project": len(m.projects), "concept": len(m.concepts), "note": len(m.notes), "quote": len(m.quotes),
		"job": len(m.jobs), "label": len(m.labels), "milestone": len(m.milestones), "notification": len(m.notifications),
	}
}

func (m *model) removableItems(kind string) []removableItem {
	items := make([]removableItem, 0)
	add := func(id, label, detail string) {
		if id == "" || (m.search != "" && !containsFold(label+" "+detail, m.search)) {
			return
		}
		items = append(items, removableItem{ID: id, Label: label, Detail: detail})
	}
	switch kind {
	case "repository":
		for _, item := range m.repositories {
			add(item.ID, item.Name, item.Description)
		}
	case "issue":
		for _, item := range m.issues {
			add(item.ID, fmt.Sprintf("%s#%d · %s", item.RepositoryName, item.Number, item.Title), string(item.Status)+" · priority "+fallback(item.Priority, "normal"))
		}
	case "pull_request":
		for _, item := range m.pullRequests {
			add(item.ID, fmt.Sprintf("%s!%d · %s", item.RepositoryName, item.Number, item.Title), string(item.Status)+fmt.Sprintf(" · Issue %s#%d", item.RepositoryName, item.IssueNumber))
		}
	case "project":
		for _, item := range m.projects {
			add(item.ID, item.Name, item.Description)
		}
	case "concept":
		for _, item := range m.concepts {
			add(item.ID, item.Name, item.Description)
		}
	case "note":
		for _, item := range m.notes {
			add(item.ID, item.Title, item.LinkedEntityLabel)
		}
	case "quote":
		for _, item := range m.quotes {
			add(item.ID, "“"+truncateVisible(item.Text, 100)+"”", fallback(item.Author, "No source"))
		}
	case "job":
		for _, item := range m.jobs {
			state := "paused"
			if item.Enabled {
				state = "enabled"
			}
			if item.CompletedAt != nil {
				state = "completed"
			}
			add(item.ID, item.Name, state+" · "+scheduleSummary(item))
		}
	case "label":
		for _, item := range m.labels {
			detail := item.ScopeType
			if item.RepositoryName != "" {
				detail += " · " + item.RepositoryName
			}
			if item.Description != "" {
				detail += " · " + item.Description
			}
			add(item.ID, item.Name, detail)
		}
	case "milestone":
		for _, item := range m.milestones {
			detail := item.ScopeType + " · " + item.ScopeName
			if item.DueDate != nil && *item.DueDate != "" {
				detail += " · due " + *item.DueDate
			}
			add(item.ID, item.Title, detail)
		}
	case "notification":
		for _, item := range m.notifications {
			state := "unread"
			if item.ReadAt != nil {
				state = "read"
			}
			add(item.ID, item.Title, state+" · "+item.CreatedAt.Local().Format("2006-01-02 15:04")+" · "+item.Body)
		}
	}
	return items
}

func (m *model) beginRemoveTarget(kind, id string) bool {
	var target deleteTarget
	found := false
	for _, item := range m.removableItems(kind) {
		if item.ID == id {
			target = deleteTarget{Kind: kind, ID: item.ID, Label: item.Label, Detail: item.Detail, Warning: removeWarning(kind)}
			found = true
			break
		}
	}
	// The current list can be filtered. Resolve by stable ID from the unfiltered snapshot
	// if the item is valid but is outside the active text filter.
	if !found {
		oldSearch := m.search
		m.search = ""
		for _, item := range m.removableItems(kind) {
			if item.ID == id {
				target = deleteTarget{Kind: kind, ID: item.ID, Label: item.Label, Detail: item.Detail, Warning: removeWarning(kind)}
				found = true
				break
			}
		}
		m.search = oldSearch
	}
	if !found {
		m.status, m.statusError = "The selected item no longer exists. Refresh the workspace and try again.", true
		return false
	}
	if kind == "repository" {
		target.TypeNameRequired = true
	}
	m.deleteTarget = target
	m.repoContext = ""
	m.page = pageDeleteConfirm
	m.cursor, m.scroll = 0, 0
	m.buttonFocus, m.focusAction, m.focusIndex = false, "", -1
	return true
}

func removeCategoryLabel(kind string) string {
	for _, cat := range removeCategories {
		if cat.kind == kind {
			return cat.label
		}
	}
	return "Items"
}

func removeTargetNoun(kind string) string {
	switch kind {
	case "repository":
		return "repository"
	case "issue":
		return "Issue"
	case "pull_request":
		return "Pull Request"
	case "project":
		return "project"
	case "concept":
		return "concept"
	case "note":
		return "note"
	case "quote":
		return "quote"
	case "job":
		return "scheduled job"
	case "label":
		return "label"
	case "milestone":
		return "milestone"
	case "notification":
		return "notification"
	default:
		return "item"
	}
}

func removeWarning(kind string) string {
	switch kind {
	case "repository":
		return "This permanently deletes the repository and its Issues, Pull Requests, completion history, branches, local labels, milestones and settings. Notes are preserved but their dead links are cleared. This cannot be undone."
	case "issue":
		return "This permanently deletes the Issue and its links. IDIOS refuses while child Issues, incoming dependencies, any Pull Request or completion history still reference it. Accepted-work history can only be removed by deleting the entire repository."
	case "pull_request":
		return "This permanently deletes the Pull Request and clears its links. A Pull Request with an accepted completion record is protected to preserve work history."
	case "project":
		return "This permanently deletes the Project, its memberships and project-scoped milestones. The Issues themselves are kept in their repositories."
	case "concept":
		return "This permanently deletes the Concept and its semantic links. Notes are kept but links to this Concept are cleared."
	case "note":
		return "This permanently deletes the Note and its label assignments."
	case "quote":
		return "This permanently deletes the saved Quote from the local workspace."
	case "job":
		return "This permanently deletes the Scheduled Job and its run history. Existing notifications are kept."
	case "label":
		return "This permanently deletes the Label and removes its assignments from Issues."
	case "milestone":
		return "This permanently deletes the Milestone and removes its assignments from Issues."
	case "notification":
		return "This permanently deletes the Notification from the local inbox."
	default:
		return "This action permanently deletes the selected item."
	}
}

func (m *model) renderDeleteConfirmation(c *canvas, p palette) {
	target := m.deleteTarget
	c.title("Remove " + removeTargetNoun(target.Kind) + "?")
	c.muted("Review the exact target and consequences before confirming.")
	c.panel("SELECTED ITEM", target.Label, target.Detail)
	c.panel("WHAT WILL HAPPEN", target.Warning)
	if target.TypeNameRequired {
		c.panel("EXTRA CONFIRMATION", "Repository deletion removes all repository-owned workflow data. You must type the exact repository name on the next screen.")
	} else {
		c.muted("If the item is protected by workflow history or a database relationship, IDIOS will stop and report why instead of forcing the deletion.")
	}
	c.buttonRow(buttonSpec{"Permanently remove", "remove:confirm", true}, buttonSpec{"Cancel", "remove:cancel", false})
}

func (m *model) runDeleteTarget() tea.Cmd {
	target := m.deleteTarget
	ctx, app := m.ctx, m.app
	return func() tea.Msg {
		err := deleteTargetRecord(ctx, app, target)
		if err != nil {
			return operationMsg{message: "Could not remove " + target.Label + ".", err: err, openPage: pageDeleteConfirm}
		}
		return operationMsg{message: "Removed: " + target.Label, openPage: pageRemove, clearDeleteTarget: true}
	}
}

func deleteTargetRecord(ctx context.Context, app *application.Service, target deleteTarget) error {
	commandByKind := map[string]string{
		"repository":   "repository.delete",
		"issue":        "issue.delete",
		"pull_request": "pull_request.delete",
		"project":      "project.delete",
		"concept":      "concept.delete",
		"note":         "note.delete",
		"quote":        "quote.delete",
		"job":          "job.delete",
		"label":        "label.delete",
		"milestone":    "milestone.delete",
		"notification": "notification.delete",
	}
	commandID, ok := commandByKind[target.Kind]
	if !ok {
		return fmt.Errorf("unsupported removal category %q", target.Kind)
	}
	field := "ref"
	switch target.Kind {
	case "note", "quote", "job", "notification":
		field = "id"
	}
	_, err := protocol.Execute(ctx, app, commandID, map[string]any{field: target.ID})
	return err
}

func (m *model) beginRepositoryDeleteConfirmation() {
	m.formReturnPage, m.formReturnRepo = pageDeleteConfirm, m.repoContext
	m.form = formDeleteRepositoryConfirm
	m.formTitle = "Confirm repository deletion"
	m.formError = ""
	m.formFocus, m.formCaret = 0, 0
	m.formFields = m.fieldsFor(formDeleteRepositoryConfirm)
	m.page = pageForm
}

func repositoryDeleteConfirmItem(target deleteTarget) string {
	return fmt.Sprintf("repository name must exactly match %q", target.Label)
}
