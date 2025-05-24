package repository

import (
	"context"
	"fmt"
	"path/filepath"
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

func (e *GitHubAPIError) Error() string {
	return fmt.Sprintf("GitHub API error (HTTP %d) for '%s': %s", e.StatusCode, e.FilePath, e.Message)
}

// RemoteFileInfo contains information about a remote file
type RemoteFileInfo struct {
	Path    string
	Content string
	SHA     string
	Size    int
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

// GitHubRepository implements the Repository interface using GitHub API
type GitHubRepository struct {
	client *github.Client
	owner  string
	name   string
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
	_, _, err := r.client.Repositories.Get(ctx, r.owner, r.name)
	if err == nil {
		// Repository exists
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
		_, _, err := r.client.Repositories.Get(ctx, r.owner, r.name)
		if err == nil {
			return nil
		}
		time.Sleep(time.Second)
	}

	return fmt.Errorf("repository creation timed out")
}

// GetDefaultBranch returns the default branch of the repository
func (r *GitHubRepository) GetDefaultBranch(ctx context.Context) (string, error) {
	repo, _, err := r.client.Repositories.Get(ctx, r.owner, r.name)
	if err != nil {
		return "", fmt.Errorf("failed to get repository: %w", err)
	}
	return repo.GetDefaultBranch(), nil
}

// CreateFile creates a file in the repository
func (r *GitHubRepository) CreateFile(ctx context.Context, path, content string) error {
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

	_, _, err := r.client.Repositories.CreateFile(ctx, r.owner, r.name, path, &github.RepositoryContentFileOptions{
		Message: github.String(fmt.Sprintf("Add %s", path)),
		Content: []byte(content),
		Branch:  github.String("main"),
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
	file, _, _, err := r.client.Repositories.GetContents(ctx, r.owner, r.name, path, &github.RepositoryContentGetOptions{
		Ref: "main",
	})
	if err != nil {
		return "", fmt.Errorf("failed to get file: %w", err)
	}
	content, err := file.GetContent()
	if err != nil {
		return "", fmt.Errorf("failed to get file content: %w", err)
	}
	return content, nil
}

// UpdateFile updates a file in the repository
func (r *GitHubRepository) UpdateFile(ctx context.Context, path, content string) error {
	file, _, _, err := r.client.Repositories.GetContents(ctx, r.owner, r.name, path, &github.RepositoryContentGetOptions{
		Ref: "main",
	})
	if err != nil {
		return fmt.Errorf("failed to get file: %w", err)
	}

	_, _, err = r.client.Repositories.UpdateFile(ctx, r.owner, r.name, path, &github.RepositoryContentFileOptions{
		Message: github.String(fmt.Sprintf("Update %s", path)),
		Content: []byte(content),
		SHA:     github.String(file.GetSHA()),
		Branch:  github.String("main"),
	})
	if err != nil {
		return fmt.Errorf("failed to update file: %w", err)
	}
	return nil
}

// DeleteFile deletes a file from the repository
func (r *GitHubRepository) DeleteFile(ctx context.Context, path string) error {
	file, _, _, err := r.client.Repositories.GetContents(ctx, r.owner, r.name, path, &github.RepositoryContentGetOptions{
		Ref: "main",
	})
	if err != nil {
		return fmt.Errorf("failed to get file: %w", err)
	}

	_, _, err = r.client.Repositories.DeleteFile(ctx, r.owner, r.name, path, &github.RepositoryContentFileOptions{
		Message: github.String(fmt.Sprintf("Delete %s", path)),
		SHA:     github.String(file.GetSHA()),
		Branch:  github.String("main"),
	})
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}
	return nil
}

// FileExists checks if a file exists in the repository
func (r *GitHubRepository) FileExists(ctx context.Context, path string) (bool, error) {
	// Get file from repository
	_, _, _, err := r.client.Repositories.GetContents(ctx, r.owner, r.name, path, &github.RepositoryContentGetOptions{
		Ref: "main",
	})
	if err != nil {
		if _, ok := err.(*github.ErrorResponse); ok {
			return false, nil
		}
		return false, fmt.Errorf("failed to check file: %w", err)
	}

	return true, nil
}

// ListFiles gets all files from the repository
func (r *GitHubRepository) ListFiles(ctx context.Context) ([]string, error) {
	files := []string{}

	// Get repository contents recursively
	err := r.listFilesRecursive(ctx, "", &files)
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	return files, nil
}

// listFilesRecursive recursively lists files in a directory
func (r *GitHubRepository) listFilesRecursive(ctx context.Context, path string, files *[]string) error {
	_, directoryContent, _, err := r.client.Repositories.GetContents(ctx, r.owner, r.name, path, &github.RepositoryContentGetOptions{
		Ref: "main",
	})
	if err != nil {
		return err
	}

	for _, content := range directoryContent {
		if content.GetType() == "file" {
			if path == "" {
				*files = append(*files, content.GetName())
			} else {
				*files = append(*files, filepath.Join(path, content.GetName()))
			}
		} else if content.GetType() == "dir" {
			// Recursively get files from subdirectory
			subPath := content.GetName()
			if path != "" {
				subPath = filepath.Join(path, content.GetName())
			}
			err := r.listFilesRecursive(ctx, subPath, files)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// GetAllFilesWithContent gets all files with their content efficiently
func (r *GitHubRepository) GetAllFilesWithContent(ctx context.Context) (map[string]*RemoteFileInfo, error) {
	files := make(map[string]*RemoteFileInfo)

	// Get repository contents recursively with content
	err := r.getAllFilesRecursive(ctx, "", files)
	if err != nil {
		return nil, fmt.Errorf("failed to get all files with content: %w", err)
	}

	return files, nil
}

// getAllFilesRecursive recursively lists files in a directory and retrieves their content
func (r *GitHubRepository) getAllFilesRecursive(ctx context.Context, path string, files map[string]*RemoteFileInfo) error {
	_, directoryContent, _, err := r.client.Repositories.GetContents(ctx, r.owner, r.name, path, &github.RepositoryContentGetOptions{
		Ref: "main",
	})
	if err != nil {
		return err
	}

	for _, content := range directoryContent {
		if content.GetType() == "file" {
			filePath := content.GetName()
			if path != "" {
				filePath = filepath.Join(path, content.GetName())
			}

			// Get content if available, otherwise make a separate call
			fileContent := ""
			if content.Content != nil {
				// Content is available in the API response (for small files)
				decodedContent, err := content.GetContent()
				if err == nil {
					fileContent = decodedContent
				}
			}

			// If content wasn't available in the listing, we'll get it separately
			if fileContent == "" {
				decodedContent, err := r.GetFile(ctx, filePath)
				if err == nil {
					fileContent = decodedContent
				}
			}

			files[filePath] = &RemoteFileInfo{
				Path:    filePath,
				Content: fileContent,
				SHA:     content.GetSHA(),
				Size:    content.GetSize(),
			}
		} else if content.GetType() == "dir" {
			// Recursively get files from subdirectory
			subPath := content.GetName()
			if path != "" {
				subPath = filepath.Join(path, content.GetName())
			}
			err := r.getAllFilesRecursive(ctx, subPath, files)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
