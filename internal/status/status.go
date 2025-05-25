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

// PrintStatus prints the status of all tracked files (local and remote)
func PrintStatus(fileManager *storage.FileManager, repo repository.Repository, baseDir string, out io.Writer) error {
	// Get all tracked files
	localFiles := fileManager.GetTrackedFiles()

	// Get remote files
	ctx := context.Background()
	remoteFiles, err := repo.GetAllFilesWithContent(ctx)
	if err != nil {
		return fmt.Errorf("failed to get remote files: %w", err)
	}

	// Create a unified map of all files (local + remote)
	allFiles := make(map[string]*storage.FileInfo)

	// Add local files to the map
	for _, file := range localFiles {
		relPath, err := filepath.Rel(baseDir, file.Path)
		if err != nil {
			relPath = file.Path
		}
		allFiles[relPath] = file
	}

	// Add remote-only files to the map
	for remotePath := range remoteFiles {
		if _, exists := allFiles[remotePath]; !exists {
			// Create a virtual FileInfo for remote-only file
			localPath := filepath.Join(baseDir, remotePath)
			allFiles[remotePath] = &storage.FileInfo{
				Path: localPath,
				Hash: "", // No local hash since file doesn't exist locally
			}
		}
	}

	if len(allFiles) == 0 {
		fmt.Fprintln(out, "No files are currently tracked or available remotely.")
		return nil
	}

	fmt.Fprintln(out, "Files Status (Local + Remote):")
	fmt.Fprintln(out, strings.Repeat("-", 60))

	for relPath, file := range allFiles {
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

	// Check if file exists locally (by checking if we have a local hash)
	localExists := file.Hash != ""

	// If file doesn't exist locally but exists remotely
	if !localExists {
		return "Remote-only"
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
