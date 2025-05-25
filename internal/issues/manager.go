package issues

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/go-github/v57/github"
	"github.com/itcaat/catapult/internal/config"
)

// Manager implements the IssueManager interface
type Manager struct {
	client    *github.Client
	owner     string
	repo      string
	tracker   *Tracker
	templates *Templates
	config    *config.IssueConfig
	logger    *log.Logger
}

// NewManager creates a new issue manager instance
func NewManager(client *github.Client, owner string, cfg *config.IssueConfig, logger *log.Logger) (*Manager, error) {
	if cfg == nil {
		return nil, fmt.Errorf("issue config cannot be nil")
	}

	// Create tracker storage path
	home, err := getHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	trackerPath := filepath.Join(home, ".catapult", "issues.json")
	tracker := NewTracker(trackerPath)

	// Load existing tracked issues
	if err := tracker.Load(); err != nil {
		logger.Printf("Warning: failed to load issue tracker: %v", err)
	}

	manager := &Manager{
		client:    client,
		owner:     owner,
		repo:      cfg.Repository,
		tracker:   tracker,
		templates: NewTemplates(cfg),
		config:    cfg,
		logger:    logger,
	}

	return manager, nil
}

// CreateIssue creates a new GitHub issue for the given problem
func (m *Manager) CreateIssue(ctx context.Context, issue *Issue) (*GitHubIssue, error) {
	if !m.config.Enabled || !m.config.AutoCreate {
		return nil, fmt.Errorf("issue creation is disabled")
	}

	// Generate issue ID if not set
	if issue.ID == "" {
		issue.ID = m.tracker.GenerateIssueID(issue)
	}

	// Generate the final templated title to search for
	content, templateErr := m.templates.Generate(issue)
	if templateErr != nil {
		m.logger.Printf("Warning: failed to generate template for title comparison: %v", templateErr)
		// Fallback to original title
		content = &IssueContent{Title: issue.Title}
	}

	// Check for existing issues with the same final title
	m.logger.Printf("Checking for existing issues with same title...")
	existing, err := m.findIssueByTitle(ctx, content.Title)
	if err != nil {
		m.logger.Printf("Warning: failed to check for existing issues: %v", err)
	}
	m.logger.Printf("Title check completed, existing: %v", existing != nil)

	if existing != nil {
		m.logger.Printf("Found existing issue with same title, adding comment instead of creating new")
		// Add comment to existing issue instead of creating new one
		return m.addCommentToIssue(ctx, existing, issue)
	}

	// Check if we've hit the max open issues limit
	openIssues := m.tracker.GetAllOpen()
	if len(openIssues) >= m.config.MaxOpenIssues {
		return nil, fmt.Errorf("maximum open issues limit reached (%d)", m.config.MaxOpenIssues)
	}

	// Reuse the content generated earlier for title comparison
	if templateErr != nil {
		// If template generation failed earlier, try again
		var err error
		content, err = m.templates.Generate(issue)
		if err != nil {
			return nil, fmt.Errorf("failed to generate issue content: %w", err)
		}
	}

	// Create GitHub issue
	githubIssue, err := m.createGitHubIssue(ctx, content)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub issue: %w", err)
	}

	// Track locally
	if err := m.tracker.Track(issue, githubIssue); err != nil {
		m.logger.Printf("Warning: failed to track issue locally: %v", err)
	}

	m.logger.Printf("Created issue #%d: %s", githubIssue.Number, githubIssue.Title)
	return githubIssue, nil
}

// UpdateIssue updates an existing GitHub issue
func (m *Manager) UpdateIssue(ctx context.Context, issueNumber int, update *IssueUpdate) error {
	if !m.config.Enabled {
		return fmt.Errorf("issue management is disabled")
	}

	issueRequest := &github.IssueRequest{}

	if update.Body != nil {
		issueRequest.Body = update.Body
	}

	if update.State != nil {
		issueRequest.State = update.State
	}

	if len(update.Labels) > 0 {
		issueRequest.Labels = &update.Labels
	}

	_, _, err := m.client.Issues.Edit(ctx, m.owner, m.repo, issueNumber, issueRequest)
	if err != nil {
		return fmt.Errorf("failed to update issue #%d: %w", issueNumber, err)
	}

	m.logger.Printf("Updated issue #%d", issueNumber)
	return nil
}

// ResolveIssue marks an issue as resolved and closes it
func (m *Manager) ResolveIssue(ctx context.Context, issueNumber int, resolution string) error {
	if !m.config.Enabled || !m.config.AutoResolve {
		return fmt.Errorf("issue resolution is disabled")
	}

	// Get current issue to append resolution
	issue, _, err := m.client.Issues.Get(ctx, m.owner, m.repo, issueNumber)
	if err != nil {
		return fmt.Errorf("failed to get issue #%d: %w", issueNumber, err)
	}

	// Append resolution to the issue body
	resolvedBody := issue.GetBody() + "\n\n---\n\n## ✅ Resolution\n\n" + resolution +
		"\n\n*This issue was automatically resolved by Catapult at " + time.Now().Format(time.RFC3339) + "*"

	// Close the issue
	state := "closed"
	update := &IssueUpdate{
		Body:  &resolvedBody,
		State: &state,
	}

	if err := m.UpdateIssue(ctx, issueNumber, update); err != nil {
		return err
	}

	// Update local tracking
	githubIssue := &GitHubIssue{
		Number:    issueNumber,
		Title:     issue.GetTitle(),
		Body:      resolvedBody,
		State:     "closed",
		Labels:    getLabelsFromIssue(issue),
		CreatedAt: issue.GetCreatedAt().Time,
		UpdatedAt: time.Now(),
		HTMLURL:   issue.GetHTMLURL(),
	}

	// Find the tracked issue and update it
	for _, tracked := range m.tracker.GetAllOpen() {
		if tracked.GitHubIssue.Number == issueNumber {
			if err := m.tracker.Update(tracked.LocalIssue.ID, githubIssue, StatusResolved); err != nil {
				m.logger.Printf("Warning: failed to update local tracking: %v", err)
			}
			break
		}
	}

	m.logger.Printf("Resolved issue #%d: %s", issueNumber, issue.GetTitle())
	return nil
}

// GetOpenIssues retrieves all open issues from GitHub
func (m *Manager) GetOpenIssues(ctx context.Context) ([]*GitHubIssue, error) {
	if !m.config.Enabled {
		return nil, fmt.Errorf("issue management is disabled")
	}

	// List issues with catapult label
	opts := &github.IssueListByRepoOptions{
		State:  "open",
		Labels: []string{"catapult"},
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	issues, _, err := m.client.Issues.ListByRepo(ctx, m.owner, m.repo, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list issues: %w", err)
	}

	var result []*GitHubIssue
	for _, issue := range issues {
		result = append(result, &GitHubIssue{
			Number:    issue.GetNumber(),
			Title:     issue.GetTitle(),
			Body:      issue.GetBody(),
			State:     issue.GetState(),
			Labels:    getLabelsFromIssue(issue),
			CreatedAt: issue.GetCreatedAt().Time,
			UpdatedAt: issue.GetUpdatedAt().Time,
			HTMLURL:   issue.GetHTMLURL(),
		})
	}

	return result, nil
}

// CheckResolution determines if an issue should be considered resolved
func (m *Manager) CheckResolution(ctx context.Context, issue *Issue) (bool, error) {
	// This is a placeholder implementation
	// In a real implementation, this would check if the underlying problem
	// that caused the issue has been resolved

	// For now, we'll consider an issue resolved if it's older than the resolution check interval
	// and no similar issues have been created recently
	if time.Since(issue.Timestamp) < m.config.ResolutionCheckInterval {
		return false, nil
	}

	// Check if there are any recent similar issues
	similar, err := m.FindSimilarIssue(ctx, issue)
	if err != nil {
		return false, err
	}

	// If no similar issues found, consider it resolved
	return similar == nil, nil
}

// FindSimilarIssue looks for existing issues that are similar to the given issue
func (m *Manager) FindSimilarIssue(ctx context.Context, issue *Issue) (*GitHubIssue, error) {
	m.logger.Printf("FindSimilarIssue: Starting search for similar issues")

	// First check local tracker
	m.logger.Printf("FindSimilarIssue: Checking local tracker")
	tracked, err := m.tracker.FindSimilar(issue)
	if err != nil {
		m.logger.Printf("FindSimilarIssue: Error in local tracker: %v", err)
		return nil, err
	}
	m.logger.Printf("FindSimilarIssue: Local tracker check completed, found: %v", tracked != nil)

	if tracked != nil && tracked.Status != StatusClosed {
		m.logger.Printf("FindSimilarIssue: Returning local tracked issue")
		return tracked.GitHubIssue, nil
	}

	// Skip GitHub search for now to avoid hanging - rely on local tracker only
	m.logger.Printf("FindSimilarIssue: Skipping GitHub search (using local tracker only)")
	return nil, nil
}

// Cleanup performs maintenance tasks like removing old issues
func (m *Manager) Cleanup() error {
	return m.tracker.Cleanup()
}

// createGitHubIssue creates an issue on GitHub
func (m *Manager) createGitHubIssue(ctx context.Context, content *IssueContent) (*GitHubIssue, error) {
	issueRequest := &github.IssueRequest{
		Title:  &content.Title,
		Body:   &content.Body,
		Labels: &content.Labels,
	}

	if len(m.config.Assignees) > 0 {
		issueRequest.Assignees = &m.config.Assignees
	}

	issue, _, err := m.client.Issues.Create(ctx, m.owner, m.repo, issueRequest)
	if err != nil {
		return nil, err
	}

	return &GitHubIssue{
		Number:    issue.GetNumber(),
		Title:     issue.GetTitle(),
		Body:      issue.GetBody(),
		State:     issue.GetState(),
		Labels:    getLabelsFromIssue(issue),
		CreatedAt: issue.GetCreatedAt().Time,
		UpdatedAt: issue.GetUpdatedAt().Time,
		HTMLURL:   issue.GetHTMLURL(),
	}, nil
}

// updateExistingIssue updates an existing issue instead of creating a new one
func (m *Manager) updateExistingIssue(ctx context.Context, existing *GitHubIssue, newIssue *Issue) (*GitHubIssue, error) {
	// Generate new content
	content, err := m.templates.Generate(newIssue)
	if err != nil {
		return nil, err
	}

	// Append new occurrence to existing issue
	updatedBody := existing.Body + "\n\n---\n\n## 🔄 Additional Occurrence\n\n" +
		"**Timestamp**: " + newIssue.Timestamp.Format(time.RFC3339) + "\n\n" +
		content.Body

	update := &IssueUpdate{
		Body: &updatedBody,
	}

	if err := m.UpdateIssue(ctx, existing.Number, update); err != nil {
		return nil, err
	}

	// Update local tracking
	updatedIssue := *existing
	updatedIssue.Body = updatedBody
	updatedIssue.UpdatedAt = time.Now()

	if err := m.tracker.Update(newIssue.ID, &updatedIssue, StatusUpdated); err != nil {
		m.logger.Printf("Warning: failed to update local tracking: %v", err)
	}

	return &updatedIssue, nil
}

// searchGitHubForSimilar searches GitHub for similar issues
func (m *Manager) searchGitHubForSimilar(ctx context.Context, issue *Issue) (*GitHubIssue, error) {
	// Search for issues with catapult label (more reliable than category-specific labels)
	query := fmt.Sprintf("repo:%s/%s is:open label:catapult", m.owner, m.repo)
	m.logger.Printf("searchGitHubForSimilar: Executing search query: %s", query)

	opts := &github.SearchOptions{
		ListOptions: github.ListOptions{
			PerPage: 10,
		},
	}

	m.logger.Printf("searchGitHubForSimilar: Calling GitHub Search API...")
	result, _, err := m.client.Search.Issues(ctx, query, opts)
	m.logger.Printf("searchGitHubForSimilar: GitHub Search API returned, error: %v", err)
	if err != nil {
		return nil, err
	}

	// Check each result for similarity
	for _, ghIssue := range result.Issues {
		if m.isGitHubIssueSimilar(ghIssue, issue) {
			return &GitHubIssue{
				Number:    ghIssue.GetNumber(),
				Title:     ghIssue.GetTitle(),
				Body:      ghIssue.GetBody(),
				State:     ghIssue.GetState(),
				Labels:    getLabelsFromIssue(ghIssue),
				CreatedAt: ghIssue.GetCreatedAt().Time,
				UpdatedAt: ghIssue.GetUpdatedAt().Time,
				HTMLURL:   ghIssue.GetHTMLURL(),
			}, nil
		}
	}

	return nil, nil
}

// isGitHubIssueSimilar checks if a GitHub issue is similar to a local issue
func (m *Manager) isGitHubIssueSimilar(ghIssue *github.Issue, localIssue *Issue) bool {
	body := ghIssue.GetBody()

	// Check for similar error messages
	if localIssue.ErrorMsg != "" && strings.Contains(body, localIssue.ErrorMsg) {
		return true
	}

	// Check for similar file names
	for _, file := range localIssue.Files {
		if strings.Contains(body, file) {
			return true
		}
	}

	return false
}

// getLabelsFromIssue extracts label names from a GitHub issue
func getLabelsFromIssue(issue *github.Issue) []string {
	var labels []string
	for _, label := range issue.Labels {
		labels = append(labels, label.GetName())
	}
	return labels
}

// findIssueByTitle looks for an existing issue with the exact same title (open or closed)
func (m *Manager) findIssueByTitle(ctx context.Context, title string) (*GitHubIssue, error) {
	m.logger.Printf("findIssueByTitle: Searching for issue with title: %s", title)

	// First check local tracker for issues with matching titles (including closed ones)
	allTracked := m.tracker.GetAll()
	for _, tracked := range allTracked {
		if tracked.GitHubIssue.Title == title {
			m.logger.Printf("findIssueByTitle: Found matching issue in local tracker #%d (status: %s)", tracked.GitHubIssue.Number, tracked.Status)
			return tracked.GitHubIssue, nil
		}
	}

	// If not found in local tracker, skip GitHub API call to avoid hanging
	// This means we might miss some issues, but it's better than hanging
	m.logger.Printf("findIssueByTitle: No matching issue found in local tracker, skipping GitHub API call")
	return nil, nil
}

// addCommentToIssue adds a comment to an existing issue and reopens it if closed
func (m *Manager) addCommentToIssue(ctx context.Context, existing *GitHubIssue, newIssue *Issue) (*GitHubIssue, error) {
	m.logger.Printf("addCommentToIssue: Adding comment to issue #%d (current state: %s)", existing.Number, existing.State)

	// Check if issue is closed and reopen it
	wasClosedBefore := existing.State == "closed"
	if wasClosedBefore {
		m.logger.Printf("addCommentToIssue: Issue #%d is closed, reopening it", existing.Number)

		state := "open"
		update := &IssueUpdate{
			State: &state,
		}

		if err := m.UpdateIssue(ctx, existing.Number, update); err != nil {
			return nil, fmt.Errorf("failed to reopen issue #%d: %w", existing.Number, err)
		}

		// Update the existing issue state
		existing.State = "open"
		m.logger.Printf("addCommentToIssue: Successfully reopened issue #%d", existing.Number)
	}

	// Generate comment content
	var commentBody string
	if wasClosedBefore {
		commentBody = fmt.Sprintf("## 🔄 Issue Reopened - Error Occurred Again\n\n"+
			"**Timestamp**: %s\n"+
			"**File**: %s\n"+
			"**Error**: %s\n\n"+
			"This issue was previously closed, but the same error has occurred again. "+
			"The issue has been automatically reopened for further investigation.",
			newIssue.Timestamp.Format("2006-01-02 15:04:05"),
			strings.Join(newIssue.Files, ", "),
			newIssue.ErrorMsg)
	} else {
		commentBody = fmt.Sprintf("## 🔄 Additional Occurrence\n\n"+
			"**Timestamp**: %s\n"+
			"**File**: %s\n"+
			"**Error**: %s\n\n"+
			"This error occurred again. The issue is still present and needs attention.",
			newIssue.Timestamp.Format("2006-01-02 15:04:05"),
			strings.Join(newIssue.Files, ", "),
			newIssue.ErrorMsg)
	}

	// Create comment
	comment := &github.IssueComment{
		Body: &commentBody,
	}

	_, _, err := m.client.Issues.CreateComment(ctx, m.owner, m.repo, existing.Number, comment)
	if err != nil {
		return nil, fmt.Errorf("failed to create comment on issue #%d: %w", existing.Number, err)
	}

	// Update local tracking status
	var newStatus IssueStatus
	if wasClosedBefore {
		newStatus = StatusOpen // Issue was reopened
	} else {
		newStatus = StatusUpdated // Issue was already open, just updated
	}

	if err := m.tracker.Update(newIssue.ID, existing, newStatus); err != nil {
		m.logger.Printf("Warning: failed to update local tracking: %v", err)
	}

	m.logger.Printf("addCommentToIssue: Successfully added comment to issue #%d", existing.Number)
	return existing, nil
}

// getHomeDir returns the user's home directory
func getHomeDir() (string, error) {
	// This is a simple implementation - in production you might want to use os.UserHomeDir()
	// but for consistency with the rest of the codebase, we'll implement it here
	home := os.Getenv("HOME")
	if home == "" {
		return "", fmt.Errorf("HOME environment variable not set")
	}
	return home, nil
}
