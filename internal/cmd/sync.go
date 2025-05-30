package cmd

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/google/go-github/v57/github"
	"github.com/itcaat/catapult/internal/autosync"
	"github.com/itcaat/catapult/internal/config"
	"github.com/itcaat/catapult/internal/issues"
	"github.com/itcaat/catapult/internal/repository"
	"github.com/itcaat/catapult/internal/storage"
	"github.com/itcaat/catapult/internal/sync"
	"github.com/spf13/cobra"
)

// NewSyncCmd creates and returns the sync command
func NewSyncCmd() *cobra.Command {
	var watchMode bool

	cmd := &cobra.Command{
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

			// Create sync instance with issue management if enabled
			var syncer *sync.Syncer
			if cfg.Issues.Enabled {
				// Create logger for issue management
				logger := log.New(os.Stdout, "[ISSUES] ", log.LstdFlags)

				// Create issue manager
				issueManager, err := issues.NewManager(client, user.GetLogin(), &cfg.Issues, logger)
				if err != nil {
					fmt.Printf("⚠️  Warning: Failed to initialize issue management: %v\n", err)
					fmt.Println("💡 Continuing without automatic issue creation")
					syncer = sync.New(repo, fileManager)
				} else {
					syncer = sync.NewWithIssueManager(repo, fileManager, issueManager, logger)
					fmt.Println("🎯 Issue management enabled - sync problems will create GitHub issues")
				}
			} else {
				syncer = sync.New(repo, fileManager)
			}

			// If watch mode is enabled, start auto-sync
			if watchMode {
				fmt.Println("🔄 Starting auto-sync with file watching...")
				fmt.Println("💡 Files will be automatically synced when changed")
				fmt.Println("⚡ Press Ctrl+C to stop watching")

				// Create logger for auto-sync
				logger := log.New(os.Stdout, "[AUTO-SYNC] ", log.LstdFlags)

				// Create auto-sync manager
				manager, err := autosync.NewManager(cfg, syncer, fileManager, repo, logger)
				if err != nil {
					return fmt.Errorf("failed to create auto-sync manager: %w", err)
				}

				// Start auto-sync (blocks until Ctrl+C)
				return manager.Start(context.Background())
			}

			// Sync all files with progress output (one-time sync)
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

	// Add --watch flag
	cmd.Flags().BoolVarP(&watchMode, "watch", "w", false, "Watch for file changes and sync automatically")

	return cmd
}
