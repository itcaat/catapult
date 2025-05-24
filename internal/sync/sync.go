package sync

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

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
	repo        repository.Repository
	fileManager *storage.FileManager
}

// New creates a new Syncer instance
func New(repo repository.Repository, fileManager *storage.FileManager) *Syncer {
	return &Syncer{
		repo:        repo,
		fileManager: fileManager,
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
			// Enhanced error handling with user-friendly messages
			s.handleSyncError(out, result.Path, result.Error)
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

// handleSyncError provides enhanced error handling with user-friendly messages
func (s *Syncer) handleSyncError(out io.Writer, path string, err error) {
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
