package network

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

// Detector monitors network connectivity
type Detector struct {
	timeout   time.Duration
	endpoints []string
}

// NewDetector creates a new network connectivity detector
func NewDetector() *Detector {
	return &Detector{
		timeout: 10 * time.Second,
		endpoints: []string{
			"https://api.github.com",
			"https://github.com",
			"https://google.com",
		},
	}
}

// IsConnected checks if internet connectivity is available
func (d *Detector) IsConnected() bool {
	// Try each endpoint until one succeeds
	for _, endpoint := range d.endpoints {
		if d.checkEndpoint(endpoint) {
			return true
		}
	}
	return false
}

// checkEndpoint tests connectivity to a specific endpoint
func (d *Detector) checkEndpoint(endpoint string) bool {
	client := &http.Client{
		Timeout: d.timeout,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout: d.timeout,
			}).DialContext,
		},
	}

	resp, err := client.Get(endpoint)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	// Consider any HTTP response as connectivity (even errors like 404)
	return resp.StatusCode > 0
}

// WaitForConnectivity blocks until internet connectivity is available
func (d *Detector) WaitForConnectivity(ctx context.Context) error {
	if d.IsConnected() {
		return nil
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if d.IsConnected() {
				return nil
			}
		}
	}
}

// WaitForGitHubConnectivity specifically waits for GitHub connectivity
func (d *Detector) WaitForGitHubConnectivity(ctx context.Context) error {
	gitHubEndpoints := []string{
		"https://api.github.com",
		"https://github.com",
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			for _, endpoint := range gitHubEndpoints {
				if d.checkEndpoint(endpoint) {
					return nil
				}
			}
		}
	}
}

// CheckConnectivityWithRetry attempts to check connectivity with exponential backoff
func (d *Detector) CheckConnectivityWithRetry(ctx context.Context, maxRetries int) error {
	delay := time.Second

	for i := 0; i < maxRetries; i++ {
		if d.IsConnected() {
			return nil
		}

		if i == maxRetries-1 {
			return fmt.Errorf("no internet connectivity after %d attempts", maxRetries)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// Exponential backoff: 1s, 2s, 4s, 8s, etc.
			delay *= 2
			if delay > 30*time.Second {
				delay = 30 * time.Second
			}
		}
	}

	return fmt.Errorf("no internet connectivity after %d attempts", maxRetries)
}
