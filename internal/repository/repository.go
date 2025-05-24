package repository

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/google/go-github/v57/github"
)

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
	_, _, err := r.client.Repositories.CreateFile(ctx, r.owner, r.name, path, &github.RepositoryContentFileOptions{
		Message: github.String(fmt.Sprintf("Add %s", path)),
		Content: []byte(content),
		Branch:  github.String("main"),
	})
	if err != nil {
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
