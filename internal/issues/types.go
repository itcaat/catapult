package issues

import (
	"context"
	"time"
)

// IssueCategory represents different types of sync issues
type IssueCategory string

const (
	CategoryConflict   IssueCategory = "conflict"
	CategoryNetwork    IssueCategory = "network"
	CategoryPermission IssueCategory = "permission"
	CategoryAuth       IssueCategory = "authentication"
	CategoryCorruption IssueCategory = "corruption"
	CategoryQuota      IssueCategory = "quota"
	CategoryUnknown    IssueCategory = "unknown"
)

// Issue represents a synchronization problem that needs to be tracked
type Issue struct {
	ID          string                 `json:"id"`
	Category    IssueCategory          `json:"category"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Files       []string               `json:"files,omitempty"`
	Error       error                  `json:"-"`
	ErrorMsg    string                 `json:"error_message"`
	Timestamp   time.Time              `json:"timestamp"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Resolved    bool                   `json:"resolved"`
}

// GitHubIssue represents an issue as it exists on GitHub
type GitHubIssue struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	State     string    `json:"state"`
	Labels    []string  `json:"labels"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	HTMLURL   string    `json:"html_url"`
}

// IssueContent represents the content to be used when creating/updating GitHub issues
type IssueContent struct {
	Title  string   `json:"title"`
	Body   string   `json:"body"`
	Labels []string `json:"labels"`
}

// IssueUpdate represents an update to an existing issue
type IssueUpdate struct {
	Body   *string  `json:"body,omitempty"`
	State  *string  `json:"state,omitempty"`
	Labels []string `json:"labels,omitempty"`
}

// IssueStatus represents the current status of a tracked issue
type IssueStatus string

const (
	StatusOpen     IssueStatus = "open"
	StatusUpdated  IssueStatus = "updated"
	StatusResolved IssueStatus = "resolved"
	StatusClosed   IssueStatus = "closed"
)

// TrackedIssue represents an issue that is being tracked locally
type TrackedIssue struct {
	LocalIssue  *Issue       `json:"local_issue"`
	GitHubIssue *GitHubIssue `json:"github_issue"`
	LastUpdated time.Time    `json:"last_updated"`
	Status      IssueStatus  `json:"status"`
}

// IssueManager defines the interface for managing GitHub issues
type IssueManager interface {
	// CreateIssue creates a new GitHub issue for the given problem
	CreateIssue(ctx context.Context, issue *Issue) (*GitHubIssue, error)

	// UpdateIssue updates an existing GitHub issue
	UpdateIssue(ctx context.Context, issueNumber int, update *IssueUpdate) error

	// ResolveIssue marks an issue as resolved and closes it
	ResolveIssue(ctx context.Context, issueNumber int, resolution string) error

	// GetOpenIssues retrieves all open issues from GitHub
	GetOpenIssues(ctx context.Context) ([]*GitHubIssue, error)

	// CheckResolution determines if an issue should be considered resolved
	CheckResolution(ctx context.Context, issue *Issue) (bool, error)

	// FindSimilarIssue looks for existing issues that are similar to the given issue
	FindSimilarIssue(ctx context.Context, issue *Issue) (*GitHubIssue, error)
}
