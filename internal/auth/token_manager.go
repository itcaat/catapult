package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Storage defines the interface for token storage
type Storage interface {
	Store(token *Token) error
	Get() (*Token, error)
}

// FileStorage implements Storage interface using a file
type FileStorage struct {
	path string
}

// NewFileStorage creates a new FileStorage instance
func NewFileStorage(path string) *FileStorage {
	return &FileStorage{
		path: path,
	}
}

// Store saves the token to a file
func (fs *FileStorage) Store(token *Token) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(fs.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Marshal token to JSON
	data, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("failed to marshal token: %w", err)
	}

	// Write to file
	if err := os.WriteFile(fs.path, data, 0600); err != nil {
		return fmt.Errorf("failed to write token: %w", err)
	}

	return nil
}

// Get retrieves the token from a file
func (fs *FileStorage) Get() (*Token, error) {
	// Check if file exists
	if _, err := os.Stat(fs.path); os.IsNotExist(err) {
		return nil, nil
	}

	// Read file
	data, err := os.ReadFile(fs.path)
	if err != nil {
		return nil, fmt.Errorf("failed to read token: %w", err)
	}

	// Unmarshal token
	var token Token
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("failed to unmarshal token: %w", err)
	}

	return &token, nil
}
