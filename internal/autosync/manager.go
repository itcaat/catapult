package autosync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/itcaat/catapult/internal/config"
	"github.com/itcaat/catapult/internal/logging"
	"github.com/itcaat/catapult/internal/network"
	"github.com/itcaat/catapult/internal/repository"
	"github.com/itcaat/catapult/internal/storage"
	catapultsync "github.com/itcaat/catapult/internal/sync"
)

// Config holds configuration for auto-sync
type Config struct {
	Enabled             bool          `yaml:"enabled"`
	WatchLocalChanges   bool          `yaml:"watch_local_changes"`
	CheckRemoteInterval time.Duration `yaml:"check_remote_interval"`
	DebounceDelay       time.Duration `yaml:"debounce_delay"`
	RetryAttempts       int           `yaml:"retry_attempts"`
	OfflineQueue        bool          `yaml:"offline_queue"`
	MaxQueueSize        int           `yaml:"max_queue_size"`
	NotificationLevel   string        `yaml:"notification_level"` // silent, minimal, verbose
}

// DefaultConfig returns sensible defaults for auto-sync
func DefaultConfig() *Config {
	return &Config{
		Enabled:             true,
		WatchLocalChanges:   true,
		CheckRemoteInterval: 5 * time.Minute,
		DebounceDelay:       2 * time.Second,
		RetryAttempts:       3,
		OfflineQueue:        true,
		MaxQueueSize:        100,
		NotificationLevel:   "minimal",
	}
}

// Manager coordinates automatic synchronization
type Manager struct {
	watcher         *Watcher
	config          *Config
	appConfig       *config.Config
	syncer          *catapultsync.Syncer
	fileManager     *storage.FileManager
	repo            repository.Repository
	logger          *logging.Logger
	done            chan struct{}
	networkDetector *network.Detector
	queue           *Queue
	jobs            chan syncJob
	queuedJobs      map[string]struct{}
	runningJobs     map[string]struct{}
	pendingJobs     map[string]struct{}
	jobsMutex       sync.Mutex
	workerDone      chan struct{}
	workerStarted   chan struct{}
	changeDebouncer *Debouncer
	shutdownOnce    sync.Once
}

type syncJob struct {
	path         string
	processQueue bool
	checkRemote  bool
}

// NewManager creates a new auto-sync manager
func NewManager(
	appConfig *config.Config,
	syncer *catapultsync.Syncer,
	fileManager *storage.FileManager,
	repo repository.Repository,
	logger *logging.Logger,
) (*Manager, error) {
	autoSyncConfig := DefaultConfig()

	// Create watcher
	watchConfig := &WatchConfig{
		DebounceDelay: autoSyncConfig.DebounceDelay,
	}
	watcher, err := NewWatcher(watchConfig, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher: %w", err)
	}

	// Create network detector
	networkDetector := network.NewDetector()

	// Create offline queue
	queuePath := filepath.Join(filepath.Dir(appConfig.Storage.StatePath), "queue.json")
	queue := NewQueue(queuePath, autoSyncConfig.MaxQueueSize)

	// Load existing queue
	if err := queue.Load(); err != nil {
		logger.Warnf("Warning: failed to load offline queue: %v", err)
	}

	return &Manager{
		watcher:         watcher,
		config:          autoSyncConfig,
		appConfig:       appConfig,
		syncer:          syncer,
		fileManager:     fileManager,
		repo:            repo,
		logger:          logger,
		done:            make(chan struct{}),
		networkDetector: networkDetector,
		queue:           queue,
		jobs:            make(chan syncJob, 64),
		queuedJobs:      make(map[string]struct{}),
		runningJobs:     make(map[string]struct{}),
		pendingJobs:     make(map[string]struct{}),
		workerDone:      make(chan struct{}),
		workerStarted:   make(chan struct{}),
		changeDebouncer: NewDebouncer(autoSyncConfig.DebounceDelay),
	}, nil
}

// Start begins automatic synchronization
func (m *Manager) Start(ctx context.Context) error {
	if !m.config.Enabled {
		m.logger.Infof("Auto-sync is disabled")
		return nil
	}

	m.logger.Infof("Starting auto-sync manager")

	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go m.syncWorker(workerCtx)
	close(m.workerStarted)

	// Process any pending queue items first
	if m.config.OfflineQueue {
		go m.processOfflineQueue(workerCtx)
	}

	// Start file watcher if enabled
	if m.config.WatchLocalChanges {
		go func() {
			if err := m.startFileWatcher(workerCtx); err != nil {
				m.logger.Infof("File watcher error: %v", err)
			}
		}()
	}

	// Start periodic remote checks if enabled
	if m.config.CheckRemoteInterval > 0 {
		go func() {
			m.startPeriodicRemoteCheck(workerCtx)
		}()
	}

	// Start periodic queue cleanup
	go m.startQueueCleanup(workerCtx)

	m.logger.Infof("Auto-sync manager started successfully")

	// Wait for cancellation or an explicit Stop, then drain in-flight work.
	select {
	case <-ctx.Done():
	case <-m.done:
	}
	cancel()
	m.shutdown()
	<-m.workerDone
	return ctx.Err()
}

func (m *Manager) syncWorker(ctx context.Context) {
	defer close(m.workerDone)
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.done:
			return
		case job := <-m.jobs:
			key := jobKey(job)
			m.jobsMutex.Lock()
			m.runningJobs[key] = struct{}{}
			m.jobsMutex.Unlock()

			// A job may be marked pending while it is running. Repeat it once
			// after completion so an event arriving during a sync is not lost.
			for {
				if job.processQueue {
					m.processPendingOperations()
				} else if job.checkRemote {
					m.checkRemoteChanges()
				} else {
					m.syncFile(job.path)
				}

				m.jobsMutex.Lock()
				if _, pending := m.pendingJobs[key]; pending {
					delete(m.pendingJobs, key)
					m.jobsMutex.Unlock()
					continue
				}
				delete(m.runningJobs, key)
				delete(m.queuedJobs, key)
				m.jobsMutex.Unlock()
				break
			}
		}
	}
}

func jobKey(job syncJob) string {
	if job.processQueue {
		return "queue"
	}
	if job.checkRemote {
		return "remote"
	}
	return "sync:" + job.path
}

func (m *Manager) enqueueSync(path string) {
	m.enqueue(syncJob{path: path})
}

func (m *Manager) enqueueQueueProcessing() {
	m.enqueue(syncJob{processQueue: true})
}

func (m *Manager) enqueueRemoteCheck() {
	m.enqueue(syncJob{checkRemote: true})
}

func (m *Manager) enqueue(job syncJob) {
	key := jobKey(job)
	m.jobsMutex.Lock()
	if _, exists := m.queuedJobs[key]; exists {
		if _, running := m.runningJobs[key]; running {
			m.pendingJobs[key] = struct{}{}
		}
		m.jobsMutex.Unlock()
		return
	}
	m.queuedJobs[key] = struct{}{}
	m.jobsMutex.Unlock()

	select {
	case m.jobs <- job:
	case <-m.done:
		m.jobsMutex.Lock()
		delete(m.queuedJobs, key)
		m.jobsMutex.Unlock()
	}
}

// startFileWatcher monitors local file changes
func (m *Manager) startFileWatcher(ctx context.Context) error {
	return m.watcher.Watch(ctx, m.appConfig.Storage.BaseDir, m.onFileChange)
}

// onFileChange handles file change events
func (m *Manager) onFileChange(event FileEvent) {
	m.logger.Debugf("Processing file change: %s", event.Path)

	// Check if file should be synced
	_, err := filepath.Rel(m.appConfig.Storage.BaseDir, event.Path)
	if err != nil {
		m.logger.Errorf("Failed to get relative path for %s: %v", event.Path, err)
		return
	}

	// Batch all changes during the debounce window into one full sync. SyncAll
	// already scans the directory, so syncing individual paths would repeat the
	// same expensive operation for every file in a bulk change.
	m.changeDebouncer.Add("all-files", func() {
		m.enqueueSync("")
	})
}

// syncFile synchronizes a specific file
func (m *Manager) syncFile(relPath string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	m.logger.Infof("Syncing file: %s", relPath)

	// Wait for network connectivity with timeout
	connectCtx, connectCancel := context.WithTimeout(ctx, 10*time.Second)
	defer connectCancel()

	if err := m.networkDetector.WaitForGitHubConnectivity(connectCtx); err != nil {
		m.logger.Infof("No GitHub connectivity, queueing sync for %s", relPath)
		m.queueOperation(relPath, "sync")
		return
	}

	// Reload file manager state to get latest file info
	if err := m.fileManager.LoadState(m.appConfig.Storage.StatePath); err != nil {
		m.logger.Errorf("Failed to load state: %v", err)
		m.queueOperation(relPath, "sync")
		return
	}

	// Scan directory to update file info
	if err := m.fileManager.ScanDirectory(); err != nil {
		m.logger.Errorf("Failed to scan directory: %v", err)
		m.queueOperation(relPath, "sync")
		return
	}

	// Perform sync
	if err := m.syncer.SyncAll(ctx, os.Stdout); err != nil {
		m.logger.Errorf("Failed to sync file %s: %v", relPath, err)
		m.queueOperation(relPath, "sync")
		return
	}

	// Save state after sync
	if err := m.fileManager.SaveState(m.appConfig.Storage.StatePath); err != nil {
		m.logger.Errorf("Failed to save state: %v", err)
	}

	if m.config.NotificationLevel != "silent" {
		fmt.Printf("✅ Auto-synced: %s\n", relPath)
	}
}

// queueOperation adds an operation to the offline queue
func (m *Manager) queueOperation(filePath, operation string) {
	if !m.config.OfflineQueue {
		return
	}

	op := &QueueOperation{
		FilePath:  filePath,
		Operation: operation,
		Timestamp: time.Now(),
	}

	if err := m.queue.Add(op); err != nil {
		m.logger.Errorf("Failed to queue operation for %s: %v", filePath, err)
	} else {
		if m.config.NotificationLevel == "verbose" {
			fmt.Printf("📥 Queued for sync: %s\n", filePath)
		}
	}
}

// processOfflineQueue processes pending operations when network is available
func (m *Manager) processOfflineQueue(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if m.networkDetector.IsConnected() {
				m.enqueueQueueProcessing()
			}
		}
	}
}

// processPendingOperations executes all pending queue operations
func (m *Manager) processPendingOperations() {
	pending := m.queue.GetPending()
	if len(pending) == 0 {
		return
	}

	m.logger.Infof("Processing %d pending operations", len(pending))

	for _, op := range pending {
		// Skip operations that have exceeded retry limit
		if op.Retries >= m.config.RetryAttempts {
			m.logger.Infof("Operation %s exceeded retry limit, removing", op.ID)
			m.queue.Remove(op.ID)
			continue
		}

		// Try to execute operation
		if err := m.executeQueuedOperation(op); err != nil {
			m.logger.Errorf("Failed to execute operation %s: %v", op.ID, err)
			m.queue.UpdateRetry(op.ID, err)
		} else {
			// Success - remove from queue
			m.queue.Remove(op.ID)
			if m.config.NotificationLevel != "silent" {
				fmt.Printf("✅ Processed queued sync: %s\n", op.FilePath)
			}
		}
	}
}

// executeQueuedOperation executes a single queued operation
func (m *Manager) executeQueuedOperation(op *QueueOperation) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Ensure we have connectivity
	if err := m.networkDetector.WaitForGitHubConnectivity(ctx); err != nil {
		return fmt.Errorf("no GitHub connectivity: %w", err)
	}

	// Execute the operation based on type
	switch op.Operation {
	case "sync":
		return m.executeSyncOperation(ctx, op.FilePath)
	default:
		return fmt.Errorf("unknown operation type: %s", op.Operation)
	}
}

// executeSyncOperation executes a sync operation
func (m *Manager) executeSyncOperation(ctx context.Context, relPath string) error {
	// Reload state
	if err := m.fileManager.LoadState(m.appConfig.Storage.StatePath); err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	// Scan directory
	if err := m.fileManager.ScanDirectory(); err != nil {
		return fmt.Errorf("failed to scan directory: %w", err)
	}

	// Perform sync
	if err := m.syncer.SyncAll(ctx, os.Stdout); err != nil {
		return fmt.Errorf("failed to sync: %w", err)
	}

	// Save state
	if err := m.fileManager.SaveState(m.appConfig.Storage.StatePath); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	return nil
}

// startPeriodicRemoteCheck checks for remote changes periodically
func (m *Manager) startPeriodicRemoteCheck(ctx context.Context) {
	ticker := time.NewTicker(m.config.CheckRemoteInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if m.networkDetector.IsConnected() {
				m.enqueueRemoteCheck()
			}
		case <-ctx.Done():
			return
		}
	}
}

// checkRemoteChanges looks for changes in the remote repository
func (m *Manager) checkRemoteChanges() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	m.logger.Debugf("Checking for remote changes")

	// Get remote files
	remoteFiles, err := m.repo.GetAllFilesWithContent(ctx)
	if err != nil {
		m.logger.Errorf("Failed to get remote files: %v", err)
		return
	}

	// Check if we have any remote-only files or modified files
	localFiles := m.fileManager.GetTrackedFiles()
	hasChanges := false

	// Check for remote-only files
	for remotePath := range remoteFiles {
		found := false
		for _, localFile := range localFiles {
			if relPath, err := filepath.Rel(m.appConfig.Storage.BaseDir, localFile.Path); err == nil {
				if relPath == remotePath {
					found = true
					break
				}
			}
		}
		if !found {
			hasChanges = true
			break
		}
	}

	if hasChanges {
		m.logger.Infof("Remote changes detected, syncing...")
		m.enqueueSync("") // Sync all files
	}
}

// startQueueCleanup periodically cleans up old queue entries
func (m *Manager) startQueueCleanup(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Clean up operations older than 24 hours or with too many retries
			if err := m.queue.Cleanup(24*time.Hour, m.config.RetryAttempts); err != nil {
				m.logger.Errorf("Failed to cleanup queue: %v", err)
			}
		}
	}
}

// Stop gracefully stops the auto-sync manager
func (m *Manager) Stop() error {
	m.logger.Infof("Stopping auto-sync manager")
	m.shutdown()
	select {
	case <-m.workerStarted:
		<-m.workerDone
	default:
	}
	return nil
}

func (m *Manager) shutdown() {
	m.shutdownOnce.Do(func() {
		close(m.done)
		m.changeDebouncer.Stop()
		if err := m.watcher.Close(); err != nil {
			m.logger.Errorf("Failed to stop file watcher: %v", err)
		}
	})
}
