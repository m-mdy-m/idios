package tui

import (
	tea "charm.land/bubbletea/v2"
	"idios/internal/domain"
	"strings"
)

func (m *model) currentRepository() (domain.Repository, bool) {
	for _, r := range m.repositories {
		if r.Name == m.repoContext {
			return r, true
		}
	}
	return domain.Repository{}, false
}

func (m *model) repoDescription() string {
	if r, ok := m.currentRepository(); ok {
		value := strings.TrimSpace(r.Description)
		if value == "" {
			return "No repository overview yet."
		}
		// The persistent navigation header gets a single-line summary; the Repository
		// Overview page displays the complete multiline README-style description.
		first, _, _ := strings.Cut(value, "\n")
		first = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(first), "#"))
		return fallback(first, "Repository overview")
	}
	return "Repository metadata"
}

func (m *model) issueCounts() map[domain.IssueStatus]int {
	out := map[domain.IssueStatus]int{}
	for _, i := range m.issues {
		out[i.Status]++
	}
	return out
}

func (m *model) blockedIssueCount() int {
	n := 0
	for _, i := range m.issues {
		if i.Blocked && i.Status != domain.IssueDone && i.Status != domain.IssueCancelled {
			n++
		}
	}
	return n
}

func (m *model) openPRCount() int {
	n := 0
	for _, pr := range m.pullRequests {
		if pr.Status == domain.PRPending {
			n++
		}
	}
	return n
}

func (m *model) findIssue(ref string) *domain.Issue {
	for i := range m.issues {
		if issueRef(m.issues[i]) == ref {
			return &m.issues[i]
		}
	}
	return nil
}

func (m *model) findPR(ref string) *domain.PullRequest {
	for i := range m.pullRequests {
		if prRef(m.pullRequests[i]) == ref {
			return &m.pullRequests[i]
		}
	}
	return nil
}

func (m *model) handleActionClick(action string) tea.Cmd { return m.handleAction(action) }
