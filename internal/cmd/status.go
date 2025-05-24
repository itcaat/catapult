package cmd

import (
	"context"
	"fmt"

	"github.com/google/go-github/v57/github"
	"github.com/itcaat/catapult/internal/config"
	"github.com/itcaat/catapult/internal/repository"
	"github.com/itcaat/catapult/internal/status"
	"github.com/itcaat/catapult/internal/storage"
	"github.com/spf13/cobra"
)

// NewStatusCmd creates and returns the status command
func NewStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show status of tracked files",
		Long:  `Show status of all files and their synchronization state with GitHub.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			fileManager := storage.NewFileManager(cfg.Storage.BaseDir)
			if err := fileManager.LoadState(cfg.Storage.StatePath); err != nil {
				return fmt.Errorf("failed to load state: %w", err)
			}

			client := github.NewClient(nil).WithAuthToken(cfg.GitHub.Token)
			user, _, err := client.Users.Get(context.Background(), "")
			if err != nil {
				return fmt.Errorf("failed to get user: %w", err)
			}

			repo := repository.New(client, user.GetLogin(), cfg.Repository.Name)
			return status.PrintStatus(fileManager, repo, cfg.Storage.BaseDir, cmd.OutOrStdout())
		},
	}
}
