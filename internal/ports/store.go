package ports

import (
	"context"
	"idios/internal/domain"
	"time"
)

type CoreStore interface {
	Close() error
	Ping(context.Context) error
}

type RepositoryStore interface {
	CreateRepository(context.Context, string, string) (domain.Repository, error)
	CreateRepositoryWithDefaultBranch(context.Context, string, string, string) (domain.Repository, error)
	UpdateRepositoryDescription(context.Context, string, string) (domain.Repository, error)
	DeleteRepository(context.Context, string) error
	GetRepository(context.Context, string) (domain.Repository, error)
	ListRepositories(context.Context) ([]domain.Repository, error)
	ListBranches(context.Context, string) ([]domain.Branch, error)
	CreateBranch(context.Context, string, string, string) (domain.Branch, error)
	AssignIssueBranch(context.Context, string, string) error
	AssignIssueParent(context.Context, string, string) error
	SetRepositorySetting(context.Context, string, string, string) error
	GetRepositorySettings(context.Context, string) ([]domain.RepositorySetting, error)
	ListRepositoryFiles(context.Context, string) ([]domain.RepositoryFile, error)
	SaveRepositoryFile(context.Context, string, string, string) (domain.RepositoryFile, error)
	DeleteRepositoryFile(context.Context, string, string) error
	ListRepositoryTags(context.Context, string) ([]domain.RepositoryTag, error)
	CreateRepositoryTag(context.Context, string, string, string) (domain.RepositoryTag, error)
	GetRepositoryTag(context.Context, string, string) (domain.RepositoryTag, error)
	GetRepositoryTagFiles(context.Context, string) ([]domain.RepositoryFile, error)
	CompareRepositoryTags(context.Context, string, string) ([]domain.TagFileChange, error)
	ListTagHistory(context.Context, string) ([]domain.CompletionRecord, error)
}

type ProjectStore interface {
	CreateProject(context.Context, string, string) (domain.Project, error)
	DeleteProject(context.Context, string) error
	GetProject(context.Context, string) (domain.Project, error)
	ListProjects(context.Context) ([]domain.Project, error)
	ListProjectRepositories(context.Context, string) ([]domain.ProjectRepository, error)
	LinkRepositoryToProject(context.Context, string, string, string, bool) error
	UnlinkRepositoryFromProject(context.Context, string, string) error
	SetPrimaryProjectRepository(context.Context, string, string) error
	AddIssueToProject(context.Context, string, string) error
	ListProjectIssues(context.Context, string) ([]domain.Issue, error)
}

type IssueStore interface {
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
}

type PullRequestStore interface {
	CreatePullRequest(context.Context, string, string, string) (domain.PullRequest, error)
	CreatePullRequestWithBranches(context.Context, string, string, string, string, string, bool) (domain.PullRequest, error)
	CreatePullRequestWithIssues(context.Context, []string, string, string, string, string, bool) (domain.PullRequest, error)
	ListPullRequestIssues(context.Context, string) ([]domain.Issue, error)
	MarkPullRequestReady(context.Context, string) (domain.PullRequest, error)
	DeletePullRequest(context.Context, string) error
	GetPullRequest(context.Context, string) (domain.PullRequest, error)
	ListPullRequests(context.Context, string, string) ([]domain.PullRequest, error)
	UpdatePullRequest(context.Context, string, string, string) (domain.PullRequest, error)
	ClosePullRequest(context.Context, string) (domain.PullRequest, error)
	AcceptPullRequest(context.Context, string, string) (domain.PullRequest, domain.CompletionRecord, domain.Issue, error)
	ListCompletions(context.Context, string) ([]domain.CompletionRecord, error)
}

type ConceptStore interface {
	CreateConcept(context.Context, string, string) (domain.Concept, error)
	DeleteConcept(context.Context, string) error
	GetConcept(context.Context, string) (domain.Concept, error)
	ListConcepts(context.Context) ([]domain.Concept, error)
	CreateRelation(context.Context, string, string, string, string, string) (domain.Relation, error)
	LinkIssueConcept(context.Context, string, string) error
}

type PlanningMetadataStore interface {
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
}

type GraphStore interface {
	ListGraph(context.Context) (domain.Graph, error)
	GetGraphLayout(context.Context) (domain.GraphLayout, error)
	SaveGraphLayout(context.Context, domain.GraphLayout) error
}

type ActivityStore interface {
	ListActivity(context.Context, string, string, int) ([]domain.ActivityEvent, error)
	RecordActivity(context.Context, string, string, string, string) error
	RecordDailyPresence(context.Context, string) error
	ListActivityDays(context.Context, string, string, string) ([]domain.ActivityDayCount, error)
}

type ImportStore interface {
	ImportLegacy(context.Context, string) (LegacyImportResult, error)
}

type DemoStore interface {
	SeedDemo(context.Context) (domain.SeedReport, error)
}

type NotesStore interface {
	ListNotes(context.Context) ([]domain.Note, error)
	ListNoteLabels(context.Context) ([]domain.NoteLabel, error)
	SetNoteLabelColor(context.Context, string, string) error
	CreateNote(context.Context, string, string) (domain.Note, error)
	CreateNoteWithLabels(context.Context, string, string, []string) (domain.Note, error)
	UpdateNote(context.Context, string, string, string) (domain.Note, error)
	UpdateNoteWithLabels(context.Context, string, string, string, []string) (domain.Note, error)
	DeleteNote(context.Context, string) error
	LinkNote(context.Context, string, string, string, bool) error
}

type QuotesStore interface {
	ListQuotes(context.Context) ([]domain.Quote, error)
	CreateQuote(context.Context, string, string) (domain.Quote, error)
	CreateQuoteWithSource(context.Context, domain.Quote) (domain.Quote, error)
	DeleteQuote(context.Context, string) error
	NextQuote(context.Context) (domain.Quote, error)
}

type ProfileStore interface {
	GetWorkspaceProfile(context.Context) (domain.WorkspaceProfile, error)
	UpdateWorkspaceProfile(context.Context, domain.WorkspaceProfile) (domain.WorkspaceProfile, error)
	ListWorkspacePins(context.Context) ([]domain.WorkspacePin, error)
	ToggleWorkspacePin(context.Context, string, string, string, string) (bool, error)
}

type JobsStore interface {
	ListScheduledJobs(context.Context) ([]domain.ScheduledJob, error)
	CreateScheduledJob(context.Context, domain.ScheduledJob) (domain.ScheduledJob, error)
	UpdateScheduledJob(context.Context, domain.ScheduledJob) (domain.ScheduledJob, error)
	DeleteScheduledJob(context.Context, string) error
	ToggleScheduledJob(context.Context, string) (domain.ScheduledJob, error)
	RunScheduledJobNow(context.Context, string, time.Time, bool) error
	ProcessDueJobs(context.Context, time.Time, bool) (int, error)
}

type PlanStore interface {
	ListPlans(context.Context, string, string) ([]domain.Plan, error)
	CreatePlan(context.Context, domain.Plan, []string) (domain.Plan, error)
	UpdatePlan(context.Context, domain.Plan) (domain.Plan, error)
	DeletePlan(context.Context, string) error
	AddPlanTask(context.Context, string, string) (domain.PlanTask, error)
	SetPlanTaskDone(context.Context, string, string, bool) (domain.PlanTask, error)
	CreatePlanReminderNotification(context.Context, string, string, string, string, time.Time) (bool, error)
}

type NotificationStore interface {
	ListNotifications(context.Context, int) ([]domain.Notification, error)
	CreateNotification(context.Context, string, string, string) (domain.Notification, error)
	DeleteNotification(context.Context, string) error
	MarkNotificationsRead(context.Context) error
}

type SettingsStore interface {
	SetAppSetting(context.Context, string, string) error
	GetAppSetting(context.Context, string) (string, error)
}

type ShelfStore interface {
	ListShelfBooks(context.Context) ([]domain.ShelfBook, error)
	CreateShelfReadingSession(context.Context, domain.ShelfReadingSession) (domain.ShelfReadingSession, error)
	ListShelfReadingSessions(context.Context, string) ([]domain.ShelfReadingSession, error)
	ListShelfAnnotations(context.Context, string) ([]domain.ShelfAnnotation, error)
	CreateShelfAnnotation(context.Context, domain.ShelfAnnotation) (domain.ShelfAnnotation, error)
	UpdateShelfAnnotation(context.Context, string, string, string) (domain.ShelfAnnotation, error)
	DeleteShelfAnnotation(context.Context, string) error
	GetShelfBook(context.Context, string) (domain.ShelfBook, error)
	SyncShelfBooks(context.Context, []domain.ShelfBook) error
	SetShelfBookTags(context.Context, string, []string) error
	SetShelfBookProgress(context.Context, string, string, int, int) (domain.ShelfBook, error)
	LinkShelfBookRepository(context.Context, string, string) error
	ListShelfBookRelations(context.Context, string) ([]domain.ShelfBookRelation, error)
	CreateShelfBookRelation(context.Context, string, string, string) (domain.ShelfBookRelation, error)
	DeleteShelfBookRelation(context.Context, string) error
	CreateShelfAsk(context.Context, domain.ShelfAsk) (domain.ShelfAsk, error)
	ListShelfAsks(context.Context, string, string) ([]domain.ShelfAsk, error)
	AnswerShelfAsk(context.Context, string, string) (domain.ShelfAsk, error)
}

type Store interface {
	CoreStore
	RepositoryHubStore
	RepositoryStore
	ProjectStore
	IssueStore
	PullRequestStore
	ConceptStore
	PlanningMetadataStore
	GraphStore
	ActivityStore
	ImportStore
	DemoStore
	NotesStore
	QuotesStore
	ProfileStore
	JobsStore
	PlanStore
	NotificationStore
	SettingsStore
	ShelfStore
}

type LegacyImportResult struct {
	Repository        string `json:"repository"`
	ImportedTasks     int    `json:"imported_tasks"`
	ImportedConcepts  int    `json:"imported_concepts"`
	ImportedRelations int    `json:"imported_relations"`
	ImportedDoneTasks int    `json:"imported_done_tasks_as_synthetic_completions"`
	SkippedTasks      int    `json:"skipped_tasks"`
}
