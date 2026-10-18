package domain

import "time"

type IssueStatus string

const (
	IssueOpen       IssueStatus = "open"
	IssueInProgress IssueStatus = "in_progress"
	IssueDone       IssueStatus = "done"
	IssueCancelled  IssueStatus = "cancelled"
)

type PRStatus string

const (
	PRPending  PRStatus = "open"
	PRAccepted PRStatus = "accepted"
	PRClosed   PRStatus = "closed"
)

type Repository struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	DefaultBranchID string    `json:"default_branch_id"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type Branch struct {
	ID                     string    `json:"id"`
	RepositoryID           string    `json:"repository_id"`
	Name                   string    `json:"name"`
	IsDefault              bool      `json:"is_default"`
	BaseBranchID           *string   `json:"base_branch_id,omitempty"`
	ForkedFromCompletionID *string   `json:"forked_from_completion_id,omitempty"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

type Issue struct {
	ID              string      `json:"id"`
	Number          int64       `json:"number"`
	RepositoryID    string      `json:"repository_id"`
	RepositoryName  string      `json:"repository_name,omitempty"`
	Title           string      `json:"title"`
	Description     string      `json:"description"`
	Status          IssueStatus `json:"status"`
	Priority        string      `json:"priority,omitempty"`
	DueDate         *string     `json:"due_date,omitempty"`
	ParentIssueID   *string     `json:"parent_issue_id,omitempty"`
	ParentReference string      `json:"parent_reference,omitempty"`
	CreatedAt       time.Time   `json:"created_at"`
	UpdatedAt       time.Time   `json:"updated_at"`
	ClosedAt        *time.Time  `json:"closed_at,omitempty"`
	Blocked         bool        `json:"blocked"`
	Blockers        []Blocker   `json:"blockers,omitempty"`
}

type Blocker struct {
	IssueID        string      `json:"issue_id"`
	RepositoryName string      `json:"repository_name"`
	Number         int64       `json:"number"`
	Title          string      `json:"title"`
	Status         IssueStatus `json:"status"`
	Reason         string      `json:"reason"`
}

type Project struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Concept struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type PullRequest struct {
	ID               string     `json:"id"`
	Number           int64      `json:"number"`
	RepositoryID     string     `json:"repository_id"`
	RepositoryName   string     `json:"repository_name,omitempty"`
	IssueID          string     `json:"issue_id"`
	IssueNumber      int64      `json:"issue_number"`
	IssueTitle       string     `json:"issue_title,omitempty"`
	Title            string     `json:"title"`
	Description      string     `json:"description"`
	Status           PRStatus   `json:"status"`
	TargetBranchID   string     `json:"target_branch_id"`
	TargetBranchName string     `json:"target_branch_name,omitempty"`
	SourceBranchID   *string    `json:"source_branch_id,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	AcceptedAt       *time.Time `json:"accepted_at,omitempty"`
}

type CompletionRecord struct {
	ID                 string    `json:"id"`
	RepositoryID       string    `json:"repository_id"`
	RepositoryName     string    `json:"repository_name,omitempty"`
	BranchID           string    `json:"branch_id"`
	BranchName         string    `json:"branch_name,omitempty"`
	IssueID            string    `json:"issue_id"`
	IssueNumber        int64     `json:"issue_number"`
	PullRequestID      string    `json:"pull_request_id"`
	Summary            string    `json:"summary"`
	Sequence           int64     `json:"sequence"`
	ParentCompletionID *string   `json:"parent_completion_id,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
}

type Relation struct {
	ID         string    `json:"id"`
	SourceType string    `json:"source_type"`
	SourceID   string    `json:"source_id"`
	TargetType string    `json:"target_type"`
	TargetID   string    `json:"target_id"`
	Type       string    `json:"type"`
	CreatedAt  time.Time `json:"created_at"`
}

type Label struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	Color          string    `json:"color"`
	ScopeType      string    `json:"scope_type"`
	RepositoryID   *string   `json:"repository_id,omitempty"`
	RepositoryName string    `json:"repository_name,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

type Milestone struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	ScopeType   string    `json:"scope_type"`
	ScopeID     string    `json:"scope_id"`
	ScopeName   string    `json:"scope_name,omitempty"`
	DueDate     *string   `json:"due_date,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ActivityEvent struct {
	ID         string    `json:"id"`
	EntityType string    `json:"entity_type"`
	EntityID   string    `json:"entity_id"`
	Action     string    `json:"action"`
	Payload    string    `json:"payload,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type GraphNode struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Label     string `json:"label"`
	Status    string `json:"status,omitempty"`
	Reference string `json:"reference,omitempty"`
}

type GraphEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"`
}

type Graph struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

type RepositorySetting struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}
