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

// MacOSLaunchAgent implements ServiceManager for macOS using LaunchAgent
type MacOSLaunchAgent struct {
	config    *ServiceConfig
	plistPath string
	label     string
}

// NewMacOSLaunchAgent creates a new macOS LaunchAgent service manager
func NewMacOSLaunchAgent(config *ServiceConfig) *MacOSLaunchAgent {
	currentUser, _ := user.Current()
	label := fmt.Sprintf("com.itcaat.%s", config.Name)
	plistPath := filepath.Join(currentUser.HomeDir, "Library", "LaunchAgents", label+".plist")

	// Set default log path if not provided
	if config.LogPath == "" {
		config.LogPath = filepath.Join(currentUser.HomeDir, "Library", "Logs", config.Name+".log")
	}

	// Set default working directory if not provided
	if config.WorkingDir == "" {
		config.WorkingDir = currentUser.HomeDir
	}

	return &MacOSLaunchAgent{
		config:    config,
		plistPath: plistPath,
		label:     label,
	}
}

// Install installs the service as a LaunchAgent
func (m *MacOSLaunchAgent) Install() error {
	// Create LaunchAgents directory if it doesn't exist
	launchAgentsDir := filepath.Dir(m.plistPath)
	if err := os.MkdirAll(launchAgentsDir, 0755); err != nil {
		return fmt.Errorf("failed to create LaunchAgents directory: %w", err)
	}

	// Create log directory if it doesn't exist
	logDir := filepath.Dir(m.config.LogPath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Create plist content
	plistContent := m.generatePlistContent()

	// Write plist file
	if err := os.WriteFile(m.plistPath, []byte(plistContent), 0644); err != nil {
		return fmt.Errorf("failed to write plist file: %w", err)
	}

	// Load the service
	if err := m.load(); err != nil {
		// Try to clean up on failure
		os.Remove(m.plistPath)
		return fmt.Errorf("failed to load service: %w", err)
	}

	return nil
}

// Uninstall removes the service
func (m *MacOSLaunchAgent) Uninstall() error {
	// Unload the service first
	m.unload()

	// Remove plist file
	if err := os.Remove(m.plistPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove plist file: %w", err)
	}

	return nil
}

// Start starts the service
func (m *MacOSLaunchAgent) Start() error {
	if !m.IsInstalled() {
		return fmt.Errorf("service is not installed")
	}

	cmd := exec.Command("launchctl", "start", m.label)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	return nil
}

// Stop stops the service
func (m *MacOSLaunchAgent) Stop() error {
	if !m.IsInstalled() {
		return fmt.Errorf("service is not installed")
	}

	cmd := exec.Command("launchctl", "stop", m.label)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stop service: %w", err)
	}

	return nil
}

// Restart restarts the service
func (m *MacOSLaunchAgent) Restart() error {
	if err := m.Stop(); err != nil {
		// Continue even if stop fails
		fmt.Printf("Warning: failed to stop service: %v\n", err)
	}

	return m.Start()
}

// Status returns the current status of the service
func (m *MacOSLaunchAgent) Status() (ServiceStatus, error) {
	if !m.IsInstalled() {
		return StatusNotInstalled, nil
	}

	if m.IsRunning() {
		return StatusRunning, nil
	}

	return StatusStopped, nil
}

// IsInstalled checks if the service is installed
func (m *MacOSLaunchAgent) IsInstalled() bool {
	_, err := os.Stat(m.plistPath)
	return err == nil
}

// IsRunning checks if the service is currently running
func (m *MacOSLaunchAgent) IsRunning() bool {
	cmd := exec.Command("launchctl", "list")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	return strings.Contains(string(output), m.label)
}

// GetLogs retrieves the last n lines from the service log
func (m *MacOSLaunchAgent) GetLogs(lines int) ([]string, error) {
	if _, err := os.Stat(m.config.LogPath); os.IsNotExist(err) {
		return []string{}, nil
	}

	cmd := exec.Command("tail", "-n", fmt.Sprintf("%d", lines), m.config.LogPath)
	output, err := cmd.Output()
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

// load loads the service using launchctl
func (m *MacOSLaunchAgent) load() error {
	cmd := exec.Command("launchctl", "load", "-w", m.plistPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("launchctl load failed: %w", err)
	}
	return nil
}

// unload unloads the service using launchctl
func (m *MacOSLaunchAgent) unload() error {
	cmd := exec.Command("launchctl", "unload", "-w", m.plistPath)
	return cmd.Run() // Ignore errors as service might not be loaded
}

// generatePlistContent creates the LaunchAgent plist XML content
func (m *MacOSLaunchAgent) generatePlistContent() string {
	// Build program arguments
	args := []string{m.config.Executable}
	args = append(args, m.config.Args...)

	// Generate ProgramArguments XML
	programArgsXML := ""
	for _, arg := range args {
		programArgsXML += fmt.Sprintf("\t\t<string>%s</string>\n", arg)
	}

	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
%s	</array>
	<key>WorkingDirectory</key>
	<string>%s</string>
	<key>StandardOutPath</key>
	<string>%s</string>
	<key>StandardErrorPath</key>
	<string>%s</string>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<dict>
		<key>NetworkState</key>
		<true/>
	</dict>
	<key>ThrottleInterval</key>
	<integer>30</integer>
</dict>
</plist>`,
		m.label,
		programArgsXML,
		m.config.WorkingDir,
		m.config.LogPath,
		m.config.LogPath,
	)
}
