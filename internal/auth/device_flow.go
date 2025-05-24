package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	fmt.Printf("Please visit: %s\n", deviceCode.VerificationURI)
	fmt.Printf("And enter code: %s\n", deviceCode.UserCode)

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

	// Print request details for debugging
	fmt.Printf("Requesting device code from: %s\n", deviceCodeURL)
	fmt.Printf("With client_id: %s\n", df.config.ClientID)
	fmt.Printf("And scopes: %s\n", strings.Join(df.config.Scopes, " "))

	// Create request
	req, err := http.NewRequest("POST", deviceCodeURL, strings.NewReader(body.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Print headers for debugging
	fmt.Printf("Request headers: %v\n", req.Header)

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

	// Print response details for debugging
	fmt.Printf("Response status: %d\n", resp.StatusCode)
	fmt.Printf("Response body: %s\n", string(data))

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(data))
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
	// Prepare request body
	body := url.Values{}
	body.Set("client_id", df.config.ClientID)
	body.Set("device_code", deviceCode)
	body.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")

	// Create request
	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(body.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Calculate timeout
	timeout := time.After(pollTimeout)
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	fmt.Println("Waiting for authorization...")
	fmt.Println("Please complete the authorization in your browser.")
	fmt.Println("This window will automatically continue once you've authorized the application.")
	fmt.Println("(Press Ctrl+C to cancel)")

	lastDot := time.Now()
	for {
		select {
		case <-timeout:
			return nil, fmt.Errorf("polling timed out after %v", pollTimeout)
		case <-ticker.C:
			// Send request
			resp, err := df.client.Do(req)
			if err != nil {
				return nil, fmt.Errorf("failed to send request: %w", err)
			}

			// Read response
			data, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				return nil, fmt.Errorf("failed to read response: %w", err)
			}

			// Check status code
			if resp.StatusCode != http.StatusOK {
				// Check for specific error responses
				var errorResp struct {
					Error            string `json:"error"`
					ErrorDescription string `json:"error_description"`
					Interval         int    `json:"interval"`
				}
				if err := json.Unmarshal(data, &errorResp); err == nil {
					switch errorResp.Error {
					case "authorization_pending":
						// Show progress dot every 5 seconds
						if time.Since(lastDot) >= 5*time.Second {
							fmt.Print(".")
							lastDot = time.Now()
						}
						continue
					case "slow_down":
						// Update interval and show message
						newInterval := errorResp.Interval
						if newInterval > 0 {
							ticker.Reset(time.Duration(newInterval) * time.Second)
							fmt.Printf("\nGitHub requested to slow down. Waiting %d seconds between checks...\n", newInterval)
						}
						continue
					case "expired_token":
						return nil, fmt.Errorf("device code expired")
					case "access_denied":
						return nil, fmt.Errorf("user denied access")
					}
				}
				return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(data))
			}

			// Parse response
			var token Token
			if err := json.Unmarshal(data, &token); err != nil {
				return nil, fmt.Errorf("failed to parse response: %w", err)
			}

			// Verify we got a valid token
			if token.AccessToken == "" {
				continue
			}

			fmt.Println("\nAuthorization successful!")
			return &token, nil
		}
	}
}
