package service

import (
	"runtime"
	"testing"
)

func TestServiceStatus_String(t *testing.T) {
	tests := []struct {
		status   ServiceStatus
		expected string
	}{
		{StatusStopped, "stopped"},
		{StatusRunning, "running"},
		{StatusInstalled, "installed"},
		{StatusNotInstalled, "not installed"},
		{StatusUnknown, "unknown"},
	}

	for _, test := range tests {
		if test.status.String() != test.expected {
			t.Errorf("Expected %s, got %s", test.expected, test.status.String())
		}
	}
}

func TestNewServiceManager(t *testing.T) {
	config := &ServiceConfig{
		Name:        "test-service",
		DisplayName: "Test Service",
		Description: "Test service description",
		Executable:  "/usr/bin/test",
		Args:        []string{"--test"},
		WorkingDir:  "/tmp",
		LogPath:     "/tmp/test.log",
		User:        "test",
	}

	manager, err := NewServiceManager(config)
	if err != nil {
		// Windows is expected to work but other platforms should succeed
		if runtime.GOOS == "windows" {
			// Windows implementation returns not implemented errors, which is expected
			t.Logf("Windows service manager created but not implemented: %v", err)
		} else {
			t.Errorf("Failed to create service manager: %v", err)
		}
	}

	if manager != nil {
		// Basic interface check
		status, err := manager.Status()
		if err != nil && runtime.GOOS != "windows" {
			// It's okay if we can't get status in test environment
			t.Logf("Status check failed (expected in test env): %v", err)
		}

		t.Logf("Service status: %s", status)
	}
}

func TestNewServiceManager_NilConfig(t *testing.T) {
	_, err := NewServiceManager(nil)
	if err == nil {
		t.Error("Expected error for nil config, got nil")
	}

	expectedError := "service config cannot be nil"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestDefaultConfig(t *testing.T) {
	executable := "/usr/bin/catapult"
	config := DefaultConfig(executable)

	if config == nil {
		t.Fatal("DefaultConfig returned nil")
	}

	if config.Name != "catapult" {
		t.Errorf("Expected name 'catapult', got '%s'", config.Name)
	}

	if config.DisplayName != "Catapult File Sync" {
		t.Errorf("Expected display name 'Catapult File Sync', got '%s'", config.DisplayName)
	}

	if config.Description != "Automatic file synchronization with GitHub" {
		t.Errorf("Expected description 'Automatic file synchronization with GitHub', got '%s'", config.Description)
	}

	if config.Executable != executable {
		t.Errorf("Expected executable '%s', got '%s'", executable, config.Executable)
	}

	expectedArgs := []string{"sync", "--watch"}
	if len(config.Args) != len(expectedArgs) {
		t.Errorf("Expected %d args, got %d", len(expectedArgs), len(config.Args))
	}

	for i, arg := range expectedArgs {
		if config.Args[i] != arg {
			t.Errorf("Expected arg[%d] '%s', got '%s'", i, arg, config.Args[i])
		}
	}
}

func TestPlatformSpecificServiceManager(t *testing.T) {
	config := DefaultConfig("/test/catapult")

	switch runtime.GOOS {
	case "darwin":
		manager := NewMacOSLaunchAgent(config)
		if manager == nil {
			t.Error("NewMacOSLaunchAgent returned nil")
		}
	case "linux":
		manager := NewLinuxSystemdService(config)
		if manager == nil {
			t.Error("NewLinuxSystemdService returned nil")
		}
	case "windows":
		manager := NewWindowsService(config)
		if manager == nil {
			t.Error("NewWindowsService returned nil")
		}
	default:
		t.Logf("Platform %s not specifically tested", runtime.GOOS)
	}
}
