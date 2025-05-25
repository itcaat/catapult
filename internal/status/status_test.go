package status

import (
	"bytes"
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

func TestPrintStatus(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "catapult-status-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create test files
	testFile1 := filepath.Join(tempDir, "local1.txt")
	testFile2 := filepath.Join(tempDir, "both.txt")

	err = os.WriteFile(testFile1, []byte("local content 1"), 0644)
	assert.NoError(t, err)
	err = os.WriteFile(testFile2, []byte("shared content"), 0644)
	assert.NoError(t, err)

	// Create file manager
	fileManager := storage.NewFileManager(tempDir)

	// Scan directory to track files automatically
	err = fileManager.ScanDirectory()
	assert.NoError(t, err)

	// Create mock repository
	mockRepo := new(MockRepository)

	t.Run("ShowLocalAndRemoteFiles", func(t *testing.T) {
		// Mock GetAllFilesWithContent to return mixed local/remote files
		remoteFiles := map[string]*repository.RemoteFileInfo{
			"both.txt": {
				Path:    "both.txt",
				Content: "shared content",
				SHA:     "sha123",
				Size:    len("shared content"),
			},
			"remote1.txt": {
				Path:    "remote1.txt",
				Content: "remote content 1",
				SHA:     "sha456",
				Size:    len("remote content 1"),
			},
			"remote2.txt": {
				Path:    "remote2.txt",
				Content: "remote content 2",
				SHA:     "sha789",
				Size:    len("remote content 2"),
			},
		}
		mockRepo.On("GetAllFilesWithContent", mock.Anything).Return(remoteFiles, nil).Once()

		var buf bytes.Buffer
		err := PrintStatus(fileManager, mockRepo, tempDir, &buf)
		assert.NoError(t, err)

		output := buf.String()

		// Verify header
		assert.Contains(t, output, "Files Status (Local + Remote):")

		// Verify local-only file
		assert.Contains(t, output, "local1.txt")
		assert.Contains(t, output, "Local-only")

		// Verify file that exists in both places
		assert.Contains(t, output, "both.txt")
		assert.Contains(t, output, "Not synced") // Since LastSyncedRemoteSHA is empty

		// Verify remote-only files
		assert.Contains(t, output, "remote1.txt")
		assert.Contains(t, output, "Remote-only")
		assert.Contains(t, output, "remote2.txt")
		assert.Contains(t, output, "Remote-only")

		mockRepo.AssertExpectations(t)
	})

	t.Run("NoFilesMessage", func(t *testing.T) {
		// Create empty file manager
		emptyFileManager := storage.NewFileManager(tempDir + "_empty")

		// Mock empty remote files
		mockRepo.ExpectedCalls = nil
		mockRepo.Calls = nil
		mockRepo.On("GetAllFilesWithContent", mock.Anything).Return(map[string]*repository.RemoteFileInfo{}, nil).Once()

		var buf bytes.Buffer
		err := PrintStatus(emptyFileManager, mockRepo, tempDir+"_empty", &buf)
		assert.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "No files are currently tracked or available remotely.")

		mockRepo.AssertExpectations(t)
	})

	t.Run("RepositoryError", func(t *testing.T) {
		// Mock repository error
		mockRepo.ExpectedCalls = nil
		mockRepo.Calls = nil
		mockRepo.On("GetAllFilesWithContent", mock.Anything).Return(map[string]*repository.RemoteFileInfo{}, assert.AnError).Once()

		var buf bytes.Buffer
		err := PrintStatus(fileManager, mockRepo, tempDir, &buf)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get remote files")

		mockRepo.AssertExpectations(t)
	})
}

func TestDetermineFileStatus(t *testing.T) {
	t.Run("LocalOnly", func(t *testing.T) {
		file := &storage.FileInfo{
			Path: "/test/file.txt",
			Hash: "localhash123",
		}
		status := determineFileStatus(file, nil)
		assert.Equal(t, "Local-only", status)
	})

	t.Run("RemoteOnly", func(t *testing.T) {
		file := &storage.FileInfo{
			Path: "/test/file.txt",
			Hash: "", // No local hash means file doesn't exist locally
		}
		remoteFile := &repository.RemoteFileInfo{
			Path:    "file.txt",
			Content: "remote content",
			SHA:     "remotesha123",
			Size:    14,
		}
		status := determineFileStatus(file, remoteFile)
		assert.Equal(t, "Remote-only", status)
	})

	t.Run("DeletedLocallyWithRemote", func(t *testing.T) {
		file := &storage.FileInfo{
			Path:    "/test/file.txt",
			Hash:    "localhash123",
			Deleted: true,
		}
		remoteFile := &repository.RemoteFileInfo{
			Path:    "file.txt",
			Content: "remote content",
			SHA:     "remotesha123",
			Size:    14,
		}
		status := determineFileStatus(file, remoteFile)
		assert.Equal(t, "Deleted locally (needs remote deletion)", status)
	})

	t.Run("DeletedLocallyNoRemote", func(t *testing.T) {
		file := &storage.FileInfo{
			Path:    "/test/file.txt",
			Hash:    "localhash123",
			Deleted: true,
		}
		status := determineFileStatus(file, nil)
		assert.Equal(t, "Deleted locally", status)
	})

	t.Run("NotSynced", func(t *testing.T) {
		file := &storage.FileInfo{
			Path:                "/test/file.txt",
			Hash:                "localhash123",
			LastSyncedRemoteSHA: "", // Never synced
		}
		remoteFile := &repository.RemoteFileInfo{
			Path:    "file.txt",
			Content: "remote content",
			SHA:     "remotesha123",
			Size:    14,
		}
		status := determineFileStatus(file, remoteFile)
		assert.Equal(t, "Not synced", status)
	})

	t.Run("Synced", func(t *testing.T) {
		file := &storage.FileInfo{
			Path:                "/test/file.txt",
			Hash:                "localhash123",
			LastSyncedHash:      "localhash123", // Same as current hash
			LastSyncedRemoteSHA: "remotesha123", // Same as remote SHA
		}
		remoteFile := &repository.RemoteFileInfo{
			Path:    "file.txt",
			Content: "remote content",
			SHA:     "remotesha123", // Same as LastSyncedRemoteSHA
			Size:    14,
		}
		status := determineFileStatus(file, remoteFile)
		assert.Equal(t, "Synced", status)
	})

	t.Run("ModifiedLocally", func(t *testing.T) {
		file := &storage.FileInfo{
			Path:                "/test/file.txt",
			Hash:                "newhash123",   // Different from LastSyncedHash
			LastSyncedHash:      "oldhash123",   // Different from current hash
			LastSyncedRemoteSHA: "remotesha123", // Same as remote SHA
		}
		remoteFile := &repository.RemoteFileInfo{
			Path:    "file.txt",
			Content: "remote content",
			SHA:     "remotesha123", // Same as LastSyncedRemoteSHA
			Size:    14,
		}
		status := determineFileStatus(file, remoteFile)
		assert.Equal(t, "Modified locally", status)
	})

	t.Run("ModifiedInRepository", func(t *testing.T) {
		file := &storage.FileInfo{
			Path:                "/test/file.txt",
			Hash:                "localhash123", // Same as LastSyncedHash
			LastSyncedHash:      "localhash123", // Same as current hash
			LastSyncedRemoteSHA: "oldremotesha", // Different from remote SHA
		}
		remoteFile := &repository.RemoteFileInfo{
			Path:    "file.txt",
			Content: "new remote content",
			SHA:     "newremotesha123", // Different from LastSyncedRemoteSHA
			Size:    18,
		}
		status := determineFileStatus(file, remoteFile)
		assert.Equal(t, "Modified in repository", status)
	})

	t.Run("Conflict", func(t *testing.T) {
		file := &storage.FileInfo{
			Path:                "/test/file.txt",
			Hash:                "newhash123",   // Different from LastSyncedHash
			LastSyncedHash:      "oldhash123",   // Different from current hash
			LastSyncedRemoteSHA: "oldremotesha", // Different from remote SHA
		}
		remoteFile := &repository.RemoteFileInfo{
			Path:    "file.txt",
			Content: "new remote content",
			SHA:     "newremotesha123", // Different from LastSyncedRemoteSHA
			Size:    18,
		}
		status := determineFileStatus(file, remoteFile)
		assert.Equal(t, "Conflict", status)
	})
}
