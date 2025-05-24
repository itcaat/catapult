package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/itcaat/catapult/internal/service"
	"github.com/spf13/cobra"
)

// NewServiceCmd creates and returns the service management command
func NewServiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service",
		Short: "Manage system autostart service",
		Long: `Install, uninstall, and manage catapult as system service for automatic startup.

This command allows you to configure catapult to start automatically when your system boots.
The service will run 'catapult sync --watch' in the background.

Supported platforms:
- macOS: Uses LaunchAgent 
- Linux: Uses systemd user service
- Windows: Not implemented yet`,
	}

	cmd.AddCommand(NewServiceInstallCmd())
	cmd.AddCommand(NewServiceUninstallCmd())
	cmd.AddCommand(NewServiceStartCmd())
	cmd.AddCommand(NewServiceStopCmd())
	cmd.AddCommand(NewServiceRestartCmd())
	cmd.AddCommand(NewServiceStatusCmd())
	cmd.AddCommand(NewServiceLogsCmd())

	return cmd
}

// NewServiceInstallCmd creates the service install command
func NewServiceInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Install catapult as system service",
		Long: `Install catapult as system service for automatic startup.

This will create a system service that starts catapult automatically when you log in.
The service will run 'catapult sync --watch' to monitor file changes.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, err := createServiceManager()
			if err != nil {
				return err
			}

			fmt.Printf("Installing catapult as system service on %s...\n", runtime.GOOS)

			if manager.IsInstalled() {
				return fmt.Errorf("service is already installed. Use 'catapult service uninstall' first")
			}

			if err := manager.Install(); err != nil {
				return fmt.Errorf("failed to install service: %w", err)
			}

			fmt.Println("✅ Service installed successfully")
			fmt.Println("💡 Catapult will now start automatically on system boot")
			fmt.Println("🔧 Use 'catapult service status' to check service status")
			fmt.Println("📋 Use 'catapult service logs' to view service logs")

			return nil
		},
	}
}

// NewServiceUninstallCmd creates the service uninstall command
func NewServiceUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Remove catapult system service",
		Long:  `Remove catapult system service and disable automatic startup.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, err := createServiceManager()
			if err != nil {
				return err
			}

			if !manager.IsInstalled() {
				fmt.Println("⚠️  Service is not installed")
				return nil
			}

			fmt.Println("Removing catapult system service...")

			// Stop service first
			if manager.IsRunning() {
				fmt.Println("Stopping service...")
				if err := manager.Stop(); err != nil {
					fmt.Printf("⚠️  Warning: failed to stop service: %v\n", err)
				}
			}

			// Uninstall
			if err := manager.Uninstall(); err != nil {
				return fmt.Errorf("failed to uninstall service: %w", err)
			}

			fmt.Println("✅ Service uninstalled successfully")
			fmt.Println("💡 Catapult will no longer start automatically")

			return nil
		},
	}
}

// NewServiceStartCmd creates the service start command
func NewServiceStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start catapult service",
		Long:  `Start the catapult system service manually.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, err := createServiceManager()
			if err != nil {
				return err
			}

			if !manager.IsInstalled() {
				return fmt.Errorf("service is not installed. Use 'catapult service install' first")
			}

			if manager.IsRunning() {
				fmt.Println("✅ Service is already running")
				return nil
			}

			fmt.Println("Starting catapult service...")
			if err := manager.Start(); err != nil {
				return fmt.Errorf("failed to start service: %w", err)
			}

			fmt.Println("✅ Service started successfully")
			return nil
		},
	}
}

// NewServiceStopCmd creates the service stop command
func NewServiceStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop catapult service",
		Long:  `Stop the catapult system service.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, err := createServiceManager()
			if err != nil {
				return err
			}

			if !manager.IsInstalled() {
				return fmt.Errorf("service is not installed")
			}

			if !manager.IsRunning() {
				fmt.Println("⚠️  Service is not running")
				return nil
			}

			fmt.Println("Stopping catapult service...")
			if err := manager.Stop(); err != nil {
				return fmt.Errorf("failed to stop service: %w", err)
			}

			fmt.Println("✅ Service stopped successfully")
			return nil
		},
	}
}

// NewServiceRestartCmd creates the service restart command
func NewServiceRestartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restart",
		Short: "Restart catapult service",
		Long:  `Restart the catapult system service.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, err := createServiceManager()
			if err != nil {
				return err
			}

			if !manager.IsInstalled() {
				return fmt.Errorf("service is not installed")
			}

			fmt.Println("Restarting catapult service...")
			if err := manager.Restart(); err != nil {
				return fmt.Errorf("failed to restart service: %w", err)
			}

			fmt.Println("✅ Service restarted successfully")
			return nil
		},
	}
}

// NewServiceStatusCmd creates the service status command
func NewServiceStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check catapult service status",
		Long:  `Check the current status of the catapult system service.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, err := createServiceManager()
			if err != nil {
				return err
			}

			status, err := manager.Status()
			if err != nil {
				return fmt.Errorf("failed to get service status: %w", err)
			}

			fmt.Printf("Service Status: %s\n", status)

			switch status {
			case service.StatusRunning:
				fmt.Println("✅ Service is running and monitoring file changes")
			case service.StatusStopped:
				fmt.Println("⏸️  Service is installed but not running")
			case service.StatusNotInstalled:
				fmt.Println("❌ Service is not installed")
				fmt.Println("💡 Use 'catapult service install' to install the service")
			default:
				fmt.Println("❓ Service status is unknown")
			}

			return nil
		},
	}
}

// NewServiceLogsCmd creates the service logs command
func NewServiceLogsCmd() *cobra.Command {
	var lines int

	cmd := &cobra.Command{
		Use:   "logs",
		Short: "View catapult service logs",
		Long:  `View the recent logs from the catapult system service.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, err := createServiceManager()
			if err != nil {
				return err
			}

			if !manager.IsInstalled() {
				return fmt.Errorf("service is not installed")
			}

			logLines, err := manager.GetLogs(lines)
			if err != nil {
				return fmt.Errorf("failed to get service logs: %w", err)
			}

			if len(logLines) == 0 {
				fmt.Println("No logs available")
				return nil
			}

			fmt.Printf("Last %d lines from service logs:\n\n", len(logLines))
			for _, line := range logLines {
				fmt.Println(line)
			}

			return nil
		},
	}

	cmd.Flags().IntVarP(&lines, "lines", "n", 20, "Number of lines to show")

	return cmd
}

// createServiceManager creates a service manager instance
func createServiceManager() (service.ServiceManager, error) {
	// Get current executable path
	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}

	// Resolve symlinks
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve executable path: %w", err)
	}

	// Create service config
	config := service.DefaultConfig(executable)

	// Create service manager
	manager, err := service.NewServiceManager(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create service manager: %w", err)
	}

	return manager, nil
}
