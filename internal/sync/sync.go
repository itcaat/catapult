package sync

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/itcaat/catapult/internal/issues"
	"github.com/itcaat/catapult/internal/repository"
	"github.com/itcaat/catapult/internal/storage"
)

// SyncStatus represents the synchronization status of a file
type SyncStatus int

const (
	SyncStatusSynced SyncStatus = iota
	SyncStatusLocalChanges
	SyncStatusRemoteChanges
	SyncStatusConflict
	SyncStatusDeleted // New status for files that were deleted
)

// SyncResult represents the result of a file synchronization
type SyncResult struct {
	Path   string
	Status SyncStatus
	Error  error
}

// Syncer handles file synchronization between local storage and GitHub
type Syncer struct {
	repo         repository.Repository
	fileManager  *storage.FileManager
	issueManager issues.IssueManager
	logger       *log.Logger
}

// New creates a new Syncer instance
func New(repo repository.Repository, fileManager *storage.FileManager) *Syncer {
	return &Syncer{
		repo:        repo,
		fileManager: fileManager,
	}
}

// NewWithIssueManager creates a new Syncer instance with issue management
func NewWithIssueManager(repo repository.Repository, fileManager *storage.FileManager, issueManager issues.IssueManager, logger *log.Logger) *Syncer {
	return &Syncer{
		repo:         repo,
		fileManager:  fileManager,
		issueManager: issueManager,
		logger:       logger,
	}
}

// SyncAll synchronizes all files in the directory
func (s *Syncer) SyncAll(ctx context.Context, out io.Writer) error {
	// Scan directory for local files
	if err := s.fileManager.ScanDirectory(); err != nil {
		return fmt.Errorf("failed to scan directory: %w", err)
	}

	// Get local files
	localFiles := s.fileManager.GetTrackedFiles()

	// Get all remote files with content efficiently
	remoteFiles, err := s.repo.GetAllFilesWithContent(ctx)
	if err != nil {
		return fmt.Errorf("failed to get remote files with content: %w", err)
	}

	// Create a map of all files (local + remote)
	allFiles := make(map[string]*storage.FileInfo)

	// Add local files
	for _, file := range localFiles {
		relPath, err := filepath.Rel(s.fileManager.BaseDir(), file.Path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}
		allFiles[relPath] = file
	}

	// Add remote files that don't exist locally
	for remotePath := range remoteFiles {
		if _, exists := allFiles[remotePath]; !exists {
			// Create a virtual FileInfo for remote-only file
			localPath := filepath.Join(s.fileManager.BaseDir(), remotePath)
			allFiles[remotePath] = &storage.FileInfo{
				Path: localPath,
				Hash: "", // Will be calculated when downloaded
			}
		}
	}

	fmt.Fprintf(out, "Syncing %d files...\n", len(allFiles))

	// Track results
	var synced, updated, pulled, conflicted, deleted int

	// Sync each file
	for relPath, file := range allFiles {
		result := s.syncFileByPath(ctx, file, relPath, remoteFiles[relPath])

		// Show what's happening with each file
		switch result.Status {
		case SyncStatusSynced:
			synced++
		case SyncStatusLocalChanges:
			fmt.Fprintf(out, "📤 Uploaded: %s\n", relPath)
			updated++
		case SyncStatusRemoteChanges:
			fmt.Fprintf(out, "📥 Downloaded: %s\n", relPath)
			pulled++
		case SyncStatusConflict:
			fmt.Fprintf(out, "⚠️  Conflict resolved (local version kept): %s\n", relPath)
			conflicted++
		case SyncStatusDeleted:
			fmt.Fprintf(out, "🗑️  Deleted from repository: %s\n", relPath)
			deleted++
		}
		if result.Error != nil {
			// Record the sync error in FileInfo for status display
			if err := s.fileManager.RecordSyncError(result.Path, result.Error); err != nil {
				// Log error but continue
				if s.logger != nil {
					s.logger.Printf("Failed to record sync error for %s: %v", result.Path, err)
				}
			}

			// Enhanced error handling with user-friendly messages
			s.handleSyncError(out, result.Path, result.Error)
		} else {
			// Clear any previous sync errors on successful sync
			if err := s.fileManager.ClearSyncError(result.Path); err != nil {
				// Log error but continue
				if s.logger != nil {
					s.logger.Printf("Failed to clear sync error for %s: %v", result.Path, err)
				}
			}
		}
	}

	// Print summary
	fmt.Fprintf(out, "\nSync Summary:\n")
	fmt.Fprintf(out, "Synced: %d\n", synced)
	fmt.Fprintf(out, "Updated: %d\n", updated)
	fmt.Fprintf(out, "Pulled: %d\n", pulled)
	fmt.Fprintf(out, "Conflicts: %d\n", conflicted)
	fmt.Fprintf(out, "Deleted: %d\n", deleted)

	return nil
}

// syncFileByPath synchronizes a single file using relative path
func (s *Syncer) syncFileByPath(ctx context.Context, file *storage.FileInfo, relPath string, remoteFile *repository.RemoteFileInfo) SyncResult {
	// Check if file exists in remote
	remoteExists := remoteFile != nil

	if !remoteExists {
		// File doesn't exist in remote, create it if it exists locally
		if _, err := os.Stat(file.Path); os.IsNotExist(err) {
			// Neither local nor remote exists - this shouldn't happen
			return SyncResult{Path: file.Path, Status: SyncStatusSynced}
		}

		content, err := os.ReadFile(file.Path)
		if err != nil {
			return SyncResult{Path: file.Path, Error: err}
		}

		if err := s.repo.CreateFile(ctx, relPath, string(content)); err != nil {
			return SyncResult{Path: file.Path, Error: err}
		}

		// Update sync info with the new SHA
		localGitSHA := s.fileManager.CalculateGitSHAFromContent(content)
		if err := s.fileManager.UpdateSyncInfo(file.Path, localGitSHA); err != nil {
			return SyncResult{Path: file.Path, Error: err}
		}

		return SyncResult{Path: file.Path, Status: SyncStatusLocalChanges}
	}

	// Check if local file exists or is marked as deleted
	_, statErr := os.Stat(file.Path)
	localFileDeleted := os.IsNotExist(statErr) || file.Deleted

	if localFileDeleted {
		// Local file doesn't exist or is marked as deleted
		// If it was previously synced, delete from remote
		// If it was never synced, download from remote

		if file.LastSyncedRemoteSHA != "" {
			// File was previously synced but now deleted locally - delete from remote
			if err := s.repo.DeleteFile(ctx, relPath); err != nil {
				return SyncResult{Path: file.Path, Error: fmt.Errorf("failed to delete remote file: %w", err)}
			}

			// Remove from file manager tracking since it's deleted
			s.fileManager.RemoveFile(file.Path)

			return SyncResult{Path: file.Path, Status: SyncStatusDeleted}
		} else {
			// File was never synced locally - download from remote
			// Create directory if it doesn't exist
			if err := os.MkdirAll(filepath.Dir(file.Path), 0755); err != nil {
				return SyncResult{Path: file.Path, Error: err}
			}

			if err := os.WriteFile(file.Path, []byte(remoteFile.Content), 0644); err != nil {
				return SyncResult{Path: file.Path, Error: err}
			}

			// Rescan directory to pick up the newly downloaded file
			if err := s.fileManager.ScanDirectory(); err != nil {
				return SyncResult{Path: file.Path, Error: err}
			}

			// Update sync info with the remote SHA
			if err := s.fileManager.UpdateSyncInfo(file.Path, remoteFile.SHA); err != nil {
				return SyncResult{Path: file.Path, Error: err}
			}

			return SyncResult{Path: file.Path, Status: SyncStatusRemoteChanges}
		}
	}

	// Get local file content
	localContent, err := os.ReadFile(file.Path)
	if err != nil {
		return SyncResult{Path: file.Path, Error: err}
	}

	// Compare content
	if string(localContent) == remoteFile.Content {
		// Content is the same, update sync info
		if err := s.fileManager.UpdateSyncInfo(file.Path, remoteFile.SHA); err != nil {
			return SyncResult{Path: file.Path, Error: err}
		}
		return SyncResult{Path: file.Path, Status: SyncStatusSynced}
	}

	// Content is different - check if file was modified locally since last sync
	lastSyncedHash := file.LastSyncedHash

	// Calculate current local file hash
	currentLocalHash, err := s.fileManager.CalculateFileHash(file.Path)
	if err != nil {
		return SyncResult{Path: file.Path, Error: fmt.Errorf("failed to calculate current file hash: %w", err)}
	}

	// If local file hasn't changed since last sync, just pull remote changes
	if lastSyncedHash == currentLocalHash {
		// Local file unchanged, remote file changed - pull remote changes
		if err := os.WriteFile(file.Path, []byte(remoteFile.Content), 0644); err != nil {
			return SyncResult{Path: file.Path, Error: err}
		}

		// Update sync info
		if err := s.fileManager.UpdateSyncInfo(file.Path, remoteFile.SHA); err != nil {
			return SyncResult{Path: file.Path, Error: err}
		}

		return SyncResult{Path: file.Path, Status: SyncStatusRemoteChanges}
	}

	// Both local and remote have changes - this is a conflict
	if err := s.resolveConflict(ctx, file, relPath, localContent, []byte(remoteFile.Content)); err != nil {
		return SyncResult{Path: file.Path, Status: SyncStatusConflict, Error: err}
	}

	return SyncResult{Path: file.Path, Status: SyncStatusConflict}
}

// resolveConflict resolves a file conflict
func (s *Syncer) resolveConflict(ctx context.Context, file *storage.FileInfo, relPath string, localContent, remoteContent []byte) error {
	// For now, just use local content
	if err := s.repo.UpdateFile(ctx, relPath, string(localContent)); err != nil {
		return err
	}

	// Calculate local Git SHA to save as remote SHA (since we uploaded local content)
	localGitSHA := s.fileManager.CalculateGitSHAFromContent(localContent)

	// Update sync info with the new SHA
	if err := s.fileManager.UpdateSyncInfo(file.Path, localGitSHA); err != nil {
		return err
	}

	return nil
}

// handleSyncError provides enhanced error handling with user-friendly messages and creates GitHub issues
func (s *Syncer) handleSyncError(out io.Writer, path string, err error) {
	// Create issue if issue manager is available
	if s.issueManager != nil {
		if s.logger != nil {
			s.logger.Printf("Creating issue for sync error: %s - %v", path, err)
		}
		s.createIssueForError(path, err)
	} else {
		if s.logger != nil {
			s.logger.Printf("Issue manager not available, skipping issue creation for: %s", path)
		}
	}

	// Check for custom repository errors that provide user-friendly messages
	switch e := err.(type) {
	case *repository.FileSizeError:
		fmt.Fprintf(out, "❌ %s\n", e.Error())
		fmt.Fprintf(out, "💡 Solutions:\n")
		fmt.Fprintf(out, "   • Use Git LFS: git lfs track \"*.mov\" (for video files)\n")
		fmt.Fprintf(out, "   • Split file: split -b 50m %s %s_part_\n", filepath.Base(e.FilePath), filepath.Base(e.FilePath))
		fmt.Fprintf(out, "   • Exclude from sync: Add pattern to .gitignore\n")
		fmt.Fprintf(out, "   • Use external storage: Upload to cloud storage instead\n\n")

	case *repository.GitHubPermissionError:
		fmt.Fprintf(out, "❌ %s\n", e.Error())
		fmt.Fprintf(out, "💡 Solutions:\n")
		fmt.Fprintf(out, "   • Check repository permissions in GitHub settings\n")
		fmt.Fprintf(out, "   • Verify your GitHub token has 'repo' scope\n")
		fmt.Fprintf(out, "   • Try re-authenticating: catapult init\n\n")

	case *repository.GitHubValidationError:
		fmt.Fprintf(out, "❌ %s\n", e.Error())
		fmt.Fprintf(out, "💡 Solutions:\n")
		fmt.Fprintf(out, "   • Check file name and content for invalid characters\n")
		fmt.Fprintf(out, "   • Ensure file is not binary or corrupted\n")
		fmt.Fprintf(out, "   • Try excluding this file type from sync\n\n")

	case *repository.GitHubRepositoryError:
		fmt.Fprintf(out, "❌ %s\n", e.Error())
		fmt.Fprintf(out, "💡 Solutions:\n")
		fmt.Fprintf(out, "   • Check repository exists and is accessible\n")
		fmt.Fprintf(out, "   • Verify repository name in config\n")
		fmt.Fprintf(out, "   • Try re-initializing: catapult init\n\n")

	case *repository.GitHubAPIError:
		fmt.Fprintf(out, "❌ %s\n", e.Error())
		fmt.Fprintf(out, "💡 Solutions:\n")
		if e.StatusCode == 403 {
			fmt.Fprintf(out, "   • You may have hit GitHub API rate limits\n")
			fmt.Fprintf(out, "   • Wait a few minutes and try again\n")
		} else if e.StatusCode >= 500 {
			fmt.Fprintf(out, "   • GitHub servers may be experiencing issues\n")
			fmt.Fprintf(out, "   • Check https://status.github.com for service status\n")
		} else {
			fmt.Fprintf(out, "   • Check GitHub service status\n")
			fmt.Fprintf(out, "   • Verify your internet connection\n")
		}
		fmt.Fprintf(out, "   • Try again later or contact support if issue persists\n\n")

	default:
		// Fallback for unknown errors
		fmt.Fprintf(out, "❌ Error syncing %s: %v\n", path, err)

		// Provide general troubleshooting advice
		fmt.Fprintf(out, "💡 General troubleshooting:\n")
		fmt.Fprintf(out, "   • Check your internet connection\n")
		fmt.Fprintf(out, "   • Verify file permissions and accessibility\n")
		fmt.Fprintf(out, "   • Try running: catapult status\n")
		fmt.Fprintf(out, "   • Check logs with: catapult service logs (if using service)\n\n")
	}
}

// createIssueForError creates a GitHub issue for the sync error
func (s *Syncer) createIssueForError(path string, err error) {
	if s.logger != nil {
		s.logger.Printf("Starting issue creation for error: %v", err)
	}

	// Categorize the error
	category := s.categorizeError(err)
	if s.logger != nil {
		s.logger.Printf("Categorized error as: %v", category)
	}

	// Create issue
	issue := &issues.Issue{
		Category:    category,
		Title:       s.generateIssueTitle(path, err),
		Description: s.generateIssueDescription(path, err),
		Files:       []string{filepath.Base(path)},
		Error:       err,
		ErrorMsg:    err.Error(),
		Timestamp:   time.Now(),
		Metadata: map[string]interface{}{
			"file_path":  path,
			"error_type": fmt.Sprintf("%T", err),
		},
	}

	if s.logger != nil {
		s.logger.Printf("Created issue object: %s", issue.Title)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if s.logger != nil {
		s.logger.Printf("Calling issue manager CreateIssue...")
	}

	githubIssue, createErr := s.issueManager.CreateIssue(ctx, issue)
	if createErr != nil {
		if s.logger != nil {
			s.logger.Printf("Failed to create issue for sync error: %v", createErr)
		}
		return
	}

	if s.logger != nil && githubIssue != nil {
		s.logger.Printf("Successfully created issue #%d for sync error: %s", githubIssue.Number, githubIssue.Title)
	} else if s.logger != nil {
		s.logger.Printf("Issue creation returned nil issue")
	}
}

// categorizeError determines the issue category based on the error type
func (s *Syncer) categorizeError(err error) issues.IssueCategory {
	switch err.(type) {
	case *repository.FileSizeError:
		return issues.CategoryQuota
	case *repository.GitHubPermissionError:
		return issues.CategoryPermission
	case *repository.GitHubValidationError:
		return issues.CategoryCorruption
	case *repository.GitHubRepositoryError:
		return issues.CategoryAuth
	case *repository.GitHubAPIError:
		return issues.CategoryNetwork
	default:
		// Check error message for common patterns
		errMsg := strings.ToLower(err.Error())
		if strings.Contains(errMsg, "permission") || strings.Contains(errMsg, "access") {
			return issues.CategoryPermission
		}
		if strings.Contains(errMsg, "network") || strings.Contains(errMsg, "connection") || strings.Contains(errMsg, "timeout") {
			return issues.CategoryNetwork
		}
		if strings.Contains(errMsg, "conflict") {
			return issues.CategoryConflict
		}
		if strings.Contains(errMsg, "auth") || strings.Contains(errMsg, "token") {
			return issues.CategoryAuth
		}
		return issues.CategoryUnknown
	}
}

// generateIssueTitle creates a descriptive title for the issue
func (s *Syncer) generateIssueTitle(path string, err error) string {
	fileName := filepath.Base(path)

	switch err.(type) {
	case *repository.FileSizeError:
		return fmt.Sprintf("File too large: %s", fileName)
	case *repository.GitHubPermissionError:
		return fmt.Sprintf("Permission denied: %s", fileName)
	case *repository.GitHubValidationError:
		return fmt.Sprintf("File validation failed: %s", fileName)
	case *repository.GitHubRepositoryError:
		return fmt.Sprintf("Repository access error: %s", fileName)
	case *repository.GitHubAPIError:
		return fmt.Sprintf("GitHub API error: %s", fileName)
	default:
		return fmt.Sprintf("Sync error: %s", fileName)
	}
}

// generateIssueDescription creates a detailed description for the issue
func (s *Syncer) generateIssueDescription(path string, err error) string {
	fileName := filepath.Base(path)

	switch e := err.(type) {
	case *repository.FileSizeError:
		return fmt.Sprintf("The file '%s' is too large to sync (%d bytes, limit: %d bytes). This file exceeds GitHub's file size limits and cannot be uploaded directly.", fileName, e.FileSize, e.Limit)
	case *repository.GitHubPermissionError:
		return fmt.Sprintf("Permission denied when trying to sync '%s'. This may be due to insufficient repository permissions or an invalid GitHub token.", fileName)
	case *repository.GitHubValidationError:
		return fmt.Sprintf("GitHub rejected the file '%s' due to validation errors. The file may contain invalid characters, be corrupted, or violate GitHub's content policies.", fileName)
	case *repository.GitHubRepositoryError:
		return fmt.Sprintf("Unable to access the repository when syncing '%s'. The repository may not exist, be private, or you may lack access permissions.", fileName)
	case *repository.GitHubAPIError:
		return fmt.Sprintf("GitHub API error occurred while syncing '%s' (HTTP %d). This may be due to rate limiting, server issues, or network problems.", fileName, e.StatusCode)
	default:
		return fmt.Sprintf("An unexpected error occurred while syncing '%s'. The sync operation failed and may require manual intervention.", fileName)
	}
}
