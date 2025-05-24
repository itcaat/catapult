package cmd

import (
	"github.com/spf13/cobra"
)

// NewRootCmd creates and returns the root command
func NewRootCmd(version, commit, date string) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "catapult",
		Short: "Catapult - GitHub file sync application",
		Long: `Catapult is a console application for file management and synchronization 
with GitHub using device flow authentication.`,
	}

	// Add subcommands
	rootCmd.AddCommand(NewVersionCmd(version, commit, date))
	rootCmd.AddCommand(NewInitCmd())
	rootCmd.AddCommand(NewSyncCmd())
	rootCmd.AddCommand(NewStatusCmd())

	return rootCmd
}
