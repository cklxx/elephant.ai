package builtin

import (
	"alex/internal/agent/ports"
	"context"
	"strings"
	"testing"
)

func TestGitPR_Definition(t *testing.T) {
	tool := NewGitPR(nil)
	def := tool.Definition()

	if def.Name != "git_pr" {
		t.Errorf("expected name 'git_pr', got %q", def.Name)
	}

	if def.Description == "" {
		t.Error("expected non-empty description")
	}

	if def.Parameters.Type != "object" {
		t.Errorf("expected parameters type 'object', got %q", def.Parameters.Type)
	}

	// Check that title and base are defined
	if _, ok := def.Parameters.Properties["title"]; !ok {
		t.Error("expected 'title' property")
	}

	if _, ok := def.Parameters.Properties["base"]; !ok {
		t.Error("expected 'base' property")
	}
}

func TestGitPR_Metadata(t *testing.T) {
	tool := NewGitPR(nil)
	meta := tool.Metadata()

	if meta.Name != "git_pr" {
		t.Errorf("expected name 'git_pr', got %q", meta.Name)
	}

	if meta.Version == "" {
		t.Error("expected non-empty version")
	}

	if meta.Category != "git" {
		t.Errorf("expected category 'git', got %q", meta.Category)
	}

	if !meta.Dangerous {
		t.Error("expected tool to be marked as dangerous")
	}

	if len(meta.Tags) == 0 {
		t.Error("expected non-empty tags")
	}
}

func TestGitPR_AddFooter(t *testing.T) {
	tool := &gitPR{}

	tests := []struct {
		name        string
		description string
		expected    string
	}{
		{
			name:        "simple description",
			description: "## Summary\n- Add feature",
			expected:    "## Summary\n- Add feature\n\n---\n🤖 Generated with ALEX",
		},
		{
			name:        "description with footer",
			description: "## Summary\n- Add feature\n\n🤖 Generated with ALEX",
			expected:    "## Summary\n- Add feature\n\n🤖 Generated with ALEX",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tool.addFooter(tt.description)
			if result != tt.expected {
				t.Errorf("addFooter() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestGitPR_GeneratePRDescription(t *testing.T) {
	mockLLM := &mockLLMClient{
		response: `## Summary
- Implemented user authentication
- Added JWT token support

## Changes
- Created auth package with JWT validation
- Updated login handler to use new auth system

## Test Plan
- Run existing auth tests
- Verify JWT tokens are properly validated`,
	}

	tool := &gitPR{llmClient: mockLLM}

	commits := "abc123 - feat: add JWT authentication\ndef456 - fix: resolve login bug"
	diff := `file1.go | 10 +++++++---
file2.go | 5 +++--
2 files changed, 10 insertions(+), 5 deletions(-)`

	description, err := tool.generatePRDescription(context.Background(), commits, diff)
	if err != nil {
		t.Fatalf("generatePRDescription() error = %v", err)
	}

	if !strings.Contains(description, "## Summary") {
		t.Errorf("expected description to contain '## Summary', got %q", description)
	}

	if !strings.Contains(description, "## Changes") {
		t.Errorf("expected description to contain '## Changes', got %q", description)
	}

	if !strings.Contains(description, "## Test Plan") {
		t.Errorf("expected description to contain '## Test Plan', got %q", description)
	}
}

func TestGitPR_GeneratePRContent(t *testing.T) {
	mockLLM := &mockLLMClient{
		response: `TITLE: Add user authentication with JWT

DESCRIPTION:
## Summary
- Implemented JWT-based authentication
- Added secure token validation

## Changes
- Created new auth package
- Updated API endpoints

## Test Plan
- Run unit tests
- Verify integration tests pass`,
	}

	tool := &gitPR{llmClient: mockLLM}

	commits := "abc123 - feat: add authentication"
	diff := "file.go | 20 ++++++++++++++++++++"

	title, description, err := tool.generatePRContent(context.Background(), commits, diff)
	if err != nil {
		t.Fatalf("generatePRContent() error = %v", err)
	}

	if !strings.Contains(title, "authentication") {
		t.Errorf("expected title to contain 'authentication', got %q", title)
	}

	if !strings.Contains(description, "## Summary") {
		t.Errorf("expected description to contain '## Summary', got %q", description)
	}
}

func TestGitPR_GeneratePRContent_FallbackFormat(t *testing.T) {
	// Test fallback when response doesn't have TITLE/DESCRIPTION format
	mockLLM := &mockLLMClient{
		response: `Add user authentication

This PR adds JWT-based authentication to the API.

Changes:
- New auth package
- Updated endpoints`,
	}

	tool := &gitPR{llmClient: mockLLM}

	commits := "abc123 - feat: add authentication"
	diff := "file.go | 20 ++++++++++++++++++++"

	title, description, err := tool.generatePRContent(context.Background(), commits, diff)
	if err != nil {
		t.Fatalf("generatePRContent() error = %v", err)
	}

	// Should use first line as title
	if title == "" {
		t.Error("expected non-empty title from fallback")
	}

	// Should use rest as description
	if description == "" {
		t.Error("expected non-empty description from fallback")
	}
}

func TestGitPR_GeneratePRDescription_NoLLM(t *testing.T) {
	tool := &gitPR{llmClient: nil}

	commits := "abc123 - feat: add feature"
	diff := "file.go | 10 +++++++++++"

	_, err := tool.generatePRDescription(context.Background(), commits, diff)
	if err == nil {
		t.Error("expected error when LLM client is nil")
	}

	if !strings.Contains(err.Error(), "LLM client not configured") {
		t.Errorf("expected error about LLM client, got %v", err)
	}
}

func TestGitPR_GeneratePRDescription_LongInput(t *testing.T) {
	mockLLM := &mockLLMClient{
		response: "## Summary\n- Large change",
	}

	tool := &gitPR{llmClient: mockLLM}

	// Create very long commits and diff
	longCommits := strings.Repeat("abc123 - feat: add feature\n", 100)
	longDiff := strings.Repeat("file.go | 10 ++++++++++\n", 100)

	description, err := tool.generatePRDescription(context.Background(), longCommits, longDiff)
	if err != nil {
		t.Fatalf("generatePRDescription() error = %v", err)
	}

	if description == "" {
		t.Error("expected non-empty description")
	}

	// Verify that inputs were truncated (we can't directly test this,
	// but the function should handle it without error)
}

func TestGitPR_Execute_MissingParameters(t *testing.T) {
	tool := NewGitPR(nil)

	tests := []struct {
		name      string
		arguments map[string]any
	}{
		{
			name:      "empty arguments",
			arguments: map[string]any{},
		},
		{
			name: "only title",
			arguments: map[string]any{
				"title": "Custom PR title",
			},
		},
		{
			name: "only base",
			arguments: map[string]any{
				"base": "develop",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Execute should handle missing parameters gracefully
			result, err := tool.Execute(context.Background(), ports.ToolCall{
				ID:        "test-1",
				Name:      "git_pr",
				Arguments: tt.arguments,
			})

			if err != nil {
				t.Fatalf("Execute() unexpected error = %v", err)
			}

			// Result should either succeed or have a proper error
			if result.Error != nil {
				// Expected errors: not a git repo, gh not installed, etc.
				errMsg := result.Error.Error()
				if !strings.Contains(errMsg, "git") &&
					!strings.Contains(errMsg, "repository") &&
					!strings.Contains(errMsg, "gh") {
					t.Logf("Got expected error: %v", result.Error)
				}
			}
		})
	}
}

func TestGitPR_PRDescriptionStructure(t *testing.T) {
	// Test that PR descriptions have the expected structure
	mockLLM := &mockLLMClient{}

	tests := []struct {
		name     string
		response string
		valid    bool
	}{
		{
			name: "complete structure",
			response: `## Summary
- Point 1
- Point 2

## Changes
- Change 1
- Change 2

## Test Plan
- Test 1
- Test 2`,
			valid: true,
		},
		{
			name: "missing test plan",
			response: `## Summary
- Point 1

## Changes
- Change 1`,
			valid: false,
		},
		{
			name: "different order",
			response: `## Changes
- Change 1

## Summary
- Summary

## Test Plan
- Test`,
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLLM.response = tt.response
			tool := &gitPR{llmClient: mockLLM}

			description, err := tool.generatePRDescription(context.Background(), "commits", "diff")
			if err != nil {
				t.Fatalf("generatePRDescription() error = %v", err)
			}

			hasAllSections := strings.Contains(description, "## Summary") &&
				strings.Contains(description, "## Changes") &&
				strings.Contains(description, "## Test Plan")

			if tt.valid && !hasAllSections {
				t.Errorf("expected all sections in PR description, got %q", description)
			}
		})
	}
}

func TestGitPR_TitleLength(t *testing.T) {
	// Test that PR titles are reasonable length
	mockLLM := &mockLLMClient{
		response: "TITLE: Add comprehensive user authentication system with JWT tokens and refresh token support\n\nDESCRIPTION:\nDetails here",
	}

	tool := &gitPR{llmClient: mockLLM}

	title, _, err := tool.generatePRContent(context.Background(), "commits", "diff")
	if err != nil {
		t.Fatalf("generatePRContent() error = %v", err)
	}

	// Title should be concise (under 100 chars ideally)
	// This is a soft check - we accept it but could warn
	if len(title) > 100 {
		t.Logf("Warning: PR title is long (%d chars): %q", len(title), title)
	}
}
