package autosync

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// QueueOperation represents a queued sync operation
type QueueOperation struct {
	ID        string    `json:"id"`
	FilePath  string    `json:"file_path"`
	Operation string    `json:"operation"` // "sync", "delete", "create"
	Timestamp time.Time `json:"timestamp"`
	Retries   int       `json:"retries"`
	LastError string    `json:"last_error,omitempty"`
}

// Queue manages offline sync operations
type Queue struct {
	operations map[string]*QueueOperation
	queuePath  string
	maxSize    int
	mutex      sync.RWMutex
}

// NewQueue creates a new offline operations queue
func NewQueue(queuePath string, maxSize int) *Queue {
	return &Queue{
		operations: make(map[string]*QueueOperation),
		queuePath:  queuePath,
		maxSize:    maxSize,
	}
}

// Add adds a new operation to the queue
func (q *Queue) Add(operation *QueueOperation) error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	// Check if queue is at capacity
	if len(q.operations) >= q.maxSize {
		// Remove oldest operation
		q.removeOldest()
	}

	// Generate ID if not provided
	if operation.ID == "" {
		operation.ID = fmt.Sprintf("%s_%d", operation.FilePath, time.Now().UnixNano())
	}

	// Set timestamp if not provided
	if operation.Timestamp.IsZero() {
		operation.Timestamp = time.Now()
	}

	q.operations[operation.ID] = operation
	return q.persist()
}

// Remove removes an operation from the queue
func (q *Queue) Remove(operationID string) error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	delete(q.operations, operationID)
	return q.persist()
}

// GetPending returns all pending operations
func (q *Queue) GetPending() []*QueueOperation {
	q.mutex.RLock()
	defer q.mutex.RUnlock()

	operations := make([]*QueueOperation, 0, len(q.operations))
	for _, op := range q.operations {
		operations = append(operations, op)
	}

	return operations
}

// Size returns the current queue size
func (q *Queue) Size() int {
	q.mutex.RLock()
	defer q.mutex.RUnlock()
	return len(q.operations)
}

// UpdateRetry increments retry count and updates error for an operation
func (q *Queue) UpdateRetry(operationID string, err error) error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	if op, exists := q.operations[operationID]; exists {
		op.Retries++
		if err != nil {
			op.LastError = err.Error()
		}
		return q.persist()
	}

	return fmt.Errorf("operation %s not found", operationID)
}

// Load loads the queue from persistent storage
func (q *Queue) Load() error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	// Create directory if it doesn't exist
	dir := filepath.Dir(q.queuePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create queue directory: %w", err)
	}

	// Check if file exists
	if _, err := os.Stat(q.queuePath); os.IsNotExist(err) {
		// File doesn't exist, start with empty queue
		return nil
	}

	// Read file
	data, err := os.ReadFile(q.queuePath)
	if err != nil {
		return fmt.Errorf("failed to read queue file: %w", err)
	}

	// Parse JSON
	var operations map[string]*QueueOperation
	if err := json.Unmarshal(data, &operations); err != nil {
		return fmt.Errorf("failed to parse queue file: %w", err)
	}

	q.operations = operations
	if q.operations == nil {
		q.operations = make(map[string]*QueueOperation)
	}

	return nil
}

// persist saves the queue to persistent storage
func (q *Queue) persist() error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(q.queuePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create queue directory: %w", err)
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(q.operations, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal queue: %w", err)
	}

	// Write to file
	if err := os.WriteFile(q.queuePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write queue file: %w", err)
	}

	return nil
}

// removeOldest removes the oldest operation from the queue
func (q *Queue) removeOldest() {
	var oldest *QueueOperation
	var oldestID string

	for id, op := range q.operations {
		if oldest == nil || op.Timestamp.Before(oldest.Timestamp) {
			oldest = op
			oldestID = id
		}
	}

	if oldestID != "" {
		delete(q.operations, oldestID)
	}
}

// Clear removes all operations from the queue
func (q *Queue) Clear() error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	q.operations = make(map[string]*QueueOperation)
	return q.persist()
}

// GetOldOperations returns operations older than the specified duration
func (q *Queue) GetOldOperations(maxAge time.Duration) []*QueueOperation {
	q.mutex.RLock()
	defer q.mutex.RUnlock()

	cutoff := time.Now().Add(-maxAge)
	var oldOps []*QueueOperation

	for _, op := range q.operations {
		if op.Timestamp.Before(cutoff) {
			oldOps = append(oldOps, op)
		}
	}

	return oldOps
}

// Cleanup removes operations that are too old or have too many retries
func (q *Queue) Cleanup(maxAge time.Duration, maxRetries int) error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	cutoff := time.Now().Add(-maxAge)
	toRemove := make([]string, 0)

	for id, op := range q.operations {
		if op.Timestamp.Before(cutoff) || op.Retries >= maxRetries {
			toRemove = append(toRemove, id)
		}
	}

	for _, id := range toRemove {
		delete(q.operations, id)
	}

	if len(toRemove) > 0 {
		return q.persist()
	}

	return nil
}
