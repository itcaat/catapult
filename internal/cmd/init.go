package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/google/go-github/v57/github"
	"github.com/itcaat/catapult/internal/auth"
	"github.com/itcaat/catapult/internal/config"
	"github.com/itcaat/catapult/internal/repository"
	"github.com/itcaat/catapult/internal/storage"
	"github.com/spf13/cobra"
)

// NewInitCmd creates and returns the init command
func NewInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize Catapult",
		Long:  `Initialize Catapult by setting up authentication and repository.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Ensure ~/.catapult/config.yaml exists
			if err := config.EnsureUserConfig(); err != nil {
				return fmt.Errorf("failed to ensure user config: %w", err)
			}

			// Migrate from old two-file config system if needed
			if err := config.MigrateFromOldConfig(); err != nil {
				return fmt.Errorf("failed to migrate old config: %w", err)
			}

			// Load configuration
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Create device flow
			deviceFlow := auth.NewDeviceFlow(&auth.Config{
				ClientID: cfg.GitHub.ClientID,
				Scopes:   cfg.GitHub.Scopes,
			})

			// Initiate authentication
			token, err := deviceFlow.Initiate()
			if err != nil {
				return fmt.Errorf("failed to authenticate: %w", err)
			}

			// Save token in configuration
			cfg.GitHub.Token = token.AccessToken
			if err := cfg.Save(); err != nil {
				return fmt.Errorf("failed to save token: %w", err)
			}

			// Create GitHub client
			fmt.Printf("Using token: %s\n", cfg.GitHub.Token)
			client := github.NewClient(nil).WithAuthToken(token.AccessToken)

			// Get authenticated user
			user, _, err := client.Users.Get(context.Background(), "")
			if err != nil {
				return fmt.Errorf("failed to get user: %w", err)
			}

			// Create repository instance
			repo := repository.New(client, user.GetLogin(), cfg.Repository.Name)

			// Ensure repository exists
			if err := repo.EnsureExists(context.Background()); err != nil {
				return fmt.Errorf("failed to ensure repository exists: %w", err)
			}

			// Create storage directories
			if err := os.MkdirAll(cfg.Storage.BaseDir, 0755); err != nil {
				return fmt.Errorf("failed to create base directory: %w", err)
			}

			// Initialize file manager
			fileManager := storage.NewFileManager(cfg.Storage.BaseDir)

			// Save initial state
			if err := fileManager.SaveState(cfg.Storage.StatePath); err != nil {
				return fmt.Errorf("failed to save initial state: %w", err)
			}

			fmt.Printf("Successfully initialized Catapult with repository: %s\n", cfg.Repository.Name)
			return nil
		},
	}
}
