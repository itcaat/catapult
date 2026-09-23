package cmd

import (
	"log/slog"
	"os"

	"github.com/itcaat/catapult/internal/logging"
	"github.com/spf13/cobra"
)

// NewRootCmd creates and returns the root command
func NewRootCmd(version, commit, date string) *cobra.Command {
	var logLevel string
	rootCmd := &cobra.Command{
		Use:   "catapult",
		Short: "Catapult - GitHub file sync application",
		Long: `Catapult is a console application for file management and synchronization 
with GitHub using device flow authentication.`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			level, err := logging.ParseLevel(logLevel)
			if err != nil {
				return err
			}
			slog.SetDefault(logging.New(os.Stderr, level).Logger)
			return nil
		},
	}
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "Log level: debug, info, warn, or error")

	// Add subcommands
	rootCmd.AddCommand(NewVersionCmd(version, commit, date))
	rootCmd.AddCommand(NewInitCmd())
	rootCmd.AddCommand(NewSyncCmd())
	rootCmd.AddCommand(NewStatusCmd())
	rootCmd.AddCommand(NewServiceCmd())
	rootCmd.AddCommand(NewOpenCmd())
	rootCmd.AddCommand(NewIssuesCmd())

	return rootCmd
}
