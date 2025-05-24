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

	t.Run("SyncExistingFiles", func(t *testing.T) {
		mockRepo.ExpectedCalls = nil
		mockRepo.Calls = nil

		// Mock FileExists to return true for both files
		mockRepo.On("FileExists", mock.Anything, "test1.txt").Return(true, nil).Once()
		mockRepo.On("FileExists", mock.Anything, "test2.txt").Return(true, nil).Once()

		// Mock GetFile to return different content
		mockRepo.On("GetFile", mock.Anything, "test1.txt").Return("remote content 1", nil).Once()
		mockRepo.On("GetFile", mock.Anything, "test2.txt").Return("remote content 2", nil).Once()

		// Mock UpdateFile for both files (using local content)
		mockRepo.On("UpdateFile", mock.Anything, "test1.txt", "test content 1").Return(nil).Once()
		mockRepo.On("UpdateFile", mock.Anything, "test2.txt", "test content 2").Return(nil).Once()

		// Run sync
		err := syncer.SyncAll(context.Background(), os.Stdout)
		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("SyncWithErrors", func(t *testing.T) {
		mockRepo.ExpectedCalls = nil
		mockRepo.Calls = nil

		// Mock FileExists to return error for first file
		mockRepo.On("FileExists", mock.Anything, "test1.txt").Return(false, assert.AnError).Once()
		mockRepo.On("FileExists", mock.Anything, "test2.txt").Return(false, nil).Once()

		// Mock CreateFile for second file
		mockRepo.On("CreateFile", mock.Anything, "test2.txt", "test content 2").Return(nil).Once()

		// Run sync
		err := syncer.SyncAll(context.Background(), os.Stdout)
		assert.NoError(t, err) // Sync continues despite errors
		mockRepo.AssertExpectations(t)
	})
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
		result := syncer.syncFile(context.Background(), &storage.FileInfo{
			Path: testFile,
			Hash: "testhash",
		})
		assert.NoError(t, result.Error)
		assert.Equal(t, SyncStatusLocalChanges, result.Status)
		mockRepo.AssertExpectations(t)
	})

	t.Run("ExistingFileNoChanges", func(t *testing.T) {
		mockRepo.ExpectedCalls = nil
		mockRepo.Calls = nil

		// Mock FileExists to return true
		mockRepo.On("FileExists", mock.Anything, "test.txt").Return(true, nil).Once()

		// Mock GetFile to return same content
		mockRepo.On("GetFile", mock.Anything, "test.txt").Return("test content", nil).Once()

		// Run sync
		result := syncer.syncFile(context.Background(), &storage.FileInfo{
			Path: testFile,
			Hash: "testhash",
		})
		assert.NoError(t, result.Error)
		assert.Equal(t, SyncStatusSynced, result.Status)
		mockRepo.AssertExpectations(t)
	})

	t.Run("ExistingFileWithChanges", func(t *testing.T) {
		mockRepo.ExpectedCalls = nil
		mockRepo.Calls = nil

		// Mock FileExists to return true
		mockRepo.On("FileExists", mock.Anything, "test.txt").Return(true, nil).Once()

		// Mock GetFile to return different content
		mockRepo.On("GetFile", mock.Anything, "test.txt").Return("remote content", nil).Once()

		// Mock UpdateFile
		mockRepo.On("UpdateFile", mock.Anything, "test.txt", "test content").Return(nil).Once()

		// Run sync
		result := syncer.syncFile(context.Background(), &storage.FileInfo{
			Path: testFile,
			Hash: "testhash",
		})
		assert.NoError(t, result.Error)
		assert.Equal(t, SyncStatusConflict, result.Status)
		mockRepo.AssertExpectations(t)
	})

	t.Run("FileExistsError", func(t *testing.T) {
		mockRepo.ExpectedCalls = nil
		mockRepo.Calls = nil

		// Mock FileExists to return error
		mockRepo.On("FileExists", mock.Anything, "test.txt").Return(false, assert.AnError).Once()

		// Run sync
		result := syncer.syncFile(context.Background(), &storage.FileInfo{
			Path: testFile,
			Hash: "testhash",
		})
		assert.Error(t, result.Error)
		assert.Equal(t, assert.AnError, result.Error)
		mockRepo.AssertExpectations(t)
	})
}
