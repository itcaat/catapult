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

	// Get remote files
	remoteFiles, err := s.repo.ListFiles(ctx)
	if err != nil {
		return fmt.Errorf("failed to list remote files: %w", err)
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
	for _, remotePath := range remoteFiles {
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
	var synced, updated, pulled, conflicted int

	// Sync each file
	for relPath, file := range allFiles {
		result := s.syncFileByPath(ctx, file, relPath)
		switch result.Status {
		case SyncStatusSynced:
			synced++
		case SyncStatusLocalChanges:
			updated++
		case SyncStatusRemoteChanges:
			pulled++
		case SyncStatusConflict:
			conflicted++
		}
		if result.Error != nil {
			fmt.Fprintf(out, "Error syncing %s: %v\n", result.Path, result.Error)
		}
	}

	// Print summary
	fmt.Fprintf(out, "\nSync Summary:\n")
	fmt.Fprintf(out, "Synced: %d\n", synced)
	fmt.Fprintf(out, "Updated: %d\n", updated)
	fmt.Fprintf(out, "Pulled: %d\n", pulled)
	fmt.Fprintf(out, "Conflicts: %d\n", conflicted)

	return nil
}

// syncFileByPath synchronizes a single file using relative path
func (s *Syncer) syncFileByPath(ctx context.Context, file *storage.FileInfo, relPath string) SyncResult {
	// Check if file exists in remote
	exists, err := s.repo.FileExists(ctx, relPath)
	if err != nil {
		return SyncResult{Path: file.Path, Error: err}
	}

	if !exists {
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

		// Update sync info
		if err := s.fileManager.UpdateSyncInfo(file.Path, ""); err != nil {
			return SyncResult{Path: file.Path, Error: err}
		}

		return SyncResult{Path: file.Path, Status: SyncStatusLocalChanges}
	}

	// Get remote file content
	remoteContent, err := s.repo.GetFile(ctx, relPath)
	if err != nil {
		return SyncResult{Path: file.Path, Error: err}
	}

	// Check if local file exists
	_, err = os.Stat(file.Path)
	if os.IsNotExist(err) {
		// Local file doesn't exist, download remote file
		// Create directory if it doesn't exist
		if err := os.MkdirAll(filepath.Dir(file.Path), 0755); err != nil {
			return SyncResult{Path: file.Path, Error: err}
		}

		if err := os.WriteFile(file.Path, []byte(remoteContent), 0644); err != nil {
			return SyncResult{Path: file.Path, Error: err}
		}

		// Rescan directory to pick up the newly downloaded file
		if err := s.fileManager.ScanDirectory(); err != nil {
			return SyncResult{Path: file.Path, Error: err}
		}

		// Update sync info
		if err := s.fileManager.UpdateSyncInfo(file.Path, ""); err != nil {
			return SyncResult{Path: file.Path, Error: err}
		}

		return SyncResult{Path: file.Path, Status: SyncStatusRemoteChanges}
	}

	// Get local file content
	localContent, err := os.ReadFile(file.Path)
	if err != nil {
		return SyncResult{Path: file.Path, Error: err}
	}

	// Compare content
	if string(localContent) == remoteContent {
		// Content is the same, update sync info
		if err := s.fileManager.UpdateSyncInfo(file.Path, ""); err != nil {
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
		if err := os.WriteFile(file.Path, []byte(remoteContent), 0644); err != nil {
			return SyncResult{Path: file.Path, Error: err}
		}

		// Update sync info
		if err := s.fileManager.UpdateSyncInfo(file.Path, ""); err != nil {
			return SyncResult{Path: file.Path, Error: err}
		}

		return SyncResult{Path: file.Path, Status: SyncStatusRemoteChanges}
	}

	// Both local and remote have changes - this is a conflict
	if err := s.resolveConflict(ctx, file, relPath, localContent, []byte(remoteContent)); err != nil {
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

	// Update sync info
	if err := s.fileManager.UpdateSyncInfo(file.Path, ""); err != nil {
		return err
	}

	return nil
}
