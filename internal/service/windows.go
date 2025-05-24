package service

import (
	"fmt"
)

// WindowsService implements ServiceManager for Windows (placeholder implementation)
type WindowsService struct {
	config *ServiceConfig
}

// NewWindowsService creates a new Windows service manager (placeholder)
func NewWindowsService(config *ServiceConfig) *WindowsService {
	return &WindowsService{
		config: config,
	}
}

// Install is not implemented for Windows yet
func (w *WindowsService) Install() error {
	return fmt.Errorf("Windows service installation is not implemented yet")
}

// Uninstall is not implemented for Windows yet
func (w *WindowsService) Uninstall() error {
	return fmt.Errorf("Windows service uninstallation is not implemented yet")
}

// Start is not implemented for Windows yet
func (w *WindowsService) Start() error {
	return fmt.Errorf("Windows service start is not implemented yet")
}

// Stop is not implemented for Windows yet
func (w *WindowsService) Stop() error {
	return fmt.Errorf("Windows service stop is not implemented yet")
}

// Restart is not implemented for Windows yet
func (w *WindowsService) Restart() error {
	return fmt.Errorf("Windows service restart is not implemented yet")
}

// Status always returns not installed for Windows
func (w *WindowsService) Status() (ServiceStatus, error) {
	return StatusNotInstalled, fmt.Errorf("Windows service status is not implemented yet")
}

// IsInstalled always returns false for Windows
func (w *WindowsService) IsInstalled() bool {
	return false
}

// IsRunning always returns false for Windows
func (w *WindowsService) IsRunning() bool {
	return false
}

// GetLogs returns empty logs for Windows
func (w *WindowsService) GetLogs(lines int) ([]string, error) {
	return []string{}, fmt.Errorf("Windows service logs are not implemented yet")
}
