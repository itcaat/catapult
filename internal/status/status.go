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
	fmt.Fprintln(out, strings.Repeat("-", 80))

	for relPath, file := range allFiles {
		// Determine status
		status := determineFileStatus(file, remoteFiles[relPath])
		emoji := getStatusEmoji(status)

		// Print status with emoji
		fmt.Fprintf(out, "%-30s %-35s %s\n", relPath, status, emoji)
	}

	return nil
}

// determineFileStatus determines the sync status of a file
func determineFileStatus(file *storage.FileInfo, remoteFile *repository.RemoteFileInfo) string {
	// Check for sync errors FIRST (highest priority)
	if file.LastSyncErrorMsg != "" {
		return formatSyncError(file.LastSyncErrorMsg)
	}

	// Check if file was deleted locally SECOND (before checking remote existence)
	if file.Deleted {
		if remoteFile != nil {
			return "Deleted locally (needs remote deletion)"
		} else {
			return "Deleted locally"
		}
	}

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

// getStatusEmoji returns the appropriate emoji for a given status
func getStatusEmoji(status string) string {
	switch {
	case strings.HasPrefix(status, "Sync Error"):
		return "🚨" // Red - Sync Error (highest priority)
	case status == "Synced":
		return "✅" // Green - Success
	case status == "Modified locally" || status == "Modified in repository" || status == "Not synced" || status == "Remote-only" || status == "Deleted locally (needs remote deletion)":
		return "⚠️" // Yellow - Needs sync
	case status == "Conflict":
		return "❌" // Red - Failed/Conflict
	case status == "Deleted locally":
		return "🗑️" // Gray - Deleted
	case status == "Local-only":
		return "📁" // Blue - Local only
	default:
		return "❓" // Unknown status
	}
}

// formatSyncError formats a sync error message for display
func formatSyncError(errorMsg string) string {
	// Categorize common error types for better user experience
	// Order matters - more specific patterns should come first
	switch {
	case strings.Contains(strings.ToLower(errorMsg), "network") || strings.Contains(strings.ToLower(errorMsg), "timeout") || strings.Contains(strings.ToLower(errorMsg), "connection"):
		return "Sync Error (Network)"
	case strings.Contains(strings.ToLower(errorMsg), "permission") || strings.Contains(strings.ToLower(errorMsg), "forbidden") || strings.Contains(strings.ToLower(errorMsg), "unauthorized"):
		return "Sync Error (Permission)"
	case strings.Contains(strings.ToLower(errorMsg), "authentication") || strings.Contains(strings.ToLower(errorMsg), "token"):
		return "Sync Error (Auth)"
	case strings.Contains(strings.ToLower(errorMsg), "validation") || strings.Contains(strings.ToLower(errorMsg), "invalid") || strings.Contains(strings.ToLower(errorMsg), "corrupted"):
		return "Sync Error (Validation)"
	case strings.Contains(strings.ToLower(errorMsg), "rate limit") || strings.Contains(strings.ToLower(errorMsg), "quota"):
		return "Sync Error (Rate Limit)"
	case strings.Contains(strings.ToLower(errorMsg), "not found") || strings.Contains(strings.ToLower(errorMsg), "404"):
		return "Sync Error (Not Found)"
	default:
		return "Sync Error (Unknown)"
	}
}
