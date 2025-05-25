package issues

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Tracker manages local tracking of issues and their GitHub counterparts
type Tracker struct {
	storage     string
	cache       map[string]*TrackedIssue
	mutex       sync.RWMutex
	maxCacheAge time.Duration
}

// NewTracker creates a new issue tracker with the specified storage path
func NewTracker(storagePath string) *Tracker {
	return &Tracker{
		storage:     storagePath,
		cache:       make(map[string]*TrackedIssue),
		maxCacheAge: 24 * time.Hour, // Keep issues in cache for 24 hours
	}
}

// Track adds a new issue to the tracker
func (t *Tracker) Track(issue *Issue, githubIssue *GitHubIssue) error {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	tracked := &TrackedIssue{
		LocalIssue:  issue,
		GitHubIssue: githubIssue,
		LastUpdated: time.Now(),
		Status:      StatusOpen,
	}

	t.cache[issue.ID] = tracked
	return t.persistToStorage()
}

// Update modifies an existing tracked issue
func (t *Tracker) Update(issueID string, githubIssue *GitHubIssue, status IssueStatus) error {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	tracked, exists := t.cache[issueID]
	if !exists {
		return fmt.Errorf("issue %s not found in tracker", issueID)
	}

	tracked.GitHubIssue = githubIssue
	tracked.Status = status
	tracked.LastUpdated = time.Now()

	return t.persistToStorage()
}

// FindSimilar looks for existing issues that are similar to the given issue
func (t *Tracker) FindSimilar(issue *Issue) (*TrackedIssue, error) {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	for _, tracked := range t.cache {
		if t.isSimilar(issue, tracked.LocalIssue) &&
			tracked.Status != StatusClosed {
			return tracked, nil
		}
	}

	return nil, nil
}

// GetTracked retrieves a tracked issue by ID
func (t *Tracker) GetTracked(issueID string) (*TrackedIssue, bool) {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	tracked, exists := t.cache[issueID]
	return tracked, exists
}

// GetAllOpen returns all tracked issues that are currently open
func (t *Tracker) GetAllOpen() []*TrackedIssue {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	var openIssues []*TrackedIssue
	for _, tracked := range t.cache {
		if tracked.Status == StatusOpen || tracked.Status == StatusUpdated {
			openIssues = append(openIssues, tracked)
		}
	}

	return openIssues
}

// GetAll returns all tracked issues regardless of status
func (t *Tracker) GetAll() []*TrackedIssue {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	var allIssues []*TrackedIssue
	for _, tracked := range t.cache {
		allIssues = append(allIssues, tracked)
	}

	return allIssues
}

// Remove removes an issue from tracking
func (t *Tracker) Remove(issueID string) error {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	delete(t.cache, issueID)
	return t.persistToStorage()
}

// Load loads tracked issues from persistent storage
func (t *Tracker) Load() error {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	// Ensure storage directory exists
	if err := os.MkdirAll(filepath.Dir(t.storage), 0755); err != nil {
		return fmt.Errorf("failed to create storage directory: %w", err)
	}

	// Try to load existing data
	data, err := os.ReadFile(t.storage)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist yet, start with empty cache
			return nil
		}
		return fmt.Errorf("failed to read tracker storage: %w", err)
	}

	var storedIssues map[string]*TrackedIssue
	if err := json.Unmarshal(data, &storedIssues); err != nil {
		return fmt.Errorf("failed to unmarshal tracker data: %w", err)
	}

	// Filter out old issues
	now := time.Now()
	for id, tracked := range storedIssues {
		if now.Sub(tracked.LastUpdated) <= t.maxCacheAge {
			t.cache[id] = tracked
		}
	}

	return nil
}

// Cleanup removes old issues from the tracker
func (t *Tracker) Cleanup() error {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	now := time.Now()
	changed := false

	for id, tracked := range t.cache {
		// Remove issues that are closed and older than cache age
		if (tracked.Status == StatusClosed || tracked.Status == StatusResolved) &&
			now.Sub(tracked.LastUpdated) > t.maxCacheAge {
			delete(t.cache, id)
			changed = true
		}
	}

	if changed {
		return t.persistToStorage()
	}

	return nil
}

// GenerateIssueID creates a unique ID for an issue based on its content
func (t *Tracker) GenerateIssueID(issue *Issue) string {
	// Create a hash based on category, error message, and files
	hasher := sha256.New()
	hasher.Write([]byte(string(issue.Category)))
	hasher.Write([]byte(issue.ErrorMsg))
	hasher.Write([]byte(strings.Join(issue.Files, ",")))

	hash := fmt.Sprintf("%x", hasher.Sum(nil))
	return fmt.Sprintf("catapult-%s-%s", issue.Category, hash[:8])
}

// isSimilar determines if two issues are similar enough to be considered the same
func (t *Tracker) isSimilar(issue1, issue2 *Issue) bool {
	// Same category is required
	if issue1.Category != issue2.Category {
		return false
	}

	// Check for similar files affected
	if len(issue1.Files) > 0 && len(issue2.Files) > 0 {
		if hasCommonFiles(issue1.Files, issue2.Files) {
			return true
		}
	}

	// Check for similar error messages
	if issue1.ErrorMsg != "" && issue2.ErrorMsg != "" {
		return isSimilarError(issue1.ErrorMsg, issue2.ErrorMsg)
	}

	// If no files or errors to compare, consider them different
	return false
}

// persistToStorage saves the current cache to persistent storage
func (t *Tracker) persistToStorage() error {
	data, err := json.MarshalIndent(t.cache, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tracker data: %w", err)
	}

	// Write with secure permissions
	if err := os.WriteFile(t.storage, data, 0600); err != nil {
		return fmt.Errorf("failed to write tracker storage: %w", err)
	}

	return nil
}

// hasCommonFiles checks if two file lists have any files in common
func hasCommonFiles(files1, files2 []string) bool {
	fileSet := make(map[string]bool)
	for _, file := range files1 {
		fileSet[file] = true
	}

	for _, file := range files2 {
		if fileSet[file] {
			return true
		}
	}

	return false
}

// isSimilarError determines if two error messages are similar
func isSimilarError(err1, err2 string) bool {
	// Normalize error messages for comparison
	norm1 := strings.ToLower(strings.TrimSpace(err1))
	norm2 := strings.ToLower(strings.TrimSpace(err2))

	// Check for exact match
	if norm1 == norm2 {
		return true
	}

	// Check for substring match (one contains the other)
	if strings.Contains(norm1, norm2) || strings.Contains(norm2, norm1) {
		return true
	}

	// Check for common error patterns
	commonPatterns := []string{
		"permission denied",
		"access denied",
		"network error",
		"connection refused",
		"timeout",
		"not found",
		"conflict",
		"authentication failed",
	}

	for _, pattern := range commonPatterns {
		if strings.Contains(norm1, pattern) && strings.Contains(norm2, pattern) {
			return true
		}
	}

	return false
}
