package repository

import (
	"context"
	"fmt"
	"net/http"
	posixpath "path"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/v57/github"
)

// Custom error types for better user experience

// FileSizeError represents file size limit errors
type FileSizeError struct {
	FilePath string
	FileSize int
	Limit    int
}

func (e *FileSizeError) Error() string {
	sizeMB := float64(e.FileSize) / (1024 * 1024)
	limitMB := float64(e.Limit) / (1024 * 1024)
	return fmt.Sprintf("File '%s' (%.1f MB) exceeds GitHub's %d MB limit. "+
		"Consider using Git LFS, splitting the file, or excluding it from sync",
		e.FilePath, sizeMB, int(limitMB))
}

// GitHubValidationError represents validation errors from GitHub API
type GitHubValidationError struct {
	FilePath string
	Message  string
	Details  string
}

func (e *GitHubValidationError) Error() string {
	return fmt.Sprintf("GitHub validation error for '%s': %s", e.FilePath, e.Message)
}

// GitHubPermissionError represents permission errors
type GitHubPermissionError struct {
	FilePath string
	Message  string
	Details  string
}

func (e *GitHubPermissionError) Error() string {
	return fmt.Sprintf("Permission denied for '%s': %s. Check repository access rights", e.FilePath, e.Message)
}

// GitHubRepositoryError represents repository access errors
type GitHubRepositoryError struct {
	Message string
	Details string
}

func (e *GitHubRepositoryError) Error() string {
	return fmt.Sprintf("Repository error: %s", e.Message)
}

// GitHubAPIError represents general GitHub API errors
type GitHubAPIError struct {
	StatusCode int
	Message    string
	FilePath   string
}

const githubFileSizeLimit = 100 * 1024 * 1024

func (e *GitHubAPIError) Error() string {
	return fmt.Sprintf("GitHub API error (HTTP %d) for '%s': %s", e.StatusCode, e.FilePath, e.Message)
}

// GitHubRateLimitError indicates that the bounded retry policy was exhausted.
type GitHubRateLimitError struct {
	StatusCode int
	Message    string
	RetryAfter time.Duration
}

func (e *GitHubRateLimitError) Error() string {
	return fmt.Sprintf("GitHub API rate limit (HTTP %d): %s; retry after %s", e.StatusCode, e.Message, e.RetryAfter.Round(time.Second))
}

// RemoteFileInfo contains information about a remote file
type RemoteFileInfo struct {
	Path    string
	Content string
	// ContentLoaded distinguishes a deliberately empty file from metadata-only entries.
	ContentLoaded bool
	SHA           string
	Size          int
}

// Repository defines the interface for repository operations
type Repository interface {
	EnsureExists(ctx context.Context) error
	GetDefaultBranch(ctx context.Context) (string, error)
	CreateFile(ctx context.Context, path, content string) error
	GetFile(ctx context.Context, path string) (string, error)
	UpdateFile(ctx context.Context, path, content string) error
	DeleteFile(ctx context.Context, path string) error
	FileExists(ctx context.Context, path string) (bool, error)
	ListFiles(ctx context.Context) ([]string, error)
	GetAllFilesWithContent(ctx context.Context) (map[string]*RemoteFileInfo, error)
}

// BatchDeleter is an optional optimization for repositories that can remove
// multiple files in one commit. Repository implementations that do not support
// it continue to use DeleteFile one file at a time.
type BatchDeleter interface {
	DeleteFiles(ctx context.Context, paths []string) error
}

// GitHubRepository implements the Repository interface using GitHub API
type GitHubRepository struct {
	client *github.Client
	owner  string
	name   string

	defaultBranchOnce sync.Once
	defaultBranch     string
	defaultBranchErr  error
}

// New creates a new GitHubRepository instance
func New(client *github.Client, owner, name string) Repository {
	return &GitHubRepository{
		client: client,
		owner:  owner,
		name:   name,
	}
}

// EnsureExists checks if the repository exists and creates it if it doesn't
func (r *GitHubRepository) EnsureExists(ctx context.Context) error {
	// Check if repository exists
	repo, _, err := r.client.Repositories.Get(ctx, r.owner, r.name)
	if err == nil {
		// Repository exists
		r.setDefaultBranch(repo.GetDefaultBranch())
		return nil
	}

	// Create repository if it doesn't exist
	_, _, err = r.client.Repositories.Create(ctx, "", &github.Repository{
		Name:             github.String(r.name),
		Description:      github.String("Catapult file synchronization repository"),
		Private:          github.Bool(true),
		AutoInit:         github.Bool(true),
		DefaultBranch:    github.String("main"),
		AllowAutoMerge:   github.Bool(true),
		AllowMergeCommit: github.Bool(true),
		AllowRebaseMerge: github.Bool(true),
		AllowSquashMerge: github.Bool(true),
	})
	if err != nil {
		return fmt.Errorf("failed to create repository: %w", err)
	}

	// Wait for repository to be ready
	for i := 0; i < 10; i++ {
		repo, _, err := r.client.Repositories.Get(ctx, r.owner, r.name)
		if err == nil {
			r.setDefaultBranch(repo.GetDefaultBranch())
			return nil
		}
		time.Sleep(time.Second)
	}

	return fmt.Errorf("repository creation timed out")
}

// GetDefaultBranch returns the default branch of the repository
func (r *GitHubRepository) GetDefaultBranch(ctx context.Context) (string, error) {
	r.defaultBranchOnce.Do(func() {
		repo, _, err := r.client.Repositories.Get(ctx, r.owner, r.name)
		if err != nil {
			r.defaultBranchErr = fmt.Errorf("failed to get repository: %w", err)
			return
		}
		r.defaultBranch = repo.GetDefaultBranch()
	})
	if r.defaultBranchErr != nil {
		return "", r.defaultBranchErr
	}
	if r.defaultBranch == "" {
		return "", fmt.Errorf("repository has no default branch")
	}
	return r.defaultBranch, nil
}

func (r *GitHubRepository) setDefaultBranch(branch string) {
	if branch == "" {
		return
	}
	r.defaultBranchOnce.Do(func() {
		r.defaultBranch = branch
	})
}

func (r *GitHubRepository) branch(ctx context.Context) (string, error) {
	return r.GetDefaultBranch(ctx)
}

func normalizeRemotePath(filePath string) string {
	filePath = strings.ReplaceAll(filePath, "\\", "/")
	if filePath == "" {
		return ""
	}
	return posixpath.Join(filePath)
}

// CreateFile creates a file in the repository
func (r *GitHubRepository) CreateFile(ctx context.Context, path, content string) error {
	path = normalizeRemotePath(path)
	// Check file size before attempting upload
	fileSize := len(content)
	const githubFileSizeLimit = 100 * 1024 * 1024 // 100MB in bytes

	if fileSize > githubFileSizeLimit {
		return &FileSizeError{
			FilePath: path,
			FileSize: fileSize,
			Limit:    githubFileSizeLimit,
		}
	}

	branch, err := r.branch(ctx)
	if err != nil {
		return err
	}

	_, _, err = r.client.Repositories.CreateFile(ctx, r.owner, r.name, path, &github.RepositoryContentFileOptions{
		Message: github.String(fmt.Sprintf("Add %s", path)),
		Content: []byte(content),
		Branch:  github.String(branch),
	})
	if err != nil {
		// Check for GitHub API specific errors
		if ghErr, ok := err.(*github.ErrorResponse); ok {
			switch ghErr.Response.StatusCode {
			case 413: // Payload Too Large
				return &FileSizeError{
					FilePath: path,
					FileSize: fileSize,
					Limit:    githubFileSizeLimit,
				}
			case 422: // Unprocessable Entity (could be file too large or other validation error)
				if fileSize > githubFileSizeLimit {
					return &FileSizeError{
						FilePath: path,
						FileSize: fileSize,
						Limit:    githubFileSizeLimit,
					}
				}
				return &GitHubValidationError{
					FilePath: path,
					Message:  "File validation failed",
					Details:  ghErr.Message,
				}
			case 403: // Forbidden
				return &GitHubPermissionError{
					FilePath: path,
					Message:  "Permission denied",
					Details:  ghErr.Message,
				}
			case 404: // Not Found
				return &GitHubRepositoryError{
					Message: "Repository not found or inaccessible",
					Details: ghErr.Message,
				}
			default:
				return &GitHubAPIError{
					StatusCode: ghErr.Response.StatusCode,
					Message:    ghErr.Message,
					FilePath:   path,
				}
			}
		}
		return fmt.Errorf("failed to create file: %w", err)
	}
	return nil
}

// GetFile gets a file from the repository
func (r *GitHubRepository) GetFile(ctx context.Context, path string) (string, error) {
	path = normalizeRemotePath(path)
	branch, err := r.branch(ctx)
	if err != nil {
		return "", err
	}
	var file *github.RepositoryContent
	err = r.withRateLimitRetry(ctx, func() error {
		var callErr error
		file, _, _, callErr = r.client.Repositories.GetContents(ctx, r.owner, r.name, path, &github.RepositoryContentGetOptions{Ref: branch})
		return callErr
	})
	if err != nil {
		return "", fmt.Errorf("failed to get file: %w", err)
	}
	if file == nil || file.GetType() != "file" {
		return "", &GitHubValidationError{FilePath: path, Message: "GitHub returned a non-file or unsupported Contents API response", Details: "expected a file response"}
	}
	if file.GetSize() > githubFileSizeLimit {
		return "", &FileSizeError{FilePath: path, FileSize: file.GetSize(), Limit: githubFileSizeLimit}
	}
	content, err := file.GetContent()
	if err != nil {
		if file.GetSize() > githubFileSizeLimit {
			return "", &FileSizeError{FilePath: path, FileSize: file.GetSize(), Limit: githubFileSizeLimit}
		}

		// The Contents API may return encoding="none" for large or binary
		// files. Fetch the blob through the Git API in that case; it returns
		// the raw bytes and is also needed when preserving a changed remote
		// version during a local deletion conflict.
		var raw []byte
		rawErr := r.withRateLimitRetry(ctx, func() error {
			var callErr error
			raw, _, callErr = r.client.Git.GetBlobRaw(ctx, r.owner, r.name, file.GetSHA())
			return callErr
		})
		if rawErr == nil {
			return string(raw), nil
		}
		return "", &GitHubValidationError{FilePath: path, Message: "unable to decode file content (binary or unsupported response)", Details: fmt.Sprintf("contents API: %v; git blob API: %v", err, rawErr)}
	}
	return content, nil
}

// UpdateFile updates a file in the repository
func (r *GitHubRepository) UpdateFile(ctx context.Context, path, content string) error {
	path = normalizeRemotePath(path)
	branch, err := r.branch(ctx)
	if err != nil {
		return err
	}
	file, _, _, err := r.client.Repositories.GetContents(ctx, r.owner, r.name, path, &github.RepositoryContentGetOptions{
		Ref: branch,
	})
	if err != nil {
		return fmt.Errorf("failed to get file: %w", err)
	}

	_, _, err = r.client.Repositories.UpdateFile(ctx, r.owner, r.name, path, &github.RepositoryContentFileOptions{
		Message: github.String(fmt.Sprintf("Update %s", path)),
		Content: []byte(content),
		SHA:     github.String(file.GetSHA()),
		Branch:  github.String(branch),
	})
	if err != nil {
		return fmt.Errorf("failed to update file: %w", err)
	}
	return nil
}

// DeleteFile deletes a file from the repository
func (r *GitHubRepository) DeleteFile(ctx context.Context, path string) error {
	path = normalizeRemotePath(path)
	branch, err := r.branch(ctx)
	if err != nil {
		return err
	}
	file, _, _, err := r.client.Repositories.GetContents(ctx, r.owner, r.name, path, &github.RepositoryContentGetOptions{
		Ref: branch,
	})
	if err != nil {
		return fmt.Errorf("failed to get file: %w", err)
	}

	_, _, err = r.client.Repositories.DeleteFile(ctx, r.owner, r.name, path, &github.RepositoryContentFileOptions{
		Message: github.String(fmt.Sprintf("Delete %s", path)),
		SHA:     github.String(file.GetSHA()),
		Branch:  github.String(branch),
	})
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}
	return nil
}

// DeleteFiles removes multiple files in a single Git commit. The Contents API
// creates one commit per file; using the Git data API reduces a bulk delete to
// four Git API requests and one commit, regardless of the number of files.
func (r *GitHubRepository) DeleteFiles(ctx context.Context, paths []string) error {
	if len(paths) == 0 {
		return nil
	}

	branch, err := r.branch(ctx)
	if err != nil {
		return err
	}
	refName := "heads/" + branch

	var ref *github.Reference
	err = r.withRateLimitRetry(ctx, func() error {
		var callErr error
		ref, _, callErr = r.client.Git.GetRef(ctx, r.owner, r.name, refName)
		return callErr
	})
	if err != nil {
		return fmt.Errorf("failed to get branch reference: %w", err)
	}
	if ref == nil || ref.Object == nil || ref.Object.SHA == nil {
		return fmt.Errorf("GitHub returned an invalid branch reference")
	}

	entries := make([]*github.TreeEntry, 0, len(paths))
	for _, path := range paths {
		path = normalizeRemotePath(path)
		entries = append(entries, &github.TreeEntry{Path: github.String(path), Type: github.String("blob")})
	}

	var tree *github.Tree
	err = r.withRateLimitRetry(ctx, func() error {
		var callErr error
		tree, _, callErr = r.client.Git.CreateTree(ctx, r.owner, r.name, ref.Object.GetSHA(), entries)
		return callErr
	})
	if err != nil {
		return fmt.Errorf("failed to create deletion tree: %w", err)
	}
	if tree == nil || tree.SHA == nil {
		return fmt.Errorf("GitHub returned an invalid deletion tree")
	}

	var commit *github.Commit
	err = r.withRateLimitRetry(ctx, func() error {
		var callErr error
		commit, _, callErr = r.client.Git.CreateCommit(ctx, r.owner, r.name, &github.Commit{
			Message: github.String(fmt.Sprintf("Delete %d files", len(paths))),
			Tree:    tree,
			Parents: []*github.Commit{{SHA: ref.Object.SHA}},
		}, nil)
		return callErr
	})
	if err != nil {
		return fmt.Errorf("failed to create deletion commit: %w", err)
	}
	if commit == nil || commit.SHA == nil {
		return fmt.Errorf("GitHub returned an invalid deletion commit")
	}

	err = r.withRateLimitRetry(ctx, func() error {
		_, _, callErr := r.client.Git.UpdateRef(ctx, r.owner, r.name, &github.Reference{
			Ref:    github.String("refs/" + refName),
			Object: &github.GitObject{SHA: commit.SHA},
		}, false)
		return callErr
	})
	if err != nil {
		return fmt.Errorf("failed to update branch reference: %w", err)
	}
	return nil
}

// FileExists checks if a file exists in the repository
func (r *GitHubRepository) FileExists(ctx context.Context, path string) (bool, error) {
	path = normalizeRemotePath(path)
	branch, err := r.branch(ctx)
	if err != nil {
		return false, err
	}
	// Get file from repository
	_, _, _, err = r.client.Repositories.GetContents(ctx, r.owner, r.name, path, &github.RepositoryContentGetOptions{
		Ref: branch,
	})
	if err != nil {
		if _, ok := err.(*github.ErrorResponse); ok {
			return false, nil
		}
		return false, fmt.Errorf("failed to check file: %w", err)
	}

	return true, nil
}

// ListFiles gets all files from the repository using the scalable Git Trees API.
func (r *GitHubRepository) ListFiles(ctx context.Context) ([]string, error) {
	metadata, err := r.listRemoteFiles(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}
	files := make([]string, 0, len(metadata))
	for path := range metadata {
		files = append(files, path)
	}
	return files, nil
}

// GetAllFilesWithContent returns remote metadata. Despite the historical name, it
// intentionally does not download file contents; callers fetch individual files
// only after comparing their SHA.
func (r *GitHubRepository) GetAllFilesWithContent(ctx context.Context) (map[string]*RemoteFileInfo, error) {
	return r.listRemoteFiles(ctx)
}

func (r *GitHubRepository) listRemoteFiles(ctx context.Context) (map[string]*RemoteFileInfo, error) {
	branch, err := r.branch(ctx)
	if err != nil {
		return nil, err
	}
	var tree *github.Tree
	err = r.withRateLimitRetry(ctx, func() error {
		var callErr error
		tree, _, callErr = r.client.Git.GetTree(ctx, r.owner, r.name, branch, true)
		return callErr
	})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve repository tree: %w", err)
	}
	if tree == nil {
		return nil, fmt.Errorf("GitHub returned an empty repository tree")
	}
	if tree.GetTruncated() {
		return nil, fmt.Errorf("repository tree is truncated by GitHub's documented size limit; sync a smaller repository or split it into repositories")
	}
	files := make(map[string]*RemoteFileInfo)
	for _, entry := range tree.Entries {
		if entry.GetType() != "blob" {
			continue
		}
		files[normalizeRemotePath(entry.GetPath())] = &RemoteFileInfo{Path: entry.GetPath(), SHA: entry.GetSHA(), Size: entry.GetSize()}
	}
	return files, nil
}

const maxRateLimitRetries = 3

func (r *GitHubRepository) withRateLimitRetry(ctx context.Context, request func() error) error {
	var last error
	for attempt := 0; attempt <= maxRateLimitRetries; attempt++ {
		last = request()
		if last == nil {
			return nil
		}
		response, ok := last.(*github.ErrorResponse)
		if !ok || !isRateLimited(response) {
			return last
		}
		if attempt == maxRateLimitRetries {
			return &GitHubRateLimitError{StatusCode: response.Response.StatusCode, Message: response.Message, RetryAfter: retryDelay(response.Response, attempt)}
		}
		if err := waitForRetry(ctx, retryDelay(response.Response, attempt)); err != nil {
			return err
		}
	}
	return last
}

func isRateLimited(response *github.ErrorResponse) bool {
	if response == nil || response.Response == nil {
		return false
	}
	if response.Response.StatusCode == http.StatusTooManyRequests {
		return true
	}
	return response.Response.StatusCode == http.StatusForbidden &&
		(response.Response.Header.Get("X-RateLimit-Remaining") == "0" || strings.Contains(strings.ToLower(response.Message), "rate limit"))
}

func retryDelay(response *http.Response, attempt int) time.Duration {
	delay := time.Duration(1<<attempt) * 100 * time.Millisecond
	if response != nil && response.Header.Get("Retry-After") != "" {
		if seconds, err := time.ParseDuration(response.Header.Get("Retry-After") + "s"); err == nil && seconds < 2*time.Second {
			delay = seconds
		}
	}
	if delay > 2*time.Second {
		return 2 * time.Second
	}
	return delay
}

func waitForRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
