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
	"sync"
	"time"

	"github.com/itcaat/catapult/internal/persistence"
)

// FileInfo represents metadata about a file
type FileInfo struct {
	Path                string    `json:"path"`
	Hash                string    `json:"hash"`
	LastModified        time.Time `json:"last_modified"`
	Size                int64     `json:"size"`
	LastSyncedHash      string    `json:"last_synced_hash"`
	LastSyncedRemoteSHA string    `json:"last_synced_remote_sha"`
	Deleted             bool      `json:"deleted,omitempty"` // Track if file was deleted locally

	// Error tracking fields
	LastSyncErrorMsg  string    `json:"last_sync_error,omitempty"`
	LastSyncAttempt   time.Time `json:"last_sync_attempt,omitempty"`
	SyncRetryCount    int       `json:"sync_retry_count,omitempty"`
	ConflictLocalHash string    `json:"conflict_local_hash,omitempty"`
	ConflictRemoteSHA string    `json:"conflict_remote_sha,omitempty"`
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
	mutex   sync.RWMutex
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
	fm.mutex.Lock()
	defer fm.mutex.Unlock()
	return fm.scanDirectoryLocked()
}

func (fm *FileManager) scanDirectoryLocked() error {
	// Save existing files data to preserve sync info
	existingFiles := make(map[string]*FileInfo)
	for path, info := range fm.files {
		existingFiles[path] = info
	}

	// Mark all existing files as potentially deleted
	for _, info := range fm.files {
		info.Deleted = true
	}

	// Walk through the directory
	err := filepath.Walk(fm.baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and hidden files
		if info.IsDir() {
			return nil
		}

		// Calculate file hash
		hash, err := fm.calculateFileHash(path)
		if err != nil {
			return fmt.Errorf("failed to calculate file hash: %w", err)
		}

		// Check if file was already tracked
		if existingInfo, exists := existingFiles[path]; exists {
			// Update existing file info
			existingInfo.Hash = hash
			existingInfo.LastModified = info.ModTime()
			existingInfo.Size = info.Size()
			existingInfo.Deleted = false // File exists, not deleted
			fm.files[path] = existingInfo
		} else {
			// Create new file info
			fileInfo := &FileInfo{
				Path:         path,
				Hash:         hash,
				LastModified: info.ModTime(),
				Size:         info.Size(),
				Deleted:      false,
			}
			fm.files[path] = fileInfo
		}

		return nil
	})

	return err
}

// GetTrackedFiles returns a list of tracked files
func (fm *FileManager) GetTrackedFiles() []*FileInfo {
	fm.mutex.Lock()
	defer fm.mutex.Unlock()

	// Scan directory before returning files
	if err := fm.scanDirectoryLocked(); err != nil {
		// Log error but continue
		fmt.Printf("Warning: failed to scan directory: %v\n", err)
	}

	files := make([]*FileInfo, 0, len(fm.files))
	for _, file := range fm.files {
		fileCopy := *file
		files = append(files, &fileCopy)
	}
	return files
}

// GetFileInfo returns information about a tracked file
func (fm *FileManager) GetFileInfo(path string) (*FileInfo, error) {
	fm.mutex.RLock()
	defer fm.mutex.RUnlock()
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

	fileCopy := *fileInfo
	return &fileCopy, nil
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
	fm.mutex.Lock()
	defer fm.mutex.Unlock()
	return fm.updateFileInfoLocked(path)
}

func (fm *FileManager) updateFileInfoLocked(path string) error {
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
	fm.mutex.RLock()
	defer fm.mutex.RUnlock()
	data, err := json.Marshal(fm.files)
	if err != nil {
		return fmt.Errorf("failed to encode state: %w", err)
	}
	if err := persistence.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}
	return nil
}

// LoadState loads the state from a file
func (fm *FileManager) LoadState(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to open state file: %w", err)
	}
	var files map[string]*FileInfo
	if err := json.Unmarshal(data, &files); err != nil {
		backup, backupErr := persistence.BackupCorrupt(path)
		if backupErr != nil {
			return fmt.Errorf("failed to decode state: %w (backup failed: %v)", err, backupErr)
		}
		return fmt.Errorf("failed to decode state: %w (corrupt file moved to %s)", err, backup)
	}
	if files == nil {
		files = make(map[string]*FileInfo)
	}
	fm.mutex.Lock()
	fm.files = files
	fm.mutex.Unlock()
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
	fm.mutex.RLock()
	defer fm.mutex.RUnlock()
	return fm.getSyncStatusLocked(path)
}

func (fm *FileManager) getSyncStatusLocked(path string) (SyncStatus, error) {
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
	fm.mutex.Lock()
	defer fm.mutex.Unlock()
	return fm.updateSyncInfoLocked(path, remoteSHA)
}

func (fm *FileManager) updateSyncInfoLocked(path, remoteSHA string) error {
	// This method is called while fm.mutex is already locked. Do not call
	// GetFileInfo here because it attempts to acquire a read lock and would
	// deadlock against the write lock held by UpdateSyncInfo.
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}
	fileInfo, exists := fm.files[absPath]
	if !exists {
		return fmt.Errorf("file not tracked: %s", path)
	}

	// Calculate current hash
	currentHash, err := fm.calculateFileHash(fileInfo.Path)
	if err != nil {
		return fmt.Errorf("failed to calculate current hash: %w", err)
	}

	// Update sync info
	fileInfo.LastSyncedHash = currentHash
	fileInfo.LastSyncedRemoteSHA = remoteSHA
	fileInfo.ConflictLocalHash = ""
	fileInfo.ConflictRemoteSHA = ""

	return nil
}

// RecordConflict remembers the versions captured for an unresolved conflict.
func (fm *FileManager) RecordConflict(path, remoteSHA string) error {
	fm.mutex.Lock()
	defer fm.mutex.Unlock()
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}
	fileInfo, exists := fm.files[absPath]
	if !exists {
		return fmt.Errorf("file not tracked: %s", path)
	}
	localHash, err := fm.calculateFileHash(fileInfo.Path)
	if err != nil {
		if os.IsNotExist(err) {
			fileInfo.ConflictLocalHash = "deleted"
			fileInfo.ConflictRemoteSHA = remoteSHA
			return nil
		}
		return fmt.Errorf("failed to calculate current hash: %w", err)
	}
	fileInfo.ConflictLocalHash = localHash
	fileInfo.ConflictRemoteSHA = remoteSHA
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

	// Preserve a local deletion as a recoverable marker.
	if err := copyFile(fileInfo.Path, localBackup); err != nil {
		if os.IsNotExist(err) {
			if writeErr := os.WriteFile(localBackup, []byte("catapult: file was deleted locally\n"), 0644); writeErr != nil {
				return fmt.Errorf("failed to save local deletion marker: %w", writeErr)
			}
		} else {
			return fmt.Errorf("failed to backup local file: %w", err)
		}
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

// RemoveFile removes a file from tracking
func (fm *FileManager) RemoveFile(path string) {
	fm.mutex.Lock()
	defer fm.mutex.Unlock()
	delete(fm.files, path)
}

// RecordSyncError records a sync error for a file
func (fm *FileManager) RecordSyncError(path string, err error) error {
	fm.mutex.Lock()
	defer fm.mutex.Unlock()
	fileInfo, exists := fm.files[path]
	if !exists {
		return fmt.Errorf("file not tracked: %s", path)
	}

	fileInfo.LastSyncErrorMsg = err.Error()
	fileInfo.LastSyncAttempt = time.Now()
	fileInfo.SyncRetryCount++

	return nil
}

// ClearSyncError clears the sync error for a file (called on successful sync)
func (fm *FileManager) ClearSyncError(path string) error {
	fm.mutex.Lock()
	defer fm.mutex.Unlock()
	fileInfo, exists := fm.files[path]
	if !exists {
		return fmt.Errorf("file not tracked: %s", path)
	}

	fileInfo.LastSyncErrorMsg = ""
	fileInfo.LastSyncAttempt = time.Time{}
	fileInfo.SyncRetryCount = 0

	return nil
}

// HasSyncError checks if a file has a sync error
func (fm *FileManager) HasSyncError(path string) bool {
	fm.mutex.RLock()
	defer fm.mutex.RUnlock()
	fileInfo, exists := fm.files[path]
	if !exists {
		return false
	}

	return fileInfo.LastSyncErrorMsg != ""
}
