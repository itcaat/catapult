package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds all configuration in a single structure
type Config struct {
	GitHub struct {
		ClientID string   `yaml:"clientid"`
		Scopes   []string `yaml:"scopes"`
		Token    string   `yaml:"token"`
	} `yaml:"github"`
	Storage struct {
		BaseDir   string `yaml:"basedir"`
		StatePath string `yaml:"statepath"`
	} `yaml:"storage"`
	Repository struct {
		Name string `yaml:"name"`
	} `yaml:"repository"`
	Issues IssueConfig `yaml:"issues"`
}

// IssueConfig holds configuration for GitHub issue management
type IssueConfig struct {
	Enabled                 bool          `yaml:"enabled"`
	Repository              string        `yaml:"repository"`
	AutoCreate              bool          `yaml:"auto_create"`
	AutoResolve             bool          `yaml:"auto_resolve"`
	IncludeFileNames        bool          `yaml:"include_file_names"`
	IncludeErrorDetails     bool          `yaml:"include_error_details"`
	IncludeSystemInfo       bool          `yaml:"include_system_info"`
	Labels                  []string      `yaml:"labels"`
	Assignees               []string      `yaml:"assignees"`
	MaxOpenIssues           int           `yaml:"max_open_issues"`
	ResolutionCheckInterval time.Duration `yaml:"resolution_check_interval"`
}

// Load loads config from ~/.catapult/config.yaml
func Load() (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	cfg := &Config{}
	configPath := filepath.Join(home, ".catapult", "config.yaml")

	// Try to load existing config
	if data, err := os.ReadFile(configPath); err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
	}

	// Set defaults if not set
	if cfg.GitHub.Scopes == nil || len(cfg.GitHub.Scopes) == 0 {
		cfg.GitHub.Scopes = []string{"repo"}
	}
	if cfg.Repository.Name == "" {
		cfg.Repository.Name = "catapult-folder"
	}
	if cfg.Storage.BaseDir == "" {
		cfg.Storage.BaseDir = filepath.Join(home, ".catapult", "files")
	}
	if cfg.Storage.StatePath == "" {
		cfg.Storage.StatePath = filepath.Join(home, ".catapult", "state.json")
	}

	// Set issue management defaults
	setIssueDefaults(&cfg.Issues)

	// Expand tilde paths if they exist
	cfg.Storage.BaseDir = expandTildePath(cfg.Storage.BaseDir, home)
	cfg.Storage.StatePath = expandTildePath(cfg.Storage.StatePath, home)

	return cfg, nil
}

// Save saves the complete config to ~/.catapult/config.yaml with secure permissions
func (c *Config) Save() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(home, ".catapult")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	configPath := filepath.Join(configDir, "config.yaml")
	// Use 0600 permissions to protect the token
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// EnsureUserConfig checks if ~/.catapult/config.yaml exists and creates it with default content if it doesn't
func EnsureUserConfig() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	// Default config content with token field and issues configuration
	defaultConfig := fmt.Sprintf(`github:
  clientid: "Ov23liVBxOiGZXrFZNB6"
  scopes:
    - repo
  token: ""

storage:
  basedir: "%s"
  statepath: "%s"

repository:
  name: "catapult-folder"

issues:
  enabled: true
  repository: "catapult-folder"
  auto_create: true
  auto_resolve: true
  include_file_names: true
  include_error_details: true
  include_system_info: false
  labels:
    - catapult
    - auto-generated
  assignees: []
  max_open_issues: 10
  resolution_check_interval: 5m`,
		filepath.Join(home, "Catapult"),
		filepath.Join(home, ".catapult", "state.json"))

	// Ensure ~/.catapult/config.yaml exists
	configDir := filepath.Join(home, ".catapult")
	configPath := filepath.Join(configDir, "config.yaml")

	// Check if config file already exists
	if _, err := os.Stat(configPath); err == nil {
		// File exists, nothing to do
		return nil
	} else if !os.IsNotExist(err) {
		// Some other error occurred
		return fmt.Errorf("failed to check config file: %w", err)
	}

	// File doesn't exist, create directory if needed
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write the default config file with secure permissions (0600 to protect token)
	if err := os.WriteFile(configPath, []byte(defaultConfig), 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Printf("Generated default config file: %s\n", configPath)
	return nil
}

// MigrateFromOldConfig migrates from the old two-file system to the new single-file system
func MigrateFromOldConfig() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(home, ".catapult")
	runtimePath := filepath.Join(configDir, "config.runtime.yaml")

	// Check if old runtime config exists
	if _, err := os.Stat(runtimePath); os.IsNotExist(err) {
		// No old config to migrate
		return nil
	}

	// Load current config
	cfg, err := Load()
	if err != nil {
		return fmt.Errorf("failed to load current config: %w", err)
	}

	// Read old runtime config
	type OldRuntimeConfig struct {
		GitHub struct {
			Token string `yaml:"token"`
		} `yaml:"github"`
		Storage struct {
			BaseDir   string `yaml:"basedir"`
			StatePath string `yaml:"statepath"`
		} `yaml:"storage"`
	}

	oldRuntimeCfg := &OldRuntimeConfig{}
	if data, err := os.ReadFile(runtimePath); err == nil {
		if err := yaml.Unmarshal(data, oldRuntimeCfg); err == nil {
			// Migrate token and storage settings if they exist
			if oldRuntimeCfg.GitHub.Token != "" {
				cfg.GitHub.Token = oldRuntimeCfg.GitHub.Token
			}
			if oldRuntimeCfg.Storage.BaseDir != "" {
				cfg.Storage.BaseDir = oldRuntimeCfg.Storage.BaseDir
			}
			if oldRuntimeCfg.Storage.StatePath != "" {
				cfg.Storage.StatePath = oldRuntimeCfg.Storage.StatePath
			}

			// Save migrated config
			if err := cfg.Save(); err != nil {
				return fmt.Errorf("failed to save migrated config: %w", err)
			}

			// Remove old runtime config file
			if err := os.Remove(runtimePath); err != nil {
				fmt.Printf("Warning: failed to remove old config file %s: %v\n", runtimePath, err)
			} else {
				fmt.Printf("Migrated configuration from old format and removed %s\n", runtimePath)
			}
		}
	}

	return nil
}

// expandTildePath expands ~ to the home directory in file paths
func expandTildePath(path, home string) string {
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}

// setIssueDefaults sets default values for issue management configuration
func setIssueDefaults(cfg *IssueConfig) {
	// Check if this is a completely uninitialized config (no repository set)
	isNewConfig := cfg.Repository == ""

	// Set basic defaults
	if cfg.Repository == "" {
		cfg.Repository = "catapult-folder"
	}
	if cfg.Labels == nil {
		cfg.Labels = []string{"catapult", "auto-generated"}
	}
	if cfg.MaxOpenIssues == 0 {
		cfg.MaxOpenIssues = 10
	}
	if cfg.ResolutionCheckInterval == 0 {
		cfg.ResolutionCheckInterval = 5 * time.Minute
	}

	// Set defaults for boolean fields for new configs or configs missing the issues section
	if isNewConfig {
		// This is a new config, set enabled by default
		cfg.Enabled = true
		cfg.AutoCreate = true
		cfg.AutoResolve = true
		cfg.IncludeFileNames = true
		cfg.IncludeErrorDetails = true
		cfg.IncludeSystemInfo = false // Privacy-conscious default
	}
}
