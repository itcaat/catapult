package cmd

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/go-github/v57/github"
	"github.com/spf13/cobra"

	"github.com/itcaat/catapult/internal/config"
	"github.com/itcaat/catapult/internal/issues"
	"github.com/itcaat/catapult/internal/logging"
)

// NewIssuesCmd creates the issues command
func NewIssuesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "issues",
		Short: "Manage GitHub issues for sync problems",
		Long:  `View and manage automatically created GitHub issues for synchronization problems.`,
	}

	cmd.AddCommand(NewIssuesListCmd())
	cmd.AddCommand(NewIssuesEnableCmd())
	cmd.AddCommand(NewIssuesDisableCmd())

	return cmd
}

// NewIssuesListCmd creates the issues list command
func NewIssuesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List open sync issues",
		Long:  `List all open GitHub issues that were automatically created for synchronization problems.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			if !cfg.Issues.Enabled {
				fmt.Println("❌ Issue management is disabled")
				fmt.Println("💡 Use 'catapult issues enable' to enable automatic issue creation")
				return nil
			}

			// Check if user is authenticated
			if cfg.GitHub.Token == "" {
				fmt.Println("❌ Not authenticated with GitHub")
				fmt.Println("💡 Run 'catapult init' to authenticate first")
				return nil
			}

			// Create GitHub client
			client := github.NewClient(nil).WithAuthToken(cfg.GitHub.Token)

			// Get user info to determine owner
			user, _, err := client.Users.Get(context.Background(), "")
			if err != nil {
				return fmt.Errorf("failed to get user info: %w", err)
			}

			// Create issue manager
			logger := logging.From(slog.Default())
			manager, err := issues.NewManager(client, user.GetLogin(), &cfg.Issues, logger)
			if err != nil {
				return fmt.Errorf("failed to create issue manager: %w", err)
			}

			// Get open issues
			openIssues, err := manager.GetOpenIssues(context.Background())
			if err != nil {
				return fmt.Errorf("failed to get open issues: %w", err)
			}

			if len(openIssues) == 0 {
				fmt.Println("✅ No open sync issues")
				fmt.Printf("💡 Issues are automatically created in the '%s' repository when sync problems occur\n", cfg.Issues.Repository)
				return nil
			}

			fmt.Printf("📋 Open Sync Issues (%d):\n\n", len(openIssues))
			for _, issue := range openIssues {
				// Extract category from labels
				category := "unknown"
				for _, label := range issue.Labels {
					if label != "catapult" && label != "auto-generated" {
						category = label
						break
					}
				}

				fmt.Printf("🔗 #%d: %s\n", issue.Number, issue.Title)
				fmt.Printf("   Category: %s | Created: %s\n",
					category, issue.CreatedAt.Format("2006-01-02 15:04"))
				fmt.Printf("   URL: %s\n\n", issue.HTMLURL)
			}

			fmt.Printf("💡 Issues are automatically resolved when sync problems are fixed\n")
			fmt.Printf("🔧 Use 'catapult issues disable' to turn off automatic issue creation\n")

			return nil
		},
	}
}

// NewIssuesEnableCmd creates the issues enable command
func NewIssuesEnableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "enable",
		Short: "Enable automatic issue creation",
		Long:  `Enable automatic creation of GitHub issues when synchronization problems occur.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			if cfg.Issues.Enabled {
				fmt.Println("✅ Automatic issue creation is already enabled")
				fmt.Printf("💡 Issues will be created in the '%s' repository\n", cfg.Issues.Repository)
				fmt.Println("🔧 Use 'catapult issues list' to view open issues")
				return nil
			}

			// Enable issue management
			cfg.Issues.Enabled = true
			if err := cfg.Save(); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			fmt.Println("✅ Automatic issue creation enabled")
			fmt.Printf("💡 Issues will be created in the '%s' repository when sync problems occur\n", cfg.Issues.Repository)
			fmt.Println("🔧 Use 'catapult issues list' to view open issues")
			fmt.Println("🔧 Use 'catapult issues disable' to turn off automatic issue creation")

			return nil
		},
	}
}

// NewIssuesDisableCmd creates the issues disable command
func NewIssuesDisableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "disable",
		Short: "Disable automatic issue creation",
		Long:  `Disable automatic creation of GitHub issues when synchronization problems occur.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			if !cfg.Issues.Enabled {
				fmt.Println("❌ Automatic issue creation is already disabled")
				fmt.Println("💡 Use 'catapult issues enable' to enable automatic issue creation")
				return nil
			}

			// Disable issue management
			cfg.Issues.Enabled = false
			if err := cfg.Save(); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			fmt.Println("❌ Automatic issue creation disabled")
			fmt.Println("💡 Sync problems will no longer create GitHub issues")
			fmt.Println("🔧 Use 'catapult issues enable' to re-enable automatic issue creation")

			return nil
		},
	}
}
