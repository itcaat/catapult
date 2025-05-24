package sync

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/itcaat/catapult/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockRepository is a mock implementation of the repository interface
type MockRepository struct {
	mock.Mock
}

func (m *MockRepository) EnsureExists(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockRepository) GetDefaultBranch(ctx context.Context) (string, error) {
	args := m.Called(ctx)
	return args.String(0), args.Error(1)
}

func (m *MockRepository) CreateFile(ctx context.Context, path, content string) error {
	args := m.Called(ctx, path, content)
	return args.Error(0)
}

func (m *MockRepository) GetFile(ctx context.Context, path string) (string, error) {
	args := m.Called(ctx, path)
	return args.String(0), args.Error(1)
}

func (m *MockRepository) UpdateFile(ctx context.Context, path, content string) error {
	args := m.Called(ctx, path, content)
	return args.Error(0)
}

func (m *MockRepository) DeleteFile(ctx context.Context, path string) error {
	args := m.Called(ctx, path)
	return args.Error(0)
}

func (m *MockRepository) FileExists(ctx context.Context, path string) (bool, error) {
	args := m.Called(ctx, path)
	return args.Bool(0), args.Error(1)
}

func TestSyncFile(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "catapult-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create test file
	testFile := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	assert.NoError(t, err)

	// Create file manager
	fileManager := storage.NewFileManager(tempDir)
	err = fileManager.AddFile(testFile)
	assert.NoError(t, err)

	// Create mock repository
	mockRepo := new(MockRepository)

	// Create sync instance
	sync := New(mockRepo, fileManager)

	// Вспомогательная функция для выставления sync info
	setSyncInfo := func(path, lastSyncedHash, lastSyncedRemoteSHA string) {
		fi, _ := fileManager.GetFileInfo(path)
		fi.LastSyncedHash = lastSyncedHash
		fi.LastSyncedRemoteSHA = lastSyncedRemoteSHA
	}

	t.Run("SyncStatusSynced", func(t *testing.T) {
		mockRepo.ExpectedCalls = nil
		mockRepo.Calls = nil
		content, _ := os.ReadFile(testFile)
		h := sha256.Sum256(content)
		hash := hex.EncodeToString(h[:])
		setSyncInfo(testFile, hash, "sha123")
		status, err := fileManager.GetSyncStatus(testFile)
		assert.NoError(t, err)
		t.Logf("Sync status before test: %v", status)
		assert.Equal(t, storage.SyncStatusSynced, status)
		err = sync.SyncFile(context.Background(), testFile)
		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("SyncStatusLocalChanges", func(t *testing.T) {
		mockRepo.ExpectedCalls = nil
		mockRepo.Calls = nil
		err := os.WriteFile(testFile, []byte("updated content"), 0644)
		assert.NoError(t, err)
		setSyncInfo(testFile, "oldhash", "sha123")
		status, err := fileManager.GetSyncStatus(testFile)
		assert.NoError(t, err)
		t.Logf("Sync status before test: %v", status)
		assert.Equal(t, storage.SyncStatusLocalChanges, status)
		mockRepo.On("UpdateFile", mock.Anything, "test.txt", "updated content").Return(nil).Once()
		err = sync.SyncFile(context.Background(), testFile)
		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("SyncStatusRemoteChanges", func(t *testing.T) {
		mockRepo.ExpectedCalls = nil
		mockRepo.Calls = nil
		content, _ := os.ReadFile(testFile)
		h := sha256.Sum256(content)
		hash := hex.EncodeToString(h[:])
		setSyncInfo(testFile, hash, "oldsha")
		status, err := fileManager.GetSyncStatus(testFile)
		assert.NoError(t, err)
		t.Logf("Sync status before test: %v", status)
		assert.Equal(t, storage.SyncStatusRemoteChanges, status)
		mockRepo.On("GetFile", mock.Anything, "test.txt").Return("remote content", nil).Once()
		err = sync.SyncFile(context.Background(), testFile)
		assert.NoError(t, err)
		content, err = os.ReadFile(testFile)
		assert.NoError(t, err)
		assert.Equal(t, "remote content", string(content))
		mockRepo.AssertExpectations(t)
	})

	t.Run("SyncStatusConflict", func(t *testing.T) {
		mockRepo.ExpectedCalls = nil
		mockRepo.Calls = nil
		err := os.WriteFile(testFile, []byte("local content"), 0644)
		assert.NoError(t, err)
		setSyncInfo(testFile, "oldhash", "oldsha")
		status, err := fileManager.GetSyncStatus(testFile)
		assert.NoError(t, err)
		t.Logf("Sync status before test: %v", status)
		assert.Equal(t, storage.SyncStatusConflict, status)
		mockRepo.On("GetFile", mock.Anything, "test.txt").Return("remote content", nil).Once()
		err = sync.SyncFile(context.Background(), testFile)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "conflict detected")
		localConflict := filepath.Join(tempDir, ".catapult", "conflicts", "test.txt.local")
		remoteConflict := filepath.Join(tempDir, ".catapult", "conflicts", "test.txt.remote")
		assert.FileExists(t, localConflict)
		assert.FileExists(t, remoteConflict)
		localContent, err := os.ReadFile(localConflict)
		assert.NoError(t, err)
		assert.Equal(t, "local content", string(localContent))
		remoteContent, err := os.ReadFile(remoteConflict)
		assert.NoError(t, err)
		assert.Equal(t, "remote content", string(remoteContent))
		mockRepo.AssertExpectations(t)
	})
}

func TestPullFile(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "catapult-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create test file
	testFile := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	assert.NoError(t, err)

	// Create file manager
	fileManager := storage.NewFileManager(tempDir)
	err = fileManager.AddFile(testFile)
	assert.NoError(t, err)

	// Create mock repository
	mockRepo := new(MockRepository)

	// Create sync instance
	sync := New(mockRepo, fileManager)

	// Вспомогательная функция для выставления sync info
	setSyncInfo := func(path, lastSyncedHash, lastSyncedRemoteSHA string) {
		fi, _ := fileManager.GetFileInfo(path)
		fi.LastSyncedHash = lastSyncedHash
		fi.LastSyncedRemoteSHA = lastSyncedRemoteSHA
	}

	t.Run("SyncStatusSynced", func(t *testing.T) {
		mockRepo.ExpectedCalls = nil
		mockRepo.Calls = nil
		content, _ := os.ReadFile(testFile)
		h := sha256.Sum256(content)
		hash := hex.EncodeToString(h[:])
		setSyncInfo(testFile, hash, "sha123")
		status, err := fileManager.GetSyncStatus(testFile)
		assert.NoError(t, err)
		t.Logf("Sync status before test: %v", status)
		assert.Equal(t, storage.SyncStatusSynced, status)
		err = sync.PullFile(context.Background(), testFile)
		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("SyncStatusLocalChanges", func(t *testing.T) {
		mockRepo.ExpectedCalls = nil
		mockRepo.Calls = nil
		err := os.WriteFile(testFile, []byte("local content"), 0644)
		assert.NoError(t, err)
		setSyncInfo(testFile, "oldhash", "sha123")
		status, err := fileManager.GetSyncStatus(testFile)
		assert.NoError(t, err)
		t.Logf("Sync status before test: %v", status)
		assert.Equal(t, storage.SyncStatusLocalChanges, status)
		mockRepo.On("GetFile", mock.Anything, "test.txt").Return("remote content", nil).Once()
		err = sync.PullFile(context.Background(), testFile)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "conflict detected")
		localConflict := filepath.Join(tempDir, ".catapult", "conflicts", "test.txt.local")
		remoteConflict := filepath.Join(tempDir, ".catapult", "conflicts", "test.txt.remote")
		assert.FileExists(t, localConflict)
		assert.FileExists(t, remoteConflict)
		localContent, err := os.ReadFile(localConflict)
		assert.NoError(t, err)
		assert.Equal(t, "local content", string(localContent))
		remoteContent, err := os.ReadFile(remoteConflict)
		assert.NoError(t, err)
		assert.Equal(t, "remote content", string(remoteContent))
		mockRepo.AssertExpectations(t)
	})

	t.Run("SyncStatusRemoteChanges", func(t *testing.T) {
		mockRepo.ExpectedCalls = nil
		mockRepo.Calls = nil
		content, _ := os.ReadFile(testFile)
		h := sha256.Sum256(content)
		hash := hex.EncodeToString(h[:])
		setSyncInfo(testFile, hash, "oldsha")
		status, err := fileManager.GetSyncStatus(testFile)
		assert.NoError(t, err)
		t.Logf("Sync status before test: %v", status)
		assert.Equal(t, storage.SyncStatusRemoteChanges, status)
		mockRepo.On("GetFile", mock.Anything, "test.txt").Return("remote content", nil).Once()
		err = sync.PullFile(context.Background(), testFile)
		assert.NoError(t, err)
		content, err = os.ReadFile(testFile)
		assert.NoError(t, err)
		assert.Equal(t, "remote content", string(content))
		mockRepo.AssertExpectations(t)
	})
}
