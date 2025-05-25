package issues

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/itcaat/catapult/internal/config"
)

// IssueTemplate defines the structure for generating issue content
type IssueTemplate struct {
	Category    IssueCategory
	TitleFormat string
	BodyFormat  string
	Labels      []string
	Priority    string
}

// Templates manages issue template generation
type Templates struct {
	templates map[IssueCategory]*IssueTemplate
	config    *config.IssueConfig
}

// NewTemplates creates a new Templates instance with default templates
func NewTemplates(cfg *config.IssueConfig) *Templates {
	t := &Templates{
		templates: make(map[IssueCategory]*IssueTemplate),
		config:    cfg,
	}

	t.initializeDefaultTemplates()
	return t
}

// Generate creates issue content from a template based on the issue category
func (t *Templates) Generate(issue *Issue) (*IssueContent, error) {
	template, exists := t.templates[issue.Category]
	if !exists {
		template = t.templates[CategoryUnknown]
	}

	// Generate title
	title := fmt.Sprintf(template.TitleFormat, issue.Title)

	// Generate body with diagnostic information
	body := t.generateBody(template, issue)

	// Combine template labels with config labels
	labels := append(template.Labels, t.config.Labels...)

	return &IssueContent{
		Title:  title,
		Body:   body,
		Labels: labels,
	}, nil
}

// generateBody creates the issue body with diagnostic information
func (t *Templates) generateBody(template *IssueTemplate, issue *Issue) string {
	var buf strings.Builder

	// Issue description
	buf.WriteString(fmt.Sprintf(template.BodyFormat, issue.Description))
	buf.WriteString("\n\n")

	// Diagnostic information section
	buf.WriteString("## 🔍 Diagnostic Information\n\n")
	buf.WriteString(fmt.Sprintf("- **Timestamp**: %s\n", issue.Timestamp.Format(time.RFC3339)))
	buf.WriteString(fmt.Sprintf("- **Category**: %s\n", issue.Category))
	buf.WriteString(fmt.Sprintf("- **Issue ID**: `%s`\n", issue.ID))

	// File information (if enabled and available)
	if len(issue.Files) > 0 && t.config.IncludeFileNames {
		buf.WriteString(fmt.Sprintf("- **Affected Files**: %s\n", strings.Join(issue.Files, ", ")))
	}

	// Error details (if enabled and available)
	if issue.ErrorMsg != "" && t.config.IncludeErrorDetails {
		buf.WriteString(fmt.Sprintf("- **Error Details**: \n```\n%s\n```\n", issue.ErrorMsg))
	}

	// System information (if enabled)
	if t.config.IncludeSystemInfo {
		buf.WriteString(fmt.Sprintf("- **Operating System**: %s\n", runtime.GOOS))
		buf.WriteString(fmt.Sprintf("- **Architecture**: %s\n", runtime.GOARCH))
		buf.WriteString(fmt.Sprintf("- **Go Version**: %s\n", runtime.Version()))
	}

	// Additional metadata
	if len(issue.Metadata) > 0 {
		buf.WriteString("- **Additional Context**:\n")
		for key, value := range issue.Metadata {
			buf.WriteString(fmt.Sprintf("  - %s: %v\n", key, value))
		}
	}

	// Auto-generated footer
	buf.WriteString("\n---\n")
	buf.WriteString("*🤖 This issue was automatically created by Catapult. ")
	buf.WriteString("It will be automatically resolved when the problem is fixed.*\n\n")
	buf.WriteString("**Need help?** Check the [Catapult documentation](https://github.com/itcaat/catapult) ")
	buf.WriteString("or [create a manual issue](https://github.com/itcaat/catapult/issues/new) if this appears to be a bug.")

	return buf.String()
}

// initializeDefaultTemplates sets up the default issue templates for each category
func (t *Templates) initializeDefaultTemplates() {
	t.templates[CategoryConflict] = &IssueTemplate{
		Category:    CategoryConflict,
		TitleFormat: "🔀 Sync Conflict: %s",
		BodyFormat: `A file synchronization conflict has been detected that requires attention.

**Problem**: %s

This conflict occurs when the same file has been modified both locally and remotely, and automatic merging is not possible. Manual intervention may be required to resolve this conflict.`,
		Labels:   []string{"conflict", "sync-issue"},
		Priority: "high",
	}

	t.templates[CategoryNetwork] = &IssueTemplate{
		Category:    CategoryNetwork,
		TitleFormat: "🌐 Network Issue: %s",
		BodyFormat: `A network connectivity issue is preventing synchronization.

**Problem**: %s

This issue typically resolves automatically when network connectivity is restored. If the problem persists, please check your internet connection and GitHub service status.`,
		Labels:   []string{"network", "connectivity"},
		Priority: "medium",
	}

	t.templates[CategoryPermission] = &IssueTemplate{
		Category:    CategoryPermission,
		TitleFormat: "🔒 Permission Issue: %s",
		BodyFormat: `A permission or access issue is preventing synchronization.

**Problem**: %s

This may be due to insufficient GitHub permissions, file system permissions, or repository access restrictions. Please verify your authentication and repository access.`,
		Labels:   []string{"permissions", "access"},
		Priority: "high",
	}

	t.templates[CategoryAuth] = &IssueTemplate{
		Category:    CategoryAuth,
		TitleFormat: "🔐 Authentication Issue: %s",
		BodyFormat: `An authentication problem is preventing access to GitHub.

**Problem**: %s

This typically indicates that your GitHub token has expired, been revoked, or lacks the necessary permissions. Please re-authenticate using 'catapult init'.`,
		Labels:   []string{"authentication", "token"},
		Priority: "high",
	}

	t.templates[CategoryCorruption] = &IssueTemplate{
		Category:    CategoryCorruption,
		TitleFormat: "💥 Data Corruption: %s",
		BodyFormat: `Potential data corruption has been detected during synchronization.

**Problem**: %s

This is a serious issue that may indicate file corruption, repository corruption, or data integrity problems. Please backup your data and investigate immediately.`,
		Labels:   []string{"corruption", "data-integrity", "urgent"},
		Priority: "critical",
	}

	t.templates[CategoryQuota] = &IssueTemplate{
		Category:    CategoryQuota,
		TitleFormat: "📊 Quota/Limit Issue: %s",
		BodyFormat: `A quota or rate limit has been exceeded.

**Problem**: %s

This may be due to GitHub API rate limits, repository size limits, or file size restrictions. The issue may resolve automatically after the rate limit resets.`,
		Labels:   []string{"quota", "rate-limit"},
		Priority: "medium",
	}

	t.templates[CategoryUnknown] = &IssueTemplate{
		Category:    CategoryUnknown,
		TitleFormat: "❓ Sync Issue: %s",
		BodyFormat: `An unexpected synchronization issue has occurred.

**Problem**: %s

This issue could not be automatically categorized. Please review the diagnostic information below and consider reporting this as a bug if it appears to be a Catapult issue.`,
		Labels:   []string{"unknown", "needs-investigation"},
		Priority: "medium",
	}
}
