package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	// GitHub OAuth device flow endpoints
	deviceCodeURL = "https://github.com/login/device/code"
	tokenURL      = "https://github.com/login/oauth/access_token"

	// Polling interval and timeout
	pollInterval = 5 * time.Second
	pollTimeout  = 10 * time.Minute
)

// DeviceCode represents the response from GitHub's device code endpoint
type DeviceCode struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// DeviceFlow handles the GitHub device flow authentication process
type DeviceFlow struct {
	client *http.Client
	config *Config
	output io.Writer
}

// Config holds the OAuth application configuration
type Config struct {
	ClientID string
	Scopes   []string
}

// Token represents an OAuth token
type Token struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
}

// NewDeviceFlow creates a new DeviceFlow instance
func NewDeviceFlow(config *Config) *DeviceFlow {
	return &DeviceFlow{
		client: &http.Client{},
		config: config,
		output: os.Stdout,
	}
}

// Initiate starts the device flow authentication process
func (df *DeviceFlow) Initiate() (*Token, error) {
	// Request device code
	deviceCode, err := df.requestDeviceCode()
	if err != nil {
		return nil, fmt.Errorf("failed to request device code: %w", err)
	}

	// Display user code and verification URL
	fmt.Fprintf(df.output, "Please visit: %s\n", deviceCode.VerificationURI)
	fmt.Fprintf(df.output, "And enter code: %s\n", deviceCode.UserCode)

	// Poll for token
	token, err := df.pollForToken(deviceCode.DeviceCode, deviceCode.Interval)
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}

	return token, nil
}

// requestDeviceCode requests a device code from GitHub
func (df *DeviceFlow) requestDeviceCode() (*DeviceCode, error) {
	// Prepare request body
	body := url.Values{}
	body.Set("client_id", df.config.ClientID)
	body.Set("scope", strings.Join(df.config.Scopes, " "))

	// Create request
	req, err := http.NewRequest("POST", deviceCodeURL, strings.NewReader(body.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Send request
	resp, err := df.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Parse response
	var deviceCode DeviceCode
	if err := json.Unmarshal(data, &deviceCode); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &deviceCode, nil
}

// pollForToken polls GitHub for the access token
func (df *DeviceFlow) pollForToken(deviceCode string, interval int) (*Token, error) {
	// Prepare request values. A new request must be created for every poll:
	// http.Client.Do consumes and closes the request body, so reusing the same
	// *http.Request makes subsequent POSTs send an empty body.
	body := url.Values{}
	body.Set("client_id", df.config.ClientID)
	body.Set("device_code", deviceCode)
	body.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
	bodyData := body.Encode()

	if interval <= 0 {
		interval = int(pollInterval / time.Second)
	}

	// Calculate timeout
	timeout := time.After(pollTimeout)
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	fmt.Fprintln(df.output, "Waiting for authorization...")
	fmt.Fprintln(df.output, "Please complete the authorization in your browser.")
	fmt.Fprintln(df.output, "This window will automatically continue once you've authorized the application.")
	fmt.Fprintln(df.output, "(Press Ctrl+C to cancel)")
	fmt.Fprintf(df.output, "Polling every %d seconds...\n", interval)

	lastDot := time.Now()
	pollCount := 0
	for {
		select {
		case <-timeout:
			return nil, fmt.Errorf("polling timed out after %v", pollTimeout)
		case <-ticker.C:
			pollCount++
			fmt.Fprintf(df.output, "[Poll #%d] Checking authorization status...\n", pollCount)

			// Create a fresh request for each poll because Do consumes req.Body.
			req, err := http.NewRequest("POST", tokenURL, strings.NewReader(bodyData))
			if err != nil {
				return nil, fmt.Errorf("failed to create request: %w", err)
			}
			req.Header.Set("Accept", "application/json")
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			// Send request. A transient transport failure should not abort the
			// device flow; the next poll can establish a new connection.
			resp, err := df.client.Do(req)
			if err != nil {
				fmt.Fprintf(df.output, "[Poll #%d] Request failed: %v\n", pollCount, err)
				continue
			}

			// Read response
			data, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				fmt.Fprintf(df.output, "[Poll #%d] Failed to read response: %v\n", pollCount, err)
				return nil, fmt.Errorf("failed to read response: %w", err)
			}

			fmt.Fprintf(df.output, "[Poll #%d] Response status: %d\n", pollCount, resp.StatusCode)

			// First, try to parse as an error response (GitHub returns errors with 200 status)
			var errorResp struct {
				Error            string `json:"error"`
				ErrorDescription string `json:"error_description"`
				Interval         int    `json:"interval"`
			}
			if err := json.Unmarshal(data, &errorResp); err == nil && errorResp.Error != "" {
				fmt.Fprintf(df.output, "[Poll #%d] OAuth response: %s\n", pollCount, errorResp.Error)
				switch errorResp.Error {
				case "authorization_pending":
					// Show progress dot every 5 seconds
					if time.Since(lastDot) >= 5*time.Second {
						fmt.Fprint(df.output, ".")
						lastDot = time.Now()
					}
					continue
				case "slow_down":
					// Update interval and show message
					newInterval := errorResp.Interval
					if newInterval > 0 {
						ticker.Reset(time.Duration(newInterval) * time.Second)
						fmt.Fprintf(df.output, "\nGitHub requested to slow down. Waiting %d seconds between checks...\n", newInterval)
					}
					continue
				case "expired_token":
					return nil, fmt.Errorf("device code expired")
				case "access_denied":
					return nil, fmt.Errorf("user denied access")
				default:
					return nil, fmt.Errorf("GitHub OAuth error: %s", errorResp.Error)
				}
			}

			// Check for non-200 status codes
			if resp.StatusCode != http.StatusOK {
				return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
			}

			// Parse as successful token response
			var token Token
			if err := json.Unmarshal(data, &token); err != nil {
				fmt.Fprintf(df.output, "[Poll #%d] Failed to parse OAuth response: %v\n", pollCount, err)
				return nil, fmt.Errorf("failed to parse response: %w", err)
			}

			// Verify we got a valid token
			if token.AccessToken == "" {
				fmt.Fprintf(df.output, "[Poll #%d] OAuth response did not contain an access token; continuing...\n", pollCount)
				continue
			}

			fmt.Fprintf(df.output, "[Poll #%d] Authorization successful.\n", pollCount)
			fmt.Fprintln(df.output, "\nAuthorization successful!")
			return &token, nil
		}
	}
}
