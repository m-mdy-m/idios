package ports

import (
	"context"
	"idios/internal/domain"
	"time"
)

type Store interface {
	Close() error
	Ping(context.Context) error

	CreateRepository(context.Context, string, string) (domain.Repository, error)
	DeleteRepository(context.Context, string) error
	GetRepository(context.Context, string) (domain.Repository, error)
	ListRepositories(context.Context) ([]domain.Repository, error)
	ListBranches(context.Context, string) ([]domain.Branch, error)
	SetRepositorySetting(context.Context, string, string, string) error
	GetRepositorySettings(context.Context, string) ([]domain.RepositorySetting, error)

	CreateProject(context.Context, string, string) (domain.Project, error)
	DeleteProject(context.Context, string) error
	GetProject(context.Context, string) (domain.Project, error)
	ListProjects(context.Context) ([]domain.Project, error)
	AddIssueToProject(context.Context, string, string) error
	ListProjectIssues(context.Context, string) ([]domain.Issue, error)

	CreateIssue(context.Context, domain.Issue) (domain.Issue, error)
	DeleteIssue(context.Context, string) error
	GetIssue(context.Context, string) (domain.Issue, error)
	ListIssues(context.Context, string, string) ([]domain.Issue, error)
	SetIssueStatus(context.Context, string, domain.IssueStatus) (domain.Issue, error)
	StartIssue(context.Context, string) (domain.Issue, error)
	ListBlockers(context.Context, string) ([]domain.Blocker, error)
	ListIssueDependencies(context.Context, string) ([]domain.Issue, error)
	AddDependency(context.Context, string, string) error
	RemoveDependency(context.Context, string, string) error
	ListDependencyEdges(context.Context) ([]domain.GraphEdge, error)

	CreatePullRequest(context.Context, string, string, string) (domain.PullRequest, error)
	DeletePullRequest(context.Context, string) error
	GetPullRequest(context.Context, string) (domain.PullRequest, error)
	ListPullRequests(context.Context, string, string) ([]domain.PullRequest, error)
	UpdatePullRequest(context.Context, string, string, string) (domain.PullRequest, error)
	ClosePullRequest(context.Context, string) (domain.PullRequest, error)
	AcceptPullRequest(context.Context, string, string) (domain.PullRequest, domain.CompletionRecord, domain.Issue, error)
	ListCompletions(context.Context, string) ([]domain.CompletionRecord, error)

	CreateConcept(context.Context, string, string) (domain.Concept, error)
	DeleteConcept(context.Context, string) error
	GetConcept(context.Context, string) (domain.Concept, error)
	ListConcepts(context.Context) ([]domain.Concept, error)
	CreateRelation(context.Context, string, string, string, string, string) (domain.Relation, error)
	LinkIssueConcept(context.Context, string, string) error

	CreateLabel(context.Context, string, string, string, string, string) (domain.Label, error)
	DeleteLabel(context.Context, string) error
	ListLabels(context.Context, string) ([]domain.Label, error)
	ListIssueLabels(context.Context) (map[string][]domain.Label, error)
	AssignLabel(context.Context, string, string) error
	CreateMilestone(context.Context, string, string, string, string, *string) (domain.Milestone, error)
	DeleteMilestone(context.Context, string) error
	ListMilestones(context.Context, string, string) ([]domain.Milestone, error)
	ListIssueMilestones(context.Context) (map[string][]domain.Milestone, error)
	AssignMilestone(context.Context, string, string) error

	ListGraph(context.Context) (domain.Graph, error)
	ListActivity(context.Context, string, string, int) ([]domain.ActivityEvent, error)
	RecordActivity(context.Context, string, string, string, string) error

	ImportLegacy(context.Context, string) (LegacyImportResult, error)

	SeedDemo(context.Context) (domain.SeedReport, error)
	ListNotes(context.Context) ([]domain.Note, error)
	ListNoteLabels(context.Context) ([]domain.NoteLabel, error)
	CreateNote(context.Context, string, string) (domain.Note, error)
	CreateNoteWithLabels(context.Context, string, string, []string) (domain.Note, error)
	UpdateNote(context.Context, string, string, string) (domain.Note, error)
	UpdateNoteWithLabels(context.Context, string, string, string, []string) (domain.Note, error)
	DeleteNote(context.Context, string) error
	LinkNote(context.Context, string, string, string, bool) error
	ListQuotes(context.Context) ([]domain.Quote, error)
	CreateQuote(context.Context, string, string) (domain.Quote, error)
	CreateQuoteWithSource(context.Context, domain.Quote) (domain.Quote, error)
	DeleteQuote(context.Context, string) error
	NextQuote(context.Context) (domain.Quote, error)
	GetWorkspaceProfile(context.Context) (domain.WorkspaceProfile, error)
	UpdateWorkspaceProfile(context.Context, domain.WorkspaceProfile) (domain.WorkspaceProfile, error)
	ListWorkspacePins(context.Context) ([]domain.WorkspacePin, error)
	ToggleWorkspacePin(context.Context, string, string, string, string) (bool, error)
	ListScheduledJobs(context.Context) ([]domain.ScheduledJob, error)
	CreateScheduledJob(context.Context, domain.ScheduledJob) (domain.ScheduledJob, error)
	UpdateScheduledJob(context.Context, domain.ScheduledJob) (domain.ScheduledJob, error)
	DeleteScheduledJob(context.Context, string) error
	ToggleScheduledJob(context.Context, string) (domain.ScheduledJob, error)
	RunScheduledJobNow(context.Context, string, time.Time, bool) error
	ProcessDueJobs(context.Context, time.Time, bool) (int, error)
	ListNotifications(context.Context, int) ([]domain.Notification, error)
	CreateNotification(context.Context, string, string, string) (domain.Notification, error)
	DeleteNotification(context.Context, string) error
	MarkNotificationsRead(context.Context) error
	SetAppSetting(context.Context, string, string) error
	GetAppSetting(context.Context, string) (string, error)
}

type LegacyImportResult struct {
	Repository        string `json:"repository"`
	ImportedTasks     int    `json:"imported_tasks"`
	ImportedConcepts  int    `json:"imported_concepts"`
	ImportedRelations int    `json:"imported_relations"`
	ImportedDoneTasks int    `json:"imported_done_tasks_as_synthetic_completions"`
	SkippedTasks      int    `json:"skipped_tasks"`
}
