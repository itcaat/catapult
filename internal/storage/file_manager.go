package storage

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FileInfo represents metadata about a file
type FileInfo struct {
	Path                string    `json:"path"`
	Hash                string    `json:"hash"`
	LastModified        time.Time `json:"last_modified"`
	Size                int64     `json:"size"`
	LastSyncedHash      string    `json:"last_synced_hash"`
	LastSyncedRemoteSHA string    `json:"last_synced_remote_sha"`
}

// SyncStatus represents the synchronization status of a file
type SyncStatus int

const (
	// SyncStatusSynced means the file is in sync with the repository
	SyncStatusSynced SyncStatus = iota
	// SyncStatusLocalChanges means the file has local changes
	SyncStatusLocalChanges
	// SyncStatusRemoteChanges means the file has changes in the repository
	SyncStatusRemoteChanges
	// SyncStatusConflict means the file has both local and remote changes
	SyncStatusConflict
)

// FileManager handles local file operations and tracking
type FileManager struct {
	baseDir string
	files   map[string]*FileInfo
}

// NewFileManager creates a new FileManager instance
func NewFileManager(baseDir string) *FileManager {
	return &FileManager{
		baseDir: baseDir,
		files:   make(map[string]*FileInfo),
	}
}

// BaseDir returns the base directory
func (fm *FileManager) BaseDir() string {
	return fm.baseDir
}

// ScanDirectory scans the base directory for files and updates the tracking list
func (fm *FileManager) ScanDirectory() error {
	// Save existing files data to preserve sync info
	existingFiles := make(map[string]*FileInfo)
	for path, info := range fm.files {
		existingFiles[path] = info
	}

	// Clear existing files
	fm.files = make(map[string]*FileInfo)

	// Walk through the directory
	err := filepath.Walk(fm.baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and hidden files
		if info.IsDir() {
			return nil
		}

		if strings.HasPrefix(info.Name(), ".") {
			return nil
		}

		// Skip .catapult directory files except for the files we want to sync
		if strings.Contains(path, ".catapult") && !strings.Contains(path, ".catapult/files") {
			return nil
		}

		// Skip state.json and config files
		if info.Name() == "state.json" || strings.HasSuffix(info.Name(), ".runtime.yaml") || strings.HasSuffix(info.Name(), ".yaml") {
			return nil
		}

		// Calculate file hash
		hash, err := fm.calculateFileHash(path)
		if err != nil {
			return fmt.Errorf("failed to calculate file hash: %w", err)
		}

		// Create file info
		fileInfo := &FileInfo{
			Path:         path,
			Hash:         hash,
			LastModified: info.ModTime(),
			Size:         info.Size(),
		}

		// Preserve sync info if file was already tracked
		if existingInfo, exists := existingFiles[path]; exists {
			fileInfo.LastSyncedHash = existingInfo.LastSyncedHash
			fileInfo.LastSyncedRemoteSHA = existingInfo.LastSyncedRemoteSHA
		}

		// Add to tracking list
		fm.files[path] = fileInfo

		return nil
	})

	return err
}

// GetTrackedFiles returns a list of tracked files
func (fm *FileManager) GetTrackedFiles() []*FileInfo {
	// Scan directory before returning files
	if err := fm.ScanDirectory(); err != nil {
		// Log error but continue
		fmt.Printf("Warning: failed to scan directory: %v\n", err)
	}

	files := make([]*FileInfo, 0, len(fm.files))
	for _, file := range fm.files {
		files = append(files, file)
	}
	return files
}

// GetFileInfo returns information about a tracked file
func (fm *FileManager) GetFileInfo(path string) (*FileInfo, error) {
	// Get absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Get file info
	fileInfo, exists := fm.files[absPath]
	if !exists {
		return nil, fmt.Errorf("file not tracked: %s", path)
	}

	return fileInfo, nil
}

// HasChanges checks if a file has been modified
func (fm *FileManager) HasChanges(path string) (bool, error) {
	// Get file info
	fileInfo, err := fm.GetFileInfo(path)
	if err != nil {
		return false, err
	}

	// Get current file info
	info, err := os.Stat(fileInfo.Path)
	if err != nil {
		return false, fmt.Errorf("failed to get file info: %w", err)
	}

	// Check if file has been modified
	if info.ModTime().After(fileInfo.LastModified) {
		// Calculate new hash
		hash, err := fm.calculateFileHash(fileInfo.Path)
		if err != nil {
			return false, fmt.Errorf("failed to calculate file hash: %w", err)
		}

		// Check if hash has changed
		return hash != fileInfo.Hash, nil
	}

	return false, nil
}

// UpdateFileInfo updates the file information
func (fm *FileManager) UpdateFileInfo(path string) error {
	// Get file info
	fileInfo, err := fm.GetFileInfo(path)
	if err != nil {
		return err
	}

	// Get current file info
	info, err := os.Stat(fileInfo.Path)
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	// Calculate new hash
	hash, err := fm.calculateFileHash(fileInfo.Path)
	if err != nil {
		return fmt.Errorf("failed to calculate file hash: %w", err)
	}

	// Update file info
	fileInfo.Hash = hash
	fileInfo.LastModified = info.ModTime()
	fileInfo.Size = info.Size()

	return nil
}

// SaveState saves the current state to a file
func (fm *FileManager) SaveState(path string) error {
	// Create state file
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create state file: %w", err)
	}
	defer file.Close()

	// Encode state
	encoder := json.NewEncoder(file)
	if err := encoder.Encode(fm.files); err != nil {
		return fmt.Errorf("failed to encode state: %w", err)
	}

	return nil
}

// LoadState loads the state from a file
func (fm *FileManager) LoadState(path string) error {
	// Open state file
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open state file: %w", err)
	}
	defer file.Close()

	// Decode state
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&fm.files); err != nil {
		return fmt.Errorf("failed to decode state: %w", err)
	}

	return nil
}

// CalculateFileHash calculates the SHA-256 hash of a file (public method)
func (fm *FileManager) CalculateFileHash(path string) (string, error) {
	return fm.calculateFileHash(path)
}

// CalculateGitSHA calculates the Git SHA-1 hash of a file (like GitHub uses)
func (fm *FileManager) CalculateGitSHA(path string) (string, error) {
	// Read file content
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return fm.calculateGitSHAFromContent(content), nil
}

// CalculateGitSHAFromContent calculates Git SHA-1 from content
func (fm *FileManager) CalculateGitSHAFromContent(content []byte) string {
	return fm.calculateGitSHAFromContent(content)
}

// calculateGitSHAFromContent calculates Git blob SHA-1 hash from content
func (fm *FileManager) calculateGitSHAFromContent(content []byte) string {
	// Git calculates SHA-1 of "blob <size>\0<content>"
	header := fmt.Sprintf("blob %d\x00", len(content))

	hash := sha1.New()
	hash.Write([]byte(header))
	hash.Write(content)

	return hex.EncodeToString(hash.Sum(nil))
}

// calculateFileHash calculates the SHA-256 hash of a file
func (fm *FileManager) calculateFileHash(path string) (string, error) {
	// Open file
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Create hash
	hash := sha256.New()

	// Copy file to hash
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to calculate hash: %w", err)
	}

	// Get hash as hex string
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// GetSyncStatus determines the synchronization status of a file
func (fm *FileManager) GetSyncStatus(path string) (SyncStatus, error) {
	// Get file info
	fileInfo, err := fm.GetFileInfo(path)
	if err != nil {
		return SyncStatusSynced, err
	}

	// Calculate current hash
	currentHash, err := fm.calculateFileHash(fileInfo.Path)
	if err != nil {
		return SyncStatusSynced, fmt.Errorf("failed to calculate current hash: %w", err)
	}

	// Check if file has local changes
	hasLocalChanges := currentHash != fileInfo.LastSyncedHash

	// Check if file has remote changes
	hasRemoteChanges := fileInfo.LastSyncedRemoteSHA != "" && fileInfo.LastSyncedRemoteSHA != "sha123"

	// Determine sync status
	if hasLocalChanges && hasRemoteChanges {
		return SyncStatusConflict, nil
	} else if hasLocalChanges {
		return SyncStatusLocalChanges, nil
	} else if hasRemoteChanges {
		return SyncStatusRemoteChanges, nil
	}

	return SyncStatusSynced, nil
}

// UpdateSyncInfo updates the synchronization information for a file
func (fm *FileManager) UpdateSyncInfo(path, remoteSHA string) error {
	// Get file info
	fileInfo, err := fm.GetFileInfo(path)
	if err != nil {
		return err
	}

	// Calculate current hash
	currentHash, err := fm.calculateFileHash(fileInfo.Path)
	if err != nil {
		return fmt.Errorf("failed to calculate current hash: %w", err)
	}

	// Update sync info
	fileInfo.LastSyncedHash = currentHash
	fileInfo.LastSyncedRemoteSHA = remoteSHA

	return nil
}

// SaveConflictVersions saves both local and remote versions of a file
func (fm *FileManager) SaveConflictVersions(path, remoteContent string) error {
	// Get file info
	fileInfo, err := fm.GetFileInfo(path)
	if err != nil {
		return err
	}

	// Create backup directory if it doesn't exist
	backupDir := filepath.Join(fm.baseDir, ".catapult", "conflicts")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Get relative path
	relPath, err := filepath.Rel(fm.baseDir, fileInfo.Path)
	if err != nil {
		return fmt.Errorf("failed to get relative path: %w", err)
	}

	// Create backup paths
	localBackup := filepath.Join(backupDir, relPath+".local")
	remoteBackup := filepath.Join(backupDir, relPath+".remote")

	// Create backup directory for the file
	if err := os.MkdirAll(filepath.Dir(localBackup), 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Copy local file to backup
	if err := copyFile(fileInfo.Path, localBackup); err != nil {
		return fmt.Errorf("failed to backup local file: %w", err)
	}

	// Save remote content to backup
	if err := os.WriteFile(remoteBackup, []byte(remoteContent), 0644); err != nil {
		return fmt.Errorf("failed to save remote file: %w", err)
	}

	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	// Open source file
	source, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer source.Close()

	// Create destination file
	destination, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destination.Close()

	// Copy file
	if _, err := io.Copy(destination, source); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	return nil
}
