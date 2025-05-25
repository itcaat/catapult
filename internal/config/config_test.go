package config

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestLoad(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	configDir := filepath.Join(tempDir, ".catapult")
	configPath := filepath.Join(configDir, "config.yaml")

	// Test loading with no config file (should use defaults)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Check defaults
	if len(cfg.GitHub.Scopes) != 1 || cfg.GitHub.Scopes[0] != "repo" {
		t.Errorf("Expected default scopes [repo], got %v", cfg.GitHub.Scopes)
	}
	if cfg.Repository.Name != "catapult-folder" {
		t.Errorf("Expected default repository name 'catapult-folder', got %s", cfg.Repository.Name)
	}
	expectedBaseDir := filepath.Join(tempDir, ".catapult", "files")
	if cfg.Storage.BaseDir != expectedBaseDir {
		t.Errorf("Expected default base dir %s, got %s", expectedBaseDir, cfg.Storage.BaseDir)
	}

	// Test loading with existing config file
	testConfig := `github:
  clientid: "test-client-id"
  scopes:
    - repo
    - user
  token: "test-token"
storage:
  basedir: "/custom/path"
  statepath: "/custom/state.json"
repository:
  name: "test-repo"`

	os.MkdirAll(configDir, 0755)
	if err := os.WriteFile(configPath, []byte(testConfig), 0600); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load() failed with existing config: %v", err)
	}

	// Check loaded values
	if cfg.GitHub.ClientID != "test-client-id" {
		t.Errorf("Expected client ID 'test-client-id', got %s", cfg.GitHub.ClientID)
	}
	if cfg.GitHub.Token != "test-token" {
		t.Errorf("Expected token 'test-token', got %s", cfg.GitHub.Token)
	}
	if len(cfg.GitHub.Scopes) != 2 || cfg.GitHub.Scopes[1] != "user" {
		t.Errorf("Expected scopes [repo, user], got %v", cfg.GitHub.Scopes)
	}
	if cfg.Storage.BaseDir != "/custom/path" {
		t.Errorf("Expected base dir '/custom/path', got %s", cfg.Storage.BaseDir)
	}
	if cfg.Repository.Name != "test-repo" {
		t.Errorf("Expected repository name 'test-repo', got %s", cfg.Repository.Name)
	}
}

func TestSave(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	cfg := &Config{
		GitHub: struct {
			ClientID string   `yaml:"clientid"`
			Scopes   []string `yaml:"scopes"`
			Token    string   `yaml:"token"`
		}{
			ClientID: "test-client",
			Scopes:   []string{"repo", "user"},
			Token:    "secret-token",
		},
		Storage: struct {
			BaseDir   string `yaml:"basedir"`
			StatePath string `yaml:"statepath"`
		}{
			BaseDir:   "/test/path",
			StatePath: "/test/state.json",
		},
		Repository: struct {
			Name string `yaml:"name"`
		}{
			Name: "test-repo",
		},
	}

	// Save config
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Check file was created with correct permissions
	configPath := filepath.Join(tempDir, ".catapult", "config.yaml")
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("Config file not created: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("Expected file permissions 0600, got %o", info.Mode().Perm())
	}

	// Load and verify content
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read saved config: %v", err)
	}

	var savedCfg Config
	if err := yaml.Unmarshal(data, &savedCfg); err != nil {
		t.Fatalf("Failed to parse saved config: %v", err)
	}

	if savedCfg.GitHub.Token != "secret-token" {
		t.Errorf("Expected saved token 'secret-token', got %s", savedCfg.GitHub.Token)
	}
	if savedCfg.GitHub.ClientID != "test-client" {
		t.Errorf("Expected saved client ID 'test-client', got %s", savedCfg.GitHub.ClientID)
	}
}

func TestEnsureUserConfig(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	configPath := filepath.Join(tempDir, ".catapult", "config.yaml")

	// Test creating new config
	if err := EnsureUserConfig(); err != nil {
		t.Fatalf("EnsureUserConfig() failed: %v", err)
	}

	// Check file was created
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("Config file not created: %v", err)
	}

	// Check file permissions
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("Failed to stat config file: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("Expected file permissions 0600, got %o", info.Mode().Perm())
	}

	// Check content includes token field
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Failed to parse config file: %v", err)
	}

	// Token should be empty string (not nil)
	if cfg.GitHub.Token != "" {
		t.Errorf("Expected empty token, got %s", cfg.GitHub.Token)
	}
	if cfg.GitHub.ClientID == "" {
		t.Error("Expected client ID to be set in default config")
	}

	// Test that existing config is not overwritten
	originalContent := string(data)
	if err := EnsureUserConfig(); err != nil {
		t.Fatalf("EnsureUserConfig() failed on second call: %v", err)
	}

	newData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config file after second call: %v", err)
	}

	if string(newData) != originalContent {
		t.Error("Config file was modified on second call to EnsureUserConfig()")
	}
}

func TestMigrateFromOldConfig(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	configDir := filepath.Join(tempDir, ".catapult")
	os.MkdirAll(configDir, 0755)

	// Create old config.yaml (static)
	staticConfig := `github:
  clientid: "old-client-id"
  scopes:
    - repo
repository:
  name: "old-repo"`

	configPath := filepath.Join(configDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(staticConfig), 0644); err != nil {
		t.Fatalf("Failed to write static config: %v", err)
	}

	// Create old config.runtime.yaml
	runtimeConfig := `github:
  token: "old-token"
storage:
  basedir: "/old/path"
  statepath: "/old/state.json"`

	runtimePath := filepath.Join(configDir, "config.runtime.yaml")
	if err := os.WriteFile(runtimePath, []byte(runtimeConfig), 0644); err != nil {
		t.Fatalf("Failed to write runtime config: %v", err)
	}

	// Run migration
	if err := MigrateFromOldConfig(); err != nil {
		t.Fatalf("MigrateFromOldConfig() failed: %v", err)
	}

	// Check that runtime config was removed
	if _, err := os.Stat(runtimePath); !os.IsNotExist(err) {
		t.Error("Old runtime config file was not removed")
	}

	// Load migrated config
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Failed to load migrated config: %v", err)
	}

	// Check that values were migrated correctly
	if cfg.GitHub.Token != "old-token" {
		t.Errorf("Expected migrated token 'old-token', got %s", cfg.GitHub.Token)
	}
	if cfg.Storage.BaseDir != "/old/path" {
		t.Errorf("Expected migrated base dir '/old/path', got %s", cfg.Storage.BaseDir)
	}
	if cfg.Storage.StatePath != "/old/state.json" {
		t.Errorf("Expected migrated state path '/old/state.json', got %s", cfg.Storage.StatePath)
	}
	// Static values should still be there
	if cfg.GitHub.ClientID != "old-client-id" {
		t.Errorf("Expected client ID 'old-client-id', got %s", cfg.GitHub.ClientID)
	}
}

func TestMigrateFromOldConfigNoOldFile(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	// Run migration with no old files
	if err := MigrateFromOldConfig(); err != nil {
		t.Fatalf("MigrateFromOldConfig() failed with no old files: %v", err)
	}

	// Should not create any files or cause errors
	configDir := filepath.Join(tempDir, ".catapult")
	if _, err := os.Stat(configDir); !os.IsNotExist(err) {
		t.Error("Config directory was created when it shouldn't have been")
	}
}

func TestTildePathExpansion(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	configDir := filepath.Join(tempDir, ".catapult")
	configPath := filepath.Join(configDir, "config.yaml")

	// Create config with tilde paths (simulating manual editing)
	configWithTildes := `github:
  clientid: "test-client-id"
  scopes:
    - repo
  token: "test-token"
storage:
  basedir: "~/CustomFolder"
  statepath: "~/.catapult/custom-state.json"
repository:
  name: "test-repo"`

	os.MkdirAll(configDir, 0755)
	if err := os.WriteFile(configPath, []byte(configWithTildes), 0600); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Load config - should expand tilde paths
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Check that tilde paths were expanded
	expectedBaseDir := filepath.Join(tempDir, "CustomFolder")
	if cfg.Storage.BaseDir != expectedBaseDir {
		t.Errorf("Expected expanded base dir %s, got %s", expectedBaseDir, cfg.Storage.BaseDir)
	}

	expectedStatePath := filepath.Join(tempDir, ".catapult", "custom-state.json")
	if cfg.Storage.StatePath != expectedStatePath {
		t.Errorf("Expected expanded state path %s, got %s", expectedStatePath, cfg.Storage.StatePath)
	}

	// Other fields should remain unchanged
	if cfg.GitHub.ClientID != "test-client-id" {
		t.Errorf("Expected client ID 'test-client-id', got %s", cfg.GitHub.ClientID)
	}
	if cfg.GitHub.Token != "test-token" {
		t.Errorf("Expected token 'test-token', got %s", cfg.GitHub.Token)
	}
}
