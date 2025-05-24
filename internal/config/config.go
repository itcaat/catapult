package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds the application configuration
type Config struct {
	GitHub struct {
		ClientID string   `mapstructure:"clientid"`
		Scopes   []string `mapstructure:"scopes"`
		Token    string   `mapstructure:"token"`
	} `mapstructure:"github"`
	Storage struct {
		TokenPath string `mapstructure:"tokenpath"`
		BaseDir   string `mapstructure:"basedir"`
		StatePath string `mapstructure:"statepath"`
	} `mapstructure:"storage"`
	Repository struct {
		Name string `mapstructure:"name"`
	} `mapstructure:"repository"`
}

// Load loads the configuration from the config file
func Load() (*Config, error) {
	// Get home directory
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	// Set default values
	cfg := &Config{}
	cfg.GitHub.Scopes = []string{"repo"}
	cfg.Repository.Name = "catapult-folder"
	cfg.Storage.BaseDir = filepath.Join(home, ".catapult", "files")
	cfg.Storage.StatePath = filepath.Join(home, ".catapult", "state.json")

	// Try to load config from current directory first
	currentDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}

	// Try local config first
	localConfigPath := filepath.Join(currentDir, "config.yaml")
	data, err := os.ReadFile(localConfigPath)
	if err == nil {
		// Local config exists, unmarshal it
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("failed to unmarshal local config: %w", err)
		}
		return cfg, nil
	}

	// If local config doesn't exist, try home directory
	configDir := filepath.Join(home, ".catapult")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	// Read config file from home directory
	configPath := filepath.Join(configDir, "config.yaml")
	data, err = os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Create default config file
			if err := cfg.Save(); err != nil {
				return nil, fmt.Errorf("failed to create default config: %w", err)
			}
			return cfg, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Unmarshal config
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return cfg, nil
}

// Save saves the configuration to the config file
func (c *Config) Save() error {
	// Get home directory
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	// Create config directory if it doesn't exist
	configDir := filepath.Join(home, ".catapult")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal config
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write config file
	configPath := filepath.Join(configDir, "config.yaml")
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
