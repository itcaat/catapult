package cmd

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/itcaat/catapult/internal/config"
	"github.com/spf13/cobra"
)

// NewOpenCmd creates and returns the open command
func NewOpenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "open",
		Short: "Open catapult folder in file manager",
		Long:  `Open the catapult folder in the default file manager (Finder on macOS, File Explorer on Windows, etc.).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load configuration to get the catapult folder path
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Get the catapult folder path
			catapultPath := cfg.Storage.BaseDir

			// Open the folder using platform-specific command
			var openCmd *exec.Cmd
			switch runtime.GOOS {
			case "darwin":
				// macOS - use 'open' command
				openCmd = exec.Command("open", catapultPath)
			case "windows":
				// Windows - use 'explorer' command
				openCmd = exec.Command("explorer", catapultPath)
			case "linux":
				// Linux - try common file managers
				openCmd = exec.Command("xdg-open", catapultPath)
			default:
				return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
			}

			// Execute the command
			if err := openCmd.Run(); err != nil {
				return fmt.Errorf("failed to open folder: %w", err)
			}

			fmt.Printf("Opened catapult folder: %s\n", catapultPath)
			return nil
		},
	}
}
