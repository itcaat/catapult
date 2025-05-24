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

// Sync handles file synchronization between local storage and GitHub repository
type Sync struct {
	repo        repository.Repository
	fileManager *storage.FileManager
}

// New creates a new Sync instance
func New(repo repository.Repository, fileManager *storage.FileManager) *Sync {
	return &Sync{
		repo:        repo,
		fileManager: fileManager,
	}
}

// SyncFile synchronizes a single file
func (s *Sync) SyncFile(ctx context.Context, path string) error {
	// Get file info
	fileInfo, err := s.fileManager.GetFileInfo(path)
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	// Get relative path
	relPath, err := filepath.Rel(s.fileManager.BaseDir(), fileInfo.Path)
	if err != nil {
		return fmt.Errorf("failed to get relative path: %w", err)
	}

	// Get sync status
	status, err := s.fileManager.GetSyncStatus(path)
	if err != nil {
		return fmt.Errorf("failed to get sync status: %w", err)
	}

	// Handle different sync statuses
	switch status {
	case storage.SyncStatusSynced:
		return nil
	case storage.SyncStatusLocalChanges:
		// Read file content
		content, err := os.ReadFile(fileInfo.Path)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}

		// Update file in repository
		if err := s.repo.UpdateFile(ctx, relPath, string(content)); err != nil {
			return fmt.Errorf("failed to update file in repository: %w", err)
		}

		// Update sync info
		if err := s.fileManager.UpdateSyncInfo(path, ""); err != nil {
			return fmt.Errorf("failed to update sync info: %w", err)
		}

	case storage.SyncStatusRemoteChanges:
		// Get file from repository
		content, err := s.repo.GetFile(ctx, relPath)
		if err != nil {
			return fmt.Errorf("failed to get file from repository: %w", err)
		}

		// Write file
		if err := os.WriteFile(fileInfo.Path, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}

		// Update sync info
		if err := s.fileManager.UpdateSyncInfo(path, ""); err != nil {
			return fmt.Errorf("failed to update sync info: %w", err)
		}

	case storage.SyncStatusConflict:
		// Get file from repository
		content, err := s.repo.GetFile(ctx, relPath)
		if err != nil {
			return fmt.Errorf("failed to get file from repository: %w", err)
		}

		// Save conflict versions
		if err := s.fileManager.SaveConflictVersions(path, content); err != nil {
			return fmt.Errorf("failed to save conflict versions: %w", err)
		}

		return fmt.Errorf("conflict detected for file %s: both local and remote changes exist", relPath)
	}

	return nil
}

// SyncAll synchronizes all tracked files and prints progress to w
func (s *Sync) SyncAll(ctx context.Context, w io.Writer) error {
	files := s.fileManager.GetTrackedFiles()
	var synced, updated, pulled, conflicted, failed int

	fmt.Fprintf(w, "\nSyncing %d tracked files...\n\n", len(files))
	for _, file := range files {
		relPath, _ := filepath.Rel(s.fileManager.BaseDir(), file.Path)
		err := s.SyncFile(ctx, file.Path)
		status, _ := s.fileManager.GetSyncStatus(file.Path)
		switch {
		case err == nil && status == storage.SyncStatusSynced:
			fmt.Fprintf(w, "[✓] %s: Synced\n", relPath)
			synced++
		case err == nil && status == storage.SyncStatusLocalChanges:
			fmt.Fprintf(w, "[→] %s: Updated in repository\n", relPath)
			updated++
		case err == nil && status == storage.SyncStatusRemoteChanges:
			fmt.Fprintf(w, "[←] %s: Pulled from repository\n", relPath)
			pulled++
		case err != nil && status == storage.SyncStatusConflict:
			fmt.Fprintf(w, "[!] %s: Conflict detected!\n", relPath)
			fmt.Fprintf(w, "    Local version:   .catapult/conflicts/%s.local\n", relPath)
			fmt.Fprintf(w, "    Remote version:  .catapult/conflicts/%s.remote\n", relPath)
			conflicted++
		case err != nil:
			fmt.Fprintf(w, "[✗] %s: Error: %v\n", relPath, err)
			failed++
		}
	}
	fmt.Fprintf(w, "\nSummary: Synced: %d, Updated: %d, Pulled: %d, Conflicts: %d, Failed: %d\n", synced, updated, pulled, conflicted, failed)
	return nil
}

// PullFile pulls a file from the repository
func (s *Sync) PullFile(ctx context.Context, path string) error {
	// Get file info
	fileInfo, err := s.fileManager.GetFileInfo(path)
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	// Get relative path
	relPath, err := filepath.Rel(s.fileManager.BaseDir(), fileInfo.Path)
	if err != nil {
		return fmt.Errorf("failed to get relative path: %w", err)
	}

	// Get sync status
	status, err := s.fileManager.GetSyncStatus(path)
	if err != nil {
		return fmt.Errorf("failed to get sync status: %w", err)
	}

	// Handle different sync statuses
	switch status {
	case storage.SyncStatusSynced:
		return nil
	case storage.SyncStatusLocalChanges:
		// Get file from repository
		content, err := s.repo.GetFile(ctx, relPath)
		if err != nil {
			return fmt.Errorf("failed to get file from repository: %w", err)
		}

		// Save conflict versions
		if err := s.fileManager.SaveConflictVersions(path, content); err != nil {
			return fmt.Errorf("failed to save conflict versions: %w", err)
		}

		return fmt.Errorf("conflict detected for file %s: local changes exist", relPath)

	case storage.SyncStatusRemoteChanges, storage.SyncStatusConflict:
		// Get file from repository
		content, err := s.repo.GetFile(ctx, relPath)
		if err != nil {
			return fmt.Errorf("failed to get file from repository: %w", err)
		}

		// Write file
		if err := os.WriteFile(fileInfo.Path, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}

		// Update sync info
		if err := s.fileManager.UpdateSyncInfo(path, ""); err != nil {
			return fmt.Errorf("failed to update sync info: %w", err)
		}
	}

	return nil
}

// PullAll pulls all tracked files from the repository and prints progress to w
func (s *Sync) PullAll(ctx context.Context, w io.Writer) error {
	files := s.fileManager.GetTrackedFiles()
	var pulled, conflicted, failed int

	fmt.Fprintf(w, "\nPulling %d tracked files...\n\n", len(files))
	for _, file := range files {
		relPath, _ := filepath.Rel(s.fileManager.BaseDir(), file.Path)
		err := s.PullFile(ctx, file.Path)
		status, _ := s.fileManager.GetSyncStatus(file.Path)
		switch {
		case err == nil && status == storage.SyncStatusSynced:
			fmt.Fprintf(w, "[✓] %s: Up to date\n", relPath)
			pulled++
		case err != nil && status == storage.SyncStatusConflict:
			fmt.Fprintf(w, "[!] %s: Conflict detected!\n", relPath)
			fmt.Fprintf(w, "    Local version:   .catapult/conflicts/%s.local\n", relPath)
			fmt.Fprintf(w, "    Remote version:  .catapult/conflicts/%s.remote\n", relPath)
			conflicted++
		case err != nil:
			fmt.Fprintf(w, "[✗] %s: Error: %v\n", relPath, err)
			failed++
		}
	}
	fmt.Fprintf(w, "\nSummary: Pulled: %d, Conflicts: %d, Failed: %d\n", pulled, conflicted, failed)
	return nil
}
