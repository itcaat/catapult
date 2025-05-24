package status

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/itcaat/catapult/internal/repository"
	"github.com/itcaat/catapult/internal/storage"
)

// PrintStatus prints the status of tracked files to the provided writer
func PrintStatus(fileManager *storage.FileManager, repo repository.Repository, baseDir string, w io.Writer) error {
	// Get local files
	localFiles := fileManager.GetTrackedFiles()

	// Get all remote files with content in one efficient call
	remoteFiles, err := repo.GetAllFilesWithContent(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get remote files with content: %w", err)
	}

	// Create map of all files (local + remote)
	allFiles := make(map[string]*storage.FileInfo)

	// Add local files
	for _, file := range localFiles {
		relPath, err := filepath.Rel(baseDir, file.Path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}
		allFiles[relPath] = file
	}

	// Add remote-only files
	for remotePath := range remoteFiles {
		if _, exists := allFiles[remotePath]; !exists {
			// Create virtual FileInfo for remote-only file
			localPath := filepath.Join(baseDir, remotePath)
			allFiles[remotePath] = &storage.FileInfo{
				Path: localPath,
				Hash: "", // Remote-only file
			}
		}
	}

	if len(allFiles) == 0 {
		fmt.Fprintln(w, "No files are being tracked")
		return nil
	}

	fmt.Fprintf(w, "\nTracked Files Status:\n")
	fmt.Fprintf(w, "====================\n\n")

	for relPath, file := range allFiles {
		fmt.Fprintf(w, "File: %s\n", relPath)

		// Check if file exists locally
		localExists := true
		if _, err := os.Stat(file.Path); os.IsNotExist(err) || file.Deleted {
			localExists = false
		}

		// Check if file exists remotely
		remoteFile, remoteExists := remoteFiles[relPath]

		// Determine real status
		if !localExists && !remoteExists {
			fmt.Fprintf(w, "  Status: File not found (this shouldn't happen)\n\n")
			continue
		}

		if !localExists && remoteExists {
			fmt.Fprintf(w, "  Size: %d bytes (remote)\n", remoteFile.Size)

			// Check if this file was previously synced (has sync info)
			// If it was synced before, it means it was deleted locally and should be deleted from remote
			// If it was never synced, it should be downloaded
			if file.LastSyncedRemoteSHA != "" {
				fmt.Fprintf(w, "  Status: Deleted locally (will be deleted from repository on sync)\n\n")
			} else {
				fmt.Fprintf(w, "  Status: Remote-only (needs to be downloaded)\n\n")
			}
			continue
		}

		if localExists && !remoteExists {
			fmt.Fprintf(w, "  Size: %d bytes\n", file.Size)
			fmt.Fprintf(w, "  Last Modified: %s\n", file.LastModified.Format("2006-01-02 15:04:05"))

			// Check for large files and warn user
			const githubFileSizeLimit = 100 * 1024 * 1024 // 100MB
			if file.Size > githubFileSizeLimit {
				sizeMB := float64(file.Size) / (1024 * 1024)
				fmt.Fprintf(w, "  ⚠️  Status: Local-only - TOO LARGE (%.1f MB > 100 MB) - WILL NOT SYNC\n", sizeMB)
			} else {
				fmt.Fprintf(w, "  Status: Local-only (needs to be uploaded)\n")
			}
			fmt.Fprintf(w, "\n")
			continue
		}

		// Both exist - compare content
		localContent, err := os.ReadFile(file.Path)
		if err != nil {
			fmt.Fprintf(w, "  Status: Error reading local file: %v\n\n", err)
			continue
		}

		fmt.Fprintf(w, "  Size: %d bytes\n", file.Size)
		fmt.Fprintf(w, "  Last Modified: %s\n", file.LastModified.Format("2006-01-02 15:04:05"))

		// Calculate local Git SHA-1 (like GitHub uses)
		localGitSHA := fileManager.CalculateGitSHAFromContent(localContent)

		// Compare SHA instead of content for efficiency
		if localGitSHA == remoteFile.SHA {
			fmt.Fprintf(w, "  Status: Synced\n")
		} else {
			// Check if file was modified locally since last sync
			lastSyncedHash := file.LastSyncedHash
			lastSyncedRemoteSHA := file.LastSyncedRemoteSHA
			currentLocalHash, err := fileManager.CalculateFileHash(file.Path)
			if err != nil {
				fmt.Fprintf(w, "  Status: Error calculating file hash: %v\n", err)
			} else if lastSyncedHash == currentLocalHash {
				// Local unchanged since last sync, remote changed
				fmt.Fprintf(w, "  Status: Modified in repository (needs to be pulled)\n")
			} else if lastSyncedRemoteSHA == remoteFile.SHA {
				// Remote unchanged since last sync, local changed
				fmt.Fprintf(w, "  Status: Modified locally (needs to be synced)\n")
			} else if lastSyncedHash == "" || lastSyncedRemoteSHA == "" {
				// No sync info - determine based on which side changed
				fmt.Fprintf(w, "  Status: Modified locally (needs to be synced)\n")
			} else {
				// Both changed since last sync - conflict
				fmt.Fprintf(w, "  Status: Conflict: both local and remote changes exist\n")
			}
		}
		fmt.Fprintln(w)
	}
	return nil
}
