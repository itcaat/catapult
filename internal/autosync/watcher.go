package autosync

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// WatchConfig holds configuration for file watching
type WatchConfig struct {
	DebounceDelay  time.Duration
	IgnorePatterns []string
}

// DefaultWatchConfig returns sensible defaults for file watching
func DefaultWatchConfig() *WatchConfig {
	return &WatchConfig{
		DebounceDelay: 2 * time.Second,
		IgnorePatterns: []string{
			".git/",
			".catapult/",
			"*.tmp",
			"*.swp",
			"*.swo",
			".DS_Store",
			"Thumbs.db",
			"*.log",
			"node_modules/",
			".vscode/",
			".idea/",
		},
	}
}

// Watcher monitors file system changes
type Watcher struct {
	fsWatcher *fsnotify.Watcher
	debouncer *Debouncer
	config    *WatchConfig
	logger    *log.Logger
}

// NewWatcher creates a new file watcher
func NewWatcher(config *WatchConfig, logger *log.Logger) (*Watcher, error) {
	if config == nil {
		config = DefaultWatchConfig()
	}

	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create fsnotify watcher: %w", err)
	}

	debouncer := NewDebouncer(config.DebounceDelay)

	return &Watcher{
		fsWatcher: fsWatcher,
		debouncer: debouncer,
		config:    config,
		logger:    logger,
	}, nil
}

// Watch starts watching the specified directory for changes
func (w *Watcher) Watch(ctx context.Context, directory string, callback func(FileEvent)) error {
	// Add directory to watcher
	if err := w.fsWatcher.Add(directory); err != nil {
		return fmt.Errorf("failed to add directory to watcher: %w", err)
	}

	w.logger.Printf("Started watching directory: %s", directory)

	for {
		select {
		case event := <-w.fsWatcher.Events:
			if w.shouldIgnore(event.Name) {
				continue
			}

			w.logger.Printf("File event: %s %s", event.Op, event.Name)

			// Use debouncer to group rapid changes
			w.debouncer.Add(event.Name, func() {
				fileEvent := FileEvent{
					Path:      event.Name,
					Op:        event.Op,
					Timestamp: time.Now(),
				}
				callback(fileEvent)
			})

		case err := <-w.fsWatcher.Errors:
			w.logger.Printf("Watcher error: %v", err)
			// Continue watching on errors

		case <-ctx.Done():
			w.logger.Printf("Stopping file watcher")
			return w.Close()
		}
	}
}

// shouldIgnore checks if a file path should be ignored based on patterns
func (w *Watcher) shouldIgnore(path string) bool {
	// Get relative path components
	relPath := filepath.Base(path)
	fullPath := filepath.Clean(path)

	for _, pattern := range w.config.IgnorePatterns {
		// Check exact match
		if pattern == relPath {
			return true
		}

		// Check if path contains pattern (for directories)
		if strings.Contains(fullPath, strings.TrimSuffix(pattern, "/")) {
			return true
		}

		// Check wildcard patterns
		if strings.HasPrefix(pattern, "*") {
			suffix := strings.TrimPrefix(pattern, "*")
			if strings.HasSuffix(relPath, suffix) {
				return true
			}
		}

		// Check directory patterns
		if strings.HasSuffix(pattern, "/") {
			dirPattern := strings.TrimSuffix(pattern, "/")
			if strings.Contains(fullPath, dirPattern) {
				return true
			}
		}
	}

	return false
}

// Close stops the watcher and releases resources
func (w *Watcher) Close() error {
	w.debouncer.Stop()
	return w.fsWatcher.Close()
}

// AddPath adds a new path to watch
func (w *Watcher) AddPath(path string) error {
	return w.fsWatcher.Add(path)
}

// RemovePath removes a path from watching
func (w *Watcher) RemovePath(path string) error {
	return w.fsWatcher.Remove(path)
}
