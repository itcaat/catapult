package auth

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestRequestDeviceCodeDoesNotWriteResponseSecrets(t *testing.T) {
	const secret = "gho_test_access_token_should_not_be_logged"

	var output bytes.Buffer
	df := NewDeviceFlow(&Config{ClientID: "client-id", Scopes: []string{"repo"}})
	df.output = &output
	df.client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"device_code":"device-secret","user_code":"ABCD-EFGH","verification_uri":"https://github.com/login/device"}` + secret)),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}

	_, err := df.requestDeviceCode()
	if err == nil {
		t.Fatal("expected malformed response error")
	}
	if strings.Contains(output.String(), secret) || strings.Contains(output.String(), "device-secret") {
		t.Fatalf("authentication output contains a secret: %q", output.String())
	}
}

func TestRequestDeviceCodeDoesNotIncludeResponseBodyInError(t *testing.T) {
	const secret = `{"error":"invalid_client","access_token":"gho_secret"}`

	df := NewDeviceFlow(&Config{ClientID: "client-id"})
	df.client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Body:       io.NopCloser(strings.NewReader(secret)),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}

	_, err := df.requestDeviceCode()
	if err == nil {
		t.Fatal("expected status error")
	}
	if strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "gho_secret") {
		t.Fatalf("error contains response body secret: %q", err)
	}
}

func TestPollForTokenDoesNotWriteAccessToken(t *testing.T) {
	const secret = "gho_test_access_token_should_not_be_logged"

	var output bytes.Buffer
	df := NewDeviceFlow(&Config{ClientID: "client-id"})
	df.output = &output
	df.client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"access_token":"` + secret + `","token_type":"bearer"}`)),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}

	token, err := df.pollForToken("device-code", 1)
	if err != nil {
		t.Fatalf("pollForToken returned an error: %v", err)
	}
	if token.AccessToken != secret {
		t.Fatalf("unexpected access token: %q", token.AccessToken)
	}
	if strings.Contains(output.String(), secret) {
		t.Fatalf("authentication output contains the access token: %q", output.String())
	}
}
