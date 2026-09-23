package persistence

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

var renameFile = os.Rename

// WriteFile writes data to path without exposing a partially written file.
// The temporary file is created next to the target so Rename is atomic.
func WriteFile(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}

	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return fmt.Errorf("set temporary file permissions: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temporary file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync temporary file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary file: %w", err)
	}
	if err := renameFile(tmpPath, path); err != nil {
		return fmt.Errorf("replace file: %w", err)
	}

	// Persist the directory entry where supported. This is harmless on filesystems
	// that do not allow directory fsync and the rename is still atomic there.
	if dirFile, err := os.Open(dir); err == nil {
		defer dirFile.Close()
		// Some platforms/filesystems do not support syncing directory handles.
		_ = dirFile.Sync()
	}
	return nil
}

// BackupCorrupt moves an invalid persistence file out of the way without
// overwriting an earlier recovery artifact.
func BackupCorrupt(path string) (string, error) {
	backup := fmt.Sprintf("%s.corrupt-%d", path, time.Now().UnixNano())
	if err := os.Rename(path, backup); err != nil {
		return "", err
	}
	return backup, nil
}
