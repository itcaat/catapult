package sync

import (
	"context"
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

func (m *MockRepository) ListFiles(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	return args.Get(0).([]string), args.Error(1)
}

func TestSyncAll(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "catapult-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create test files
	testFile1 := filepath.Join(tempDir, "test1.txt")
	testFile2 := filepath.Join(tempDir, "test2.txt")
	err = os.WriteFile(testFile1, []byte("test content 1"), 0644)
	assert.NoError(t, err)
	err = os.WriteFile(testFile2, []byte("test content 2"), 0644)
	assert.NoError(t, err)

	// Create file manager
	fileManager := storage.NewFileManager(tempDir)

	// Create mock repository
	mockRepo := new(MockRepository)

	// Create sync instance
	syncer := New(mockRepo, fileManager)

	t.Run("SyncNewFiles", func(t *testing.T) {
		mockRepo.ExpectedCalls = nil
		mockRepo.Calls = nil

		// Mock ListFiles to return empty list (no remote files)
		mockRepo.On("ListFiles", mock.Anything).Return([]string{}, nil).Once()

		// Mock FileExists to return false for both files
		mockRepo.On("FileExists", mock.Anything, "test1.txt").Return(false, nil).Once()
		mockRepo.On("FileExists", mock.Anything, "test2.txt").Return(false, nil).Once()

		// Mock CreateFile for both files
		mockRepo.On("CreateFile", mock.Anything, "test1.txt", "test content 1").Return(nil).Once()
		mockRepo.On("CreateFile", mock.Anything, "test2.txt", "test content 2").Return(nil).Once()

		// Run sync
		err := syncer.SyncAll(context.Background(), os.Stdout)
		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("SyncRemoteFiles", func(t *testing.T) {
		mockRepo.ExpectedCalls = nil
		mockRepo.Calls = nil

		// Mock ListFiles to return remote-only files
		mockRepo.On("ListFiles", mock.Anything).Return([]string{"test1.txt", "test2.txt", "remote.txt"}, nil).Once()

		// Mock for existing local files
		mockRepo.On("FileExists", mock.Anything, "test1.txt").Return(true, nil).Once()
		mockRepo.On("FileExists", mock.Anything, "test2.txt").Return(true, nil).Once()
		mockRepo.On("GetFile", mock.Anything, "test1.txt").Return("test content 1", nil).Once()
		mockRepo.On("GetFile", mock.Anything, "test2.txt").Return("test content 2", nil).Once()

		// Mock for remote-only file
		mockRepo.On("FileExists", mock.Anything, "remote.txt").Return(true, nil).Once()
		mockRepo.On("GetFile", mock.Anything, "remote.txt").Return("remote file content", nil).Once()

		// Run sync
		err := syncer.SyncAll(context.Background(), os.Stdout)
		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)

		// Check that remote file was downloaded
		remoteFilePath := filepath.Join(tempDir, "remote.txt")
		content, err := os.ReadFile(remoteFilePath)
		assert.NoError(t, err)
		assert.Equal(t, "remote file content", string(content))
	})

	t.Run("SyncWithErrors", func(t *testing.T) {
		// Create separate temp directory for this test
		errorTestDir, err := os.MkdirTemp("", "catapult-error-test-*")
		assert.NoError(t, err)
		defer os.RemoveAll(errorTestDir)

		// Create test files for error test
		errorFile1 := filepath.Join(errorTestDir, "error1.txt")
		errorFile2 := filepath.Join(errorTestDir, "error2.txt")
		err = os.WriteFile(errorFile1, []byte("error content 1"), 0644)
		assert.NoError(t, err)
		err = os.WriteFile(errorFile2, []byte("error content 2"), 0644)
		assert.NoError(t, err)

		// Create new file manager for error test
		errorFileManager := storage.NewFileManager(errorTestDir)

		// Create new sync instance for error test
		errorSyncer := New(mockRepo, errorFileManager)

		mockRepo.ExpectedCalls = nil
		mockRepo.Calls = nil

		// Mock ListFiles to return empty list
		mockRepo.On("ListFiles", mock.Anything).Return([]string{}, nil).Once()

		// Mock FileExists to return error for first file
		mockRepo.On("FileExists", mock.Anything, "error1.txt").Return(false, assert.AnError).Once()
		mockRepo.On("FileExists", mock.Anything, "error2.txt").Return(false, nil).Once()

		// Mock CreateFile for second file
		mockRepo.On("CreateFile", mock.Anything, "error2.txt", "error content 2").Return(nil).Once()

		// Run sync
		err = errorSyncer.SyncAll(context.Background(), os.Stdout)
		assert.NoError(t, err) // Sync continues despite errors
		mockRepo.AssertExpectations(t)
	})
}

func TestSyncFileByPath(t *testing.T) {
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

	// Create mock repository
	mockRepo := new(MockRepository)

	// Create sync instance
	syncer := New(mockRepo, fileManager)

	// Scan directory to track files
	err = fileManager.ScanDirectory()
	assert.NoError(t, err)

	t.Run("NewFile", func(t *testing.T) {
		mockRepo.ExpectedCalls = nil
		mockRepo.Calls = nil

		// Mock FileExists to return false
		mockRepo.On("FileExists", mock.Anything, "test.txt").Return(false, nil).Once()

		// Mock CreateFile
		mockRepo.On("CreateFile", mock.Anything, "test.txt", "test content").Return(nil).Once()

		// Run sync
		result := syncer.syncFileByPath(context.Background(), &storage.FileInfo{
			Path: testFile,
			Hash: "testhash",
		}, "test.txt")
		assert.NoError(t, result.Error)
		assert.Equal(t, SyncStatusLocalChanges, result.Status)
		mockRepo.AssertExpectations(t)
	})

	t.Run("RemoteOnlyFile", func(t *testing.T) {
		mockRepo.ExpectedCalls = nil
		mockRepo.Calls = nil

		// Create a path for non-existent local file
		remoteOnlyFile := filepath.Join(tempDir, "remote_only.txt")

		// Mock FileExists to return true (exists in remote)
		mockRepo.On("FileExists", mock.Anything, "remote_only.txt").Return(true, nil).Once()

		// Mock GetFile to return remote content
		mockRepo.On("GetFile", mock.Anything, "remote_only.txt").Return("remote content", nil).Once()

		// Run sync
		result := syncer.syncFileByPath(context.Background(), &storage.FileInfo{
			Path: remoteOnlyFile,
			Hash: "",
		}, "remote_only.txt")
		assert.NoError(t, result.Error)
		assert.Equal(t, SyncStatusRemoteChanges, result.Status)
		mockRepo.AssertExpectations(t)

		// Check that file was downloaded
		content, err := os.ReadFile(remoteOnlyFile)
		assert.NoError(t, err)
		assert.Equal(t, "remote content", string(content))
	})

	t.Run("ExistingFileNoChanges", func(t *testing.T) {
		mockRepo.ExpectedCalls = nil
		mockRepo.Calls = nil

		// Mock FileExists to return true
		mockRepo.On("FileExists", mock.Anything, "test.txt").Return(true, nil).Once()

		// Mock GetFile to return same content
		mockRepo.On("GetFile", mock.Anything, "test.txt").Return("test content", nil).Once()

		// Run sync
		result := syncer.syncFileByPath(context.Background(), &storage.FileInfo{
			Path: testFile,
			Hash: "testhash",
		}, "test.txt")
		assert.NoError(t, result.Error)
		assert.Equal(t, SyncStatusSynced, result.Status)
		mockRepo.AssertExpectations(t)
	})

	t.Run("FileExistsError", func(t *testing.T) {
		mockRepo.ExpectedCalls = nil
		mockRepo.Calls = nil

		// Mock FileExists to return error
		mockRepo.On("FileExists", mock.Anything, "test.txt").Return(false, assert.AnError).Once()

		// Run sync
		result := syncer.syncFileByPath(context.Background(), &storage.FileInfo{
			Path: testFile,
			Hash: "testhash",
		}, "test.txt")
		assert.Error(t, result.Error)
		assert.Equal(t, assert.AnError, result.Error)
		mockRepo.AssertExpectations(t)
	})
}
