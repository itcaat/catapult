package issues

import (
	"strings"
	"testing"
	"time"

	"github.com/itcaat/catapult/internal/config"
)

func TestTemplates_Generate(t *testing.T) {
	cfg := &config.IssueConfig{
		IncludeFileNames:    true,
		IncludeErrorDetails: true,
		IncludeSystemInfo:   false,
		Labels:              []string{"catapult", "auto-generated"},
	}

	templates := NewTemplates(cfg)

	issue := &Issue{
		ID:          "test-issue-1",
		Category:    CategoryConflict,
		Title:       "Test conflict",
		Description: "A test conflict occurred",
		Files:       []string{"file1.txt", "file2.txt"},
		ErrorMsg:    "merge conflict detected",
		Timestamp:   time.Now(),
	}

	content, err := templates.Generate(issue)
	if err != nil {
		t.Fatalf("Failed to generate issue content: %v", err)
	}

	// Check title format
	expectedTitle := "🔀 Sync Conflict: Test conflict"
	if content.Title != expectedTitle {
		t.Errorf("Expected title %q, got %q", expectedTitle, content.Title)
	}

	// Check that body contains expected sections
	if !strings.Contains(content.Body, "A test conflict occurred") {
		t.Error("Body should contain issue description")
	}

	if !strings.Contains(content.Body, "## 🔍 Diagnostic Information") {
		t.Error("Body should contain diagnostic information section")
	}

	if !strings.Contains(content.Body, "file1.txt, file2.txt") {
		t.Error("Body should contain affected files when IncludeFileNames is true")
	}

	if !strings.Contains(content.Body, "merge conflict detected") {
		t.Error("Body should contain error details when IncludeErrorDetails is true")
	}

	// Check labels
	expectedLabels := []string{"conflict", "sync-issue", "catapult", "auto-generated"}
	if len(content.Labels) != len(expectedLabels) {
		t.Errorf("Expected %d labels, got %d", len(expectedLabels), len(content.Labels))
	}
}

func TestTemplates_GenerateWithPrivacySettings(t *testing.T) {
	cfg := &config.IssueConfig{
		IncludeFileNames:    false,
		IncludeErrorDetails: false,
		IncludeSystemInfo:   false,
		Labels:              []string{"catapult"},
	}

	templates := NewTemplates(cfg)

	issue := &Issue{
		ID:          "test-issue-2",
		Category:    CategoryNetwork,
		Title:       "Network error",
		Description: "Connection failed",
		Files:       []string{"secret.txt"},
		ErrorMsg:    "connection refused: sensitive info",
		Timestamp:   time.Now(),
	}

	content, err := templates.Generate(issue)
	if err != nil {
		t.Fatalf("Failed to generate issue content: %v", err)
	}

	// Should not contain sensitive information when privacy settings are disabled
	if strings.Contains(content.Body, "secret.txt") {
		t.Error("Body should not contain file names when IncludeFileNames is false")
	}

	if strings.Contains(content.Body, "sensitive info") {
		t.Error("Body should not contain error details when IncludeErrorDetails is false")
	}

	// Should still contain basic diagnostic info
	if !strings.Contains(content.Body, "## 🔍 Diagnostic Information") {
		t.Error("Body should still contain diagnostic information section")
	}

	if !strings.Contains(content.Body, "network") {
		t.Error("Body should contain category information")
	}
}

func TestTemplates_AllCategories(t *testing.T) {
	cfg := &config.IssueConfig{
		IncludeFileNames:    true,
		IncludeErrorDetails: true,
		IncludeSystemInfo:   true,
		Labels:              []string{"catapult"},
	}

	templates := NewTemplates(cfg)

	categories := []IssueCategory{
		CategoryConflict,
		CategoryNetwork,
		CategoryPermission,
		CategoryAuth,
		CategoryCorruption,
		CategoryQuota,
		CategoryUnknown,
	}

	for _, category := range categories {
		issue := &Issue{
			ID:          "test-" + string(category),
			Category:    category,
			Title:       "Test " + string(category),
			Description: "Test description",
			ErrorMsg:    "test error",
			Timestamp:   time.Now(),
		}

		content, err := templates.Generate(issue)
		if err != nil {
			t.Errorf("Failed to generate content for category %s: %v", category, err)
			continue
		}

		if content.Title == "" {
			t.Errorf("Title should not be empty for category %s", category)
		}

		if content.Body == "" {
			t.Errorf("Body should not be empty for category %s", category)
		}

		if len(content.Labels) == 0 {
			t.Errorf("Labels should not be empty for category %s", category)
		}

		// Check that system info is included when enabled
		if !strings.Contains(content.Body, "Operating System") {
			t.Errorf("Body should contain system info when IncludeSystemInfo is true for category %s", category)
		}
	}
}
