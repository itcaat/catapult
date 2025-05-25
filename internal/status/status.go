package status

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/itcaat/catapult/internal/repository"
	"github.com/itcaat/catapult/internal/storage"
)

// PrintStatus prints the status of all tracked files
func PrintStatus(fileManager *storage.FileManager, repo repository.Repository, baseDir string, out io.Writer) error {
	// Get all tracked files
	files := fileManager.GetTrackedFiles()

	if len(files) == 0 {
		fmt.Fprintln(out, "No files are currently tracked.")
		return nil
	}

	// Get remote files
	ctx := context.Background()
	remoteFiles, err := repo.GetAllFilesWithContent(ctx)
	if err != nil {
		return fmt.Errorf("failed to get remote files: %w", err)
	}

	fmt.Fprintln(out, "Tracked Files Status:")
	fmt.Fprintln(out, strings.Repeat("-", 50))

	for _, file := range files {
		// Calculate relative path
		relPath, err := filepath.Rel(baseDir, file.Path)
		if err != nil {
			relPath = file.Path
		}

		// Determine status
		status := determineFileStatus(file, remoteFiles[relPath])

		// Print status
		fmt.Fprintf(out, "%-30s %s\n", relPath, status)
	}

	return nil
}

// determineFileStatus determines the sync status of a file
func determineFileStatus(file *storage.FileInfo, remoteFile *repository.RemoteFileInfo) string {
	// Check if file exists remotely
	if remoteFile == nil {
		return "Local-only"
	}

	// Check if file was deleted locally
	if file.Deleted {
		return "Deleted locally"
	}

	// Compare hashes to determine status
	if file.LastSyncedRemoteSHA == "" {
		return "Not synced"
	}

	if file.LastSyncedRemoteSHA == remoteFile.SHA {
		if file.Hash == file.LastSyncedHash {
			return "Synced"
		} else {
			return "Modified locally"
		}
	} else {
		if file.Hash == file.LastSyncedHash {
			return "Modified in repository"
		} else {
			return "Conflict"
		}
	}
}
