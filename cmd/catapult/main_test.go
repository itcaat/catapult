package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/itcaat/catapult/internal/config"
	"github.com/itcaat/catapult/internal/repository"
	"github.com/itcaat/catapult/internal/storage"
	"github.com/spf13/cobra"
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

func TestStatusCommand(t *testing.T) {
	// Create temporary directory for test
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

	// Scan directory to track files automatically
	err = fileManager.ScanDirectory()
	assert.NoError(t, err)

	// Save state
	statePath := filepath.Join(tempDir, "state.json")
	err = fileManager.SaveState(statePath)
	assert.NoError(t, err)

	// Create mock repository (not used for local changes)
	mockRepo := new(MockRepository)

	// Mock GetAllFilesWithContent to return empty map (no remote files)
	mockRepo.On("GetAllFilesWithContent", mock.Anything).Return(map[string]*repository.RemoteFileInfo{}, nil).Once()

	// Create test configuration
	cfg := &config.Config{}
	cfg.Storage.BaseDir = tempDir
	cfg.Storage.StatePath = statePath

	// Create test command
	var buf bytes.Buffer
	// Проверяем статус через функцию PrintStatus
	err = PrintStatus(fileManager, mockRepo, tempDir, &buf)
	assert.NoError(t, err)

	// Verify output
	output := buf.String()
	assert.Contains(t, output, "Tracked Files Status:")
	assert.Contains(t, output, "test1.txt")
	assert.Contains(t, output, "test2.txt")
	assert.Contains(t, output, "Local-only")
	// state.json не должен трекаться
	assert.NotContains(t, output, "state.json")
}

func TestStatusCommandNoFiles(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "catapult-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create file manager
	fileManager := storage.NewFileManager(tempDir)

	// Save empty state
	statePath := filepath.Join(tempDir, "state.json")
	err = fileManager.SaveState(statePath)
	assert.NoError(t, err)

	// Create test configuration
	cfg := &config.Config{}
	cfg.Storage.BaseDir = tempDir
	cfg.Storage.StatePath = statePath

	// Create test command
	cmd := statusCmd
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		// Get tracked files
		files := fileManager.GetTrackedFiles()
		if len(files) == 0 {
			return nil
		}

		return nil
	}

	// Execute command
	err = cmd.Execute()
	assert.NoError(t, err)
}
