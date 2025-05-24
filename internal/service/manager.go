package service

import (
	"fmt"
	"runtime"
)

// ServiceStatus represents the current status of a service
type ServiceStatus int

const (
	StatusUnknown ServiceStatus = iota
	StatusStopped
	StatusRunning
	StatusInstalled
	StatusNotInstalled
)

// String returns the string representation of service status
func (s ServiceStatus) String() string {
	switch s {
	case StatusStopped:
		return "stopped"
	case StatusRunning:
		return "running"
	case StatusInstalled:
		return "installed"
	case StatusNotInstalled:
		return "not installed"
	default:
		return "unknown"
	}
}

// ServiceConfig holds configuration for system service
type ServiceConfig struct {
	Name        string
	DisplayName string
	Description string
	Executable  string
	Args        []string
	WorkingDir  string
	LogPath     string
	User        string
}

// ServiceManager interface for cross-platform service management
type ServiceManager interface {
	Install() error
	Uninstall() error
	Start() error
	Stop() error
	Restart() error
	Status() (ServiceStatus, error)
	IsInstalled() bool
	IsRunning() bool
	GetLogs(lines int) ([]string, error)
}

// NewServiceManager creates a platform-specific service manager
func NewServiceManager(config *ServiceConfig) (ServiceManager, error) {
	if config == nil {
		return nil, fmt.Errorf("service config cannot be nil")
	}

	switch runtime.GOOS {
	case "darwin":
		return NewMacOSLaunchAgent(config), nil
	case "linux":
		return NewLinuxSystemdService(config), nil
	case "windows":
		return NewWindowsService(config), nil
	default:
		return nil, fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

// DefaultConfig returns default service configuration for catapult
func DefaultConfig(executable string) *ServiceConfig {
	return &ServiceConfig{
		Name:        "catapult",
		DisplayName: "Catapult File Sync",
		Description: "Automatic file synchronization with GitHub",
		Executable:  executable,
		Args:        []string{"sync", "--watch"},
		WorkingDir:  "",
		LogPath:     "",
		User:        "",
	}
}
