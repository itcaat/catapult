package autosync

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestQueue_Add(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "queue_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	queuePath := filepath.Join(tempDir, "test_queue.json")
	queue := NewQueue(queuePath, 10)

	op := &QueueOperation{
		FilePath:  "test.txt",
		Operation: "sync",
	}

	err = queue.Add(op)
	if err != nil {
		t.Errorf("Failed to add operation: %v", err)
	}

	if queue.Size() != 1 {
		t.Errorf("Expected queue size 1, got %d", queue.Size())
	}

	// Check that operation was persisted
	newQueue := NewQueue(queuePath, 10)
	err = newQueue.Load()
	if err != nil {
		t.Errorf("Failed to load queue: %v", err)
	}

	if newQueue.Size() != 1 {
		t.Errorf("Expected loaded queue size 1, got %d", newQueue.Size())
	}
}

func TestQueue_Remove(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "queue_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	queuePath := filepath.Join(tempDir, "test_queue.json")
	queue := NewQueue(queuePath, 10)

	op := &QueueOperation{
		ID:        "test_op",
		FilePath:  "test.txt",
		Operation: "sync",
	}

	queue.Add(op)

	err = queue.Remove("test_op")
	if err != nil {
		t.Errorf("Failed to remove operation: %v", err)
	}

	if queue.Size() != 0 {
		t.Errorf("Expected queue size 0 after removal, got %d", queue.Size())
	}
}

func TestQueue_MaxSize(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "queue_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	queuePath := filepath.Join(tempDir, "test_queue.json")
	queue := NewQueue(queuePath, 2) // Max size of 2

	// Add 3 operations
	for i := 0; i < 3; i++ {
		op := &QueueOperation{
			FilePath:  "test" + string(rune(i)) + ".txt",
			Operation: "sync",
			Timestamp: time.Now().Add(time.Duration(i) * time.Second),
		}
		queue.Add(op)
		time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	}

	// Should only have 2 operations (oldest one removed)
	if queue.Size() != 2 {
		t.Errorf("Expected queue size 2 after exceeding max size, got %d", queue.Size())
	}
}

func TestQueue_UpdateRetry(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "queue_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	queuePath := filepath.Join(tempDir, "test_queue.json")
	queue := NewQueue(queuePath, 10)

	op := &QueueOperation{
		ID:        "test_op",
		FilePath:  "test.txt",
		Operation: "sync",
	}

	queue.Add(op)

	err = queue.UpdateRetry("test_op", nil)
	if err != nil {
		t.Errorf("Failed to update retry: %v", err)
	}

	pending := queue.GetPending()
	if len(pending) != 1 {
		t.Fatal("Expected 1 pending operation")
	}

	if pending[0].Retries != 1 {
		t.Errorf("Expected retry count 1, got %d", pending[0].Retries)
	}
}

func TestQueue_Cleanup(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "queue_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	queuePath := filepath.Join(tempDir, "test_queue.json")
	queue := NewQueue(queuePath, 10)

	// Add old operation
	oldOp := &QueueOperation{
		ID:        "old_op",
		FilePath:  "old.txt",
		Operation: "sync",
		Timestamp: time.Now().Add(-25 * time.Hour), // 25 hours ago
	}
	queue.Add(oldOp)

	// Add operation with too many retries
	retryOp := &QueueOperation{
		ID:        "retry_op",
		FilePath:  "retry.txt",
		Operation: "sync",
		Retries:   5,
	}
	queue.Add(retryOp)

	// Add normal operation
	normalOp := &QueueOperation{
		ID:        "normal_op",
		FilePath:  "normal.txt",
		Operation: "sync",
	}
	queue.Add(normalOp)

	// Cleanup operations older than 24 hours or with more than 3 retries
	err = queue.Cleanup(24*time.Hour, 3)
	if err != nil {
		t.Errorf("Failed to cleanup queue: %v", err)
	}

	// Should only have the normal operation left
	if queue.Size() != 1 {
		t.Errorf("Expected 1 operation after cleanup, got %d", queue.Size())
	}

	pending := queue.GetPending()
	if len(pending) != 1 || pending[0].ID != "normal_op" {
		t.Error("Expected only normal_op to remain after cleanup")
	}
}

func TestQueue_GetOldOperations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "queue_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	queuePath := filepath.Join(tempDir, "test_queue.json")
	queue := NewQueue(queuePath, 10)

	// Add old operation
	oldOp := &QueueOperation{
		ID:        "old_op",
		FilePath:  "old.txt",
		Operation: "sync",
		Timestamp: time.Now().Add(-2 * time.Hour),
	}
	queue.Add(oldOp)

	// Add new operation
	newOp := &QueueOperation{
		ID:        "new_op",
		FilePath:  "new.txt",
		Operation: "sync",
		Timestamp: time.Now(),
	}
	queue.Add(newOp)

	oldOps := queue.GetOldOperations(1 * time.Hour)
	if len(oldOps) != 1 {
		t.Errorf("Expected 1 old operation, got %d", len(oldOps))
	}

	if oldOps[0].ID != "old_op" {
		t.Error("Expected old_op to be returned as old operation")
	}
}

func TestQueue_Clear(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "queue_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	queuePath := filepath.Join(tempDir, "test_queue.json")
	queue := NewQueue(queuePath, 10)

	// Add some operations
	for i := 0; i < 3; i++ {
		op := &QueueOperation{
			FilePath:  "test" + string(rune(i)) + ".txt",
			Operation: "sync",
		}
		queue.Add(op)
	}

	err = queue.Clear()
	if err != nil {
		t.Errorf("Failed to clear queue: %v", err)
	}

	if queue.Size() != 0 {
		t.Errorf("Expected queue size 0 after clear, got %d", queue.Size())
	}
}
