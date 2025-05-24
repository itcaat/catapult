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
	// Scan directory for files
	if err := s.fileManager.ScanDirectory(); err != nil {
		return fmt.Errorf("failed to scan directory: %w", err)
	}

	// Get all files
	files := s.fileManager.GetTrackedFiles()
	fmt.Fprintf(out, "Syncing %d files...\n", len(files))

	// Track results
	var synced, updated, pulled, conflicted int

	// Sync each file
	for _, file := range files {
		result := s.syncFile(ctx, file)
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

// syncFile synchronizes a single file
func (s *Syncer) syncFile(ctx context.Context, file *storage.FileInfo) SyncResult {
	// Get relative path
	relPath, err := filepath.Rel(s.fileManager.BaseDir(), file.Path)
	if err != nil {
		return SyncResult{Path: file.Path, Error: err}
	}

	// Check if file exists in remote
	exists, err := s.repo.FileExists(ctx, relPath)
	if err != nil {
		return SyncResult{Path: file.Path, Error: err}
	}

	if !exists {
		// File doesn't exist in remote, create it
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
		if err := os.WriteFile(file.Path, []byte(remoteContent), 0644); err != nil {
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
