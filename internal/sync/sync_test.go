package sync

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/itcaat/catapult/internal/repository"
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

func (m *MockRepository) GetAllFilesWithContent(ctx context.Context) (map[string]*repository.RemoteFileInfo, error) {
	args := m.Called(ctx)
	return args.Get(0).(map[string]*repository.RemoteFileInfo), args.Error(1)
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

		// Mock GetAllFilesWithContent to return empty map (no remote files)
		mockRepo.On("GetAllFilesWithContent", mock.Anything).Return(map[string]*repository.RemoteFileInfo{}, nil).Once()

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

		// Calculate Git SHA for local content to match remote
		localGitSHA1 := fileManager.CalculateGitSHAFromContent([]byte("test content 1"))
		localGitSHA2 := fileManager.CalculateGitSHAFromContent([]byte("test content 2"))

		// Mock GetAllFilesWithContent to return remote files
		remoteFiles := map[string]*repository.RemoteFileInfo{
			"test1.txt": {
				Path:    "test1.txt",
				Content: "test content 1",
				SHA:     localGitSHA1, // Same as local
				Size:    len("test content 1"),
			},
			"test2.txt": {
				Path:    "test2.txt",
				Content: "test content 2",
				SHA:     localGitSHA2, // Same as local
				Size:    len("test content 2"),
			},
			"remote.txt": {
				Path:    "remote.txt",
				Content: "remote file content",
				SHA:     "sha3",
				Size:    len("remote file content"),
			},
		}
		mockRepo.On("GetAllFilesWithContent", mock.Anything).Return(remoteFiles, nil).Once()

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

		// Mock GetAllFilesWithContent to return empty map
		mockRepo.On("GetAllFilesWithContent", mock.Anything).Return(map[string]*repository.RemoteFileInfo{}, nil).Once()

		// Mock CreateFile calls - one will succeed, one will fail
		mockRepo.On("CreateFile", mock.Anything, "error1.txt", "error content 1").Return(assert.AnError).Once()
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

		// Mock CreateFile
		mockRepo.On("CreateFile", mock.Anything, "test.txt", "test content").Return(nil).Once()

		// Run sync
		result := syncer.syncFileByPath(context.Background(), &storage.FileInfo{
			Path: testFile,
			Hash: "testhash",
		}, "test.txt", nil) // nil means file doesn't exist in remote
		assert.NoError(t, result.Error)
		assert.Equal(t, SyncStatusLocalChanges, result.Status)
		mockRepo.AssertExpectations(t)
	})

	t.Run("RemoteOnlyFile", func(t *testing.T) {
		mockRepo.ExpectedCalls = nil
		mockRepo.Calls = nil

		// Create a path for non-existent local file
		remoteOnlyFile := filepath.Join(tempDir, "remote_only.txt")

		// Create RemoteFileInfo for remote file
		remoteFileInfo := &repository.RemoteFileInfo{
			Path:    "remote_only.txt",
			Content: "remote content",
			SHA:     "remotesha123",
			Size:    len("remote content"),
		}

		// Run sync
		result := syncer.syncFileByPath(context.Background(), &storage.FileInfo{
			Path: remoteOnlyFile,
			Hash: "",
		}, "remote_only.txt", remoteFileInfo)
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

		// Calculate Git SHA for test content
		localGitSHA := fileManager.CalculateGitSHAFromContent([]byte("test content"))

		// Create RemoteFileInfo with same content
		remoteFileInfo := &repository.RemoteFileInfo{
			Path:    "test.txt",
			Content: "test content",
			SHA:     localGitSHA, // Same SHA as local
			Size:    len("test content"),
		}

		// Run sync
		result := syncer.syncFileByPath(context.Background(), &storage.FileInfo{
			Path: testFile,
			Hash: "testhash",
		}, "test.txt", remoteFileInfo)
		assert.NoError(t, result.Error)
		assert.Equal(t, SyncStatusSynced, result.Status)
		mockRepo.AssertExpectations(t)
	})

	t.Run("FileReadError", func(t *testing.T) {
		mockRepo.ExpectedCalls = nil
		mockRepo.Calls = nil

		// Create a file that exists but will cause read error
		// We'll create a directory with the same name as the file to cause read error
		errorDir := filepath.Join(tempDir, "error_file")
		err := os.Mkdir(errorDir, 0755)
		assert.NoError(t, err)

		// Run sync with nil remote file (local only) - this should fail when trying to read the directory as a file
		result := syncer.syncFileByPath(context.Background(), &storage.FileInfo{
			Path: errorDir,
			Hash: "testhash",
		}, "error_file", nil)
		assert.Error(t, result.Error)
		mockRepo.AssertExpectations(t)
	})
}
