package repository

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/google/go-github/v57/github"
)

func TestNormalizeRemotePath(t *testing.T) {
	tests := map[string]string{
		`nested\path\file.txt`: "nested/path/file.txt",
		"nested/path/file.txt": "nested/path/file.txt",
		"":                     "",
	}

	for input, want := range tests {
		if got := normalizeRemotePath(input); got != want {
			t.Errorf("normalizeRemotePath(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestGitHubRepositoryUsesDiscoveredDefaultBranchAndPOSIXPaths(t *testing.T) {
	var repositoryRequests int
	var contentRequests int
	var createPath string
	var createBranch string
	var contentRef string

	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/owner/repo":
			repositoryRequests++
			return jsonResponse(`{"default_branch":"master"}`), nil
		case r.Method == http.MethodPut && r.URL.Path == "/repos/owner/repo/contents/nested/file.txt":
			createPath = r.URL.Path
			var request struct {
				Branch string `json:"branch"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Errorf("decode create request: %v", err)
			}
			createBranch = request.Branch
			return jsonResponse(`{"content":{"sha":"new-sha"}}`), nil
		case r.Method == http.MethodGet && r.URL.Path == "/repos/owner/repo/contents/nested/file.txt":
			contentRequests++
			contentRef = r.URL.Query().Get("ref")
			return jsonResponse(`{"type":"file","content":"","sha":"file-sha"}`), nil
		default:
			return &http.Response{StatusCode: http.StatusNotFound, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("not found"))}, nil
		}
	})

	client := github.NewClient(&http.Client{Transport: transport})
	baseURL, err := url.Parse("https://api.github.test/")
	if err != nil {
		t.Fatal(err)
	}
	client.BaseURL = baseURL
	repo := &GitHubRepository{client: client, owner: "owner", name: "repo"}

	if err := repo.CreateFile(context.Background(), `nested\file.txt`, "content"); err != nil {
		t.Fatalf("CreateFile() error = %v", err)
	}
	if _, err := repo.GetFile(context.Background(), `nested\file.txt`); err != nil {
		t.Fatalf("GetFile() error = %v", err)
	}

	if repositoryRequests != 1 {
		t.Fatalf("repository metadata requested %d times, want once", repositoryRequests)
	}
	if createPath != "/repos/owner/repo/contents/nested/file.txt" {
		t.Fatalf("create path = %q", createPath)
	}
	if createBranch != "master" {
		t.Fatalf("create branch = %q, want master", createBranch)
	}
	if contentRequests != 1 || contentRef != "master" {
		t.Fatalf("content request count/ref = %d/%q, want 1/master", contentRequests, contentRef)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
