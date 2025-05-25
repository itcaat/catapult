package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// StaticConfig holds the static (base) configuration
// Only fields that do not change at runtime
// (clientid, scopes, repo name)
type StaticConfig struct {
	GitHub struct {
		ClientID string   `yaml:"clientid"`
		Scopes   []string `yaml:"scopes"`
	} `yaml:"github"`
	Repository struct {
		Name string `yaml:"name"`
	} `yaml:"repository"`
}

// RuntimeConfig holds dynamic fields (token, paths)
type RuntimeConfig struct {
	GitHub struct {
		Token string `yaml:"token"`
	} `yaml:"github"`
	Storage struct {
		BaseDir   string `yaml:"basedir"`
		StatePath string `yaml:"statepath"`
	} `yaml:"storage"`
}

// Config is the merged config for use in the app
// (not saved directly)
type Config struct {
	GitHub struct {
		ClientID string
		Scopes   []string
		Token    string
	}
	Storage struct {
		BaseDir   string
		StatePath string
	}
	Repository struct {
		Name string
	}
}

// Load loads config from ~/.catapult/config.yaml (static) and ~/.catapult/config.runtime.yaml (dynamic), merges them
func Load() (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	// 1. Load static config from ~/.catapult/config.yaml
	staticCfg := &StaticConfig{}
	staticPath := filepath.Join(home, ".catapult", "config.yaml")
	if data, err := os.ReadFile(staticPath); err == nil {
		yaml.Unmarshal(data, staticCfg)
	}
	// Defaults if not set
	if staticCfg.GitHub.Scopes == nil || len(staticCfg.GitHub.Scopes) == 0 {
		staticCfg.GitHub.Scopes = []string{"repo"}
	}
	if staticCfg.Repository.Name == "" {
		staticCfg.Repository.Name = "catapult-folder"
	}

	// 2. Load runtime config from home dir
	runtimeCfg := &RuntimeConfig{}
	runtimePath := filepath.Join(home, ".catapult", "config.runtime.yaml")
	if data, err := os.ReadFile(runtimePath); err == nil {
		yaml.Unmarshal(data, runtimeCfg)
	}
	// Set runtime defaults if not set
	if runtimeCfg.Storage.BaseDir == "" {
		runtimeCfg.Storage.BaseDir = filepath.Join(home, ".catapult", "files")
	}
	if runtimeCfg.Storage.StatePath == "" {
		runtimeCfg.Storage.StatePath = filepath.Join(home, ".catapult", "state.json")
	}

	// 3. Merge
	cfg := &Config{}
	cfg.GitHub.ClientID = staticCfg.GitHub.ClientID
	cfg.GitHub.Scopes = staticCfg.GitHub.Scopes
	cfg.GitHub.Token = runtimeCfg.GitHub.Token
	cfg.Repository.Name = staticCfg.Repository.Name
	cfg.Storage.BaseDir = runtimeCfg.Storage.BaseDir
	cfg.Storage.StatePath = runtimeCfg.Storage.StatePath

	return cfg, nil
}

// Save saves only the runtime config (token, paths) to ~/.catapult/config.runtime.yaml
func (c *Config) Save() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}
	configDir := filepath.Join(home, ".catapult")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	runtimeCfg := &RuntimeConfig{}
	runtimeCfg.GitHub.Token = c.GitHub.Token
	runtimeCfg.Storage.BaseDir = c.Storage.BaseDir
	runtimeCfg.Storage.StatePath = c.Storage.StatePath

	data, err := yaml.Marshal(runtimeCfg)
	if err != nil {
		return fmt.Errorf("failed to marshal runtime config: %w", err)
	}
	runtimePath := filepath.Join(configDir, "config.runtime.yaml")
	if err := os.WriteFile(runtimePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write runtime config: %w", err)
	}
	return nil
}

// EnsureUserConfig checks if ~/.catapult/config.yaml exists and creates it with default content if it doesn't
func EnsureUserConfig() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	// Default config content
	defaultConfig := `github:
  clientid: "Ov23liVBxOiGZXrFZNB6"
  scopes:
    - repo

storage:
  basedir: "~/Catapult"
  statepath: "~/.catapult/state.json"

repository:
  name: "catapult-folder"`

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

	// Write the default config file
	if err := os.WriteFile(configPath, []byte(defaultConfig), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Printf("Generated default config file: %s\n", configPath)
	return nil
}
