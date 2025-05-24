package service

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
)

// LinuxSystemdService implements ServiceManager for Linux using systemd user services
type LinuxSystemdService struct {
	config   *ServiceConfig
	unitPath string
	unitName string
}

// NewLinuxSystemdService creates a new Linux systemd service manager
func NewLinuxSystemdService(config *ServiceConfig) *LinuxSystemdService {
	currentUser, _ := user.Current()
	unitName := config.Name + ".service"
	unitPath := filepath.Join(currentUser.HomeDir, ".config", "systemd", "user", unitName)

	// Set default log path if not provided
	if config.LogPath == "" {
		config.LogPath = filepath.Join(currentUser.HomeDir, ".local", "share", config.Name, "logs", config.Name+".log")
	}

	// Set default working directory if not provided
	if config.WorkingDir == "" {
		config.WorkingDir = currentUser.HomeDir
	}

	return &LinuxSystemdService{
		config:   config,
		unitPath: unitPath,
		unitName: unitName,
	}
}

// Install installs the service as a systemd user service
func (l *LinuxSystemdService) Install() error {
	// Create systemd user directory if it doesn't exist
	systemdUserDir := filepath.Dir(l.unitPath)
	if err := os.MkdirAll(systemdUserDir, 0755); err != nil {
		return fmt.Errorf("failed to create systemd user directory: %w", err)
	}

	// Create log directory if it doesn't exist
	logDir := filepath.Dir(l.config.LogPath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Create unit file content
	unitContent := l.generateUnitContent()

	// Write unit file
	if err := os.WriteFile(l.unitPath, []byte(unitContent), 0644); err != nil {
		return fmt.Errorf("failed to write unit file: %w", err)
	}

	// Reload systemd daemon
	if err := l.daemonReload(); err != nil {
		// Try to clean up on failure
		os.Remove(l.unitPath)
		return fmt.Errorf("failed to reload systemd daemon: %w", err)
	}

	// Enable the service
	if err := l.enable(); err != nil {
		// Try to clean up on failure
		os.Remove(l.unitPath)
		return fmt.Errorf("failed to enable service: %w", err)
	}

	return nil
}

// Uninstall removes the service
func (l *LinuxSystemdService) Uninstall() error {
	// Stop the service first
	l.Stop()

	// Disable the service
	l.disable()

	// Remove unit file
	if err := os.Remove(l.unitPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove unit file: %w", err)
	}

	// Reload systemd daemon
	l.daemonReload()

	return nil
}

// Start starts the service
func (l *LinuxSystemdService) Start() error {
	if !l.IsInstalled() {
		return fmt.Errorf("service is not installed")
	}

	cmd := exec.Command("systemctl", "--user", "start", l.config.Name)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	return nil
}

// Stop stops the service
func (l *LinuxSystemdService) Stop() error {
	cmd := exec.Command("systemctl", "--user", "stop", l.config.Name)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stop service: %w", err)
	}

	return nil
}

// Restart restarts the service
func (l *LinuxSystemdService) Restart() error {
	if !l.IsInstalled() {
		return fmt.Errorf("service is not installed")
	}

	cmd := exec.Command("systemctl", "--user", "restart", l.config.Name)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to restart service: %w", err)
	}

	return nil
}

// Status returns the current status of the service
func (l *LinuxSystemdService) Status() (ServiceStatus, error) {
	if !l.IsInstalled() {
		return StatusNotInstalled, nil
	}

	if l.IsRunning() {
		return StatusRunning, nil
	}

	return StatusStopped, nil
}

// IsInstalled checks if the service is installed
func (l *LinuxSystemdService) IsInstalled() bool {
	_, err := os.Stat(l.unitPath)
	return err == nil
}

// IsRunning checks if the service is currently running
func (l *LinuxSystemdService) IsRunning() bool {
	cmd := exec.Command("systemctl", "--user", "is-active", l.config.Name)
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	return strings.TrimSpace(string(output)) == "active"
}

// GetLogs retrieves the last n lines from the service log
func (l *LinuxSystemdService) GetLogs(lines int) ([]string, error) {
	// Try journalctl first (systemd logs)
	cmd := exec.Command("journalctl", "--user", "-u", l.config.Name, "-n", fmt.Sprintf("%d", lines), "--no-pager")
	output, err := cmd.Output()
	if err == nil {
		var logLines []string
		scanner := bufio.NewScanner(strings.NewReader(string(output)))
		for scanner.Scan() {
			logLines = append(logLines, scanner.Text())
		}
		return logLines, nil
	}

	// Fallback to file logs if journalctl fails
	if _, err := os.Stat(l.config.LogPath); os.IsNotExist(err) {
		return []string{}, nil
	}

	cmd = exec.Command("tail", "-n", fmt.Sprintf("%d", lines), l.config.LogPath)
	output, err = cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to read log file: %w", err)
	}

	var logLines []string
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		logLines = append(logLines, scanner.Text())
	}

	return logLines, nil
}

// enable enables the service to start at boot
func (l *LinuxSystemdService) enable() error {
	cmd := exec.Command("systemctl", "--user", "enable", l.config.Name)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("systemctl enable failed: %w", err)
	}
	return nil
}

// disable disables the service from starting at boot
func (l *LinuxSystemdService) disable() error {
	cmd := exec.Command("systemctl", "--user", "disable", l.config.Name)
	return cmd.Run() // Ignore errors as service might not be enabled
}

// daemonReload reloads systemd daemon to pick up changes
func (l *LinuxSystemdService) daemonReload() error {
	cmd := exec.Command("systemctl", "--user", "daemon-reload")
	return cmd.Run()
}

// generateUnitContent creates the systemd unit file content
func (l *LinuxSystemdService) generateUnitContent() string {
	// Build ExecStart command
	args := append([]string{l.config.Executable}, l.config.Args...)
	execStart := strings.Join(args, " ")

	return fmt.Sprintf(`[Unit]
Description=%s
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=%s
WorkingDirectory=%s
Restart=on-failure
RestartSec=5
StandardOutput=append:%s
StandardError=append:%s

[Install]
WantedBy=default.target
`,
		l.config.Description,
		execStart,
		l.config.WorkingDir,
		l.config.LogPath,
		l.config.LogPath,
	)
}
