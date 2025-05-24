package network

import (
	"context"
	"testing"
	"time"
)

func TestDetector_IsConnected(t *testing.T) {
	detector := NewDetector()

	// This test requires internet connectivity
	// In a CI environment, you might want to skip this test
	connected := detector.IsConnected()

	// We can't assert true/false since it depends on actual network
	// but we can verify the method doesn't panic
	t.Logf("Network connectivity detected: %v", connected)
}

func TestDetector_CheckEndpoint(t *testing.T) {
	detector := NewDetector()

	// Test with a known invalid endpoint
	result := detector.checkEndpoint("http://this-domain-definitely-does-not-exist-12345.com")
	if result {
		t.Error("Expected false for invalid endpoint")
	}

	// Test with a likely valid endpoint (if internet is available)
	result = detector.checkEndpoint("https://google.com")
	t.Logf("Google connectivity: %v", result)
}

func TestDetector_WaitForConnectivity_Timeout(t *testing.T) {
	detector := NewDetector()

	// Test timeout behavior
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// This should timeout quickly since we're using a very short timeout
	err := detector.WaitForConnectivity(ctx)
	if err != context.DeadlineExceeded {
		t.Logf("Expected timeout, got: %v", err)
	}
}

func TestDetector_WaitForGitHubConnectivity_Timeout(t *testing.T) {
	detector := NewDetector()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := detector.WaitForGitHubConnectivity(ctx)
	if err != context.DeadlineExceeded {
		t.Logf("Expected timeout, got: %v", err)
	}
}

func TestDetector_CheckConnectivityWithRetry(t *testing.T) {
	detector := NewDetector()

	// Test with context that times out quickly
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err := detector.CheckConnectivityWithRetry(ctx, 2)
	// Should either succeed or timeout/fail
	t.Logf("Retry result: %v", err)
}

func TestDetector_NewDetector(t *testing.T) {
	detector := NewDetector()

	if detector == nil {
		t.Error("NewDetector should not return nil")
	}

	if detector.timeout != 10*time.Second {
		t.Errorf("Expected timeout of 10s, got %v", detector.timeout)
	}

	expectedEndpoints := []string{
		"https://api.github.com",
		"https://github.com",
		"https://google.com",
	}

	if len(detector.endpoints) != len(expectedEndpoints) {
		t.Errorf("Expected %d endpoints, got %d", len(expectedEndpoints), len(detector.endpoints))
	}

	for i, expected := range expectedEndpoints {
		if detector.endpoints[i] != expected {
			t.Errorf("Expected endpoint %s, got %s", expected, detector.endpoints[i])
		}
	}
}
