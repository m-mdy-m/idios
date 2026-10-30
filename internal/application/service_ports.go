package application

import "idios/internal/ports"

type issueServiceStore interface {
	ports.IssueStore
	ports.RepositoryStore
}

type pullRequestServiceStore interface {
	ports.PullRequestStore
	ports.IssueStore
}

type shelfServiceStore interface {
	ports.ShelfStore
	ports.SettingsStore
}
