package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOpenCmd(t *testing.T) {
	// Create a temporary config for testing
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".catapult")
	configFile := filepath.Join(configDir, "config.yaml")

	// Create config directory
	err := os.MkdirAll(configDir, 0755)
	require.NoError(t, err)

	// Create a test config file
	configContent := `github:
  clientid: "test-client-id"
  scopes:
    - repo
  token: "test-token"
storage:
  basedir: "` + filepath.Join(tempDir, "test-catapult") + `"
  statepath: "` + filepath.Join(tempDir, ".catapult", "state.json") + `"
repository:
  name: "test-repo"
`
	err = os.WriteFile(configFile, []byte(configContent), 0600)
	require.NoError(t, err)

	// Set HOME to temp directory for testing
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tempDir)

	// Create the command
	cmd := NewOpenCmd()

	// Test command properties
	assert.Equal(t, "open", cmd.Use)
	assert.Equal(t, "Open catapult folder in file manager", cmd.Short)
	assert.Contains(t, cmd.Long, "Open the catapult folder in the default file manager")

	// Test that the command has a RunE function
	assert.NotNil(t, cmd.RunE)

	// Note: We don't actually execute the command in tests since it would
	// try to open a file manager, which isn't appropriate for automated tests.
	// In a real-world scenario, you might want to mock the exec.Command
	// or test the command logic separately from the file manager opening.
}
