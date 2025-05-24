package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewVersionCmd creates and returns the version command
func NewVersionCmd(version, commit, date string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Long:  `Display version, commit hash, and build date information.`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Catapult %s\n", version)
			fmt.Printf("Commit: %s\n", commit)
			fmt.Printf("Built: %s\n", date)
		},
	}
}
