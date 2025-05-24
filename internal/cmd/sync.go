package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/google/go-github/v57/github"
	"github.com/itcaat/catapult/internal/config"
	"github.com/itcaat/catapult/internal/repository"
	"github.com/itcaat/catapult/internal/storage"
	"github.com/itcaat/catapult/internal/sync"
	"github.com/spf13/cobra"
)

// NewSyncCmd creates and returns the sync command
func NewSyncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Sync files with GitHub",
		Long:  `Sync all files in the current directory with GitHub repository.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load configuration
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Ensure base directory exists
			if err := os.MkdirAll(cfg.Storage.BaseDir, 0755); err != nil {
				return fmt.Errorf("failed to create base directory: %w", err)
			}

			// Create file manager
			fileManager := storage.NewFileManager(cfg.Storage.BaseDir)

			// Load state if exists
			if err := fileManager.LoadState(cfg.Storage.StatePath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("failed to load state: %w", err)
			}

			// Scan directory for files
			if err := fileManager.ScanDirectory(); err != nil {
				return fmt.Errorf("failed to scan directory: %w", err)
			}

			// Create GitHub client
			client := github.NewClient(nil).WithAuthToken(cfg.GitHub.Token)

			// Get authenticated user
			user, _, err := client.Users.Get(context.Background(), "")
			if err != nil {
				return fmt.Errorf("failed to get user: %w", err)
			}

			// Create repository instance
			repo := repository.New(client, user.GetLogin(), cfg.Repository.Name)

			// Create sync instance
			syncer := sync.New(repo, fileManager)

			// Sync all files with progress output
			if err := syncer.SyncAll(context.Background(), os.Stdout); err != nil {
				return fmt.Errorf("failed to sync files: %w", err)
			}

			// Save state after sync
			if err := fileManager.SaveState(cfg.Storage.StatePath); err != nil {
				return fmt.Errorf("failed to save state: %w", err)
			}

			return nil
		},
	}
}
