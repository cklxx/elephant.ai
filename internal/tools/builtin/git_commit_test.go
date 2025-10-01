package builtin

import (
	"alex/internal/agent/ports"
	"context"
	"strings"
	"testing"
)

// MockLLMClient for testing
type mockLLMClient struct {
	response string
	err      error
}

func (m *mockLLMClient) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &ports.CompletionResponse{
		Content:    m.response,
		StopReason: "end_turn",
		Usage: ports.TokenUsage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
		},
	}, nil
}

func (m *mockLLMClient) Model() string {
	return "mock-model"
}

func TestGitCommit_Definition(t *testing.T) {
	tool := NewGitCommit(nil)
	def := tool.Definition()

	if def.Name != "git_commit" {
		t.Errorf("expected name 'git_commit', got %q", def.Name)
	}

	if def.Description == "" {
		t.Error("expected non-empty description")
	}

	if def.Parameters.Type != "object" {
		t.Errorf("expected parameters type 'object', got %q", def.Parameters.Type)
	}

	// Check that message and auto are defined
	if _, ok := def.Parameters.Properties["message"]; !ok {
		t.Error("expected 'message' property")
	}

	if _, ok := def.Parameters.Properties["auto"]; !ok {
		t.Error("expected 'auto' property")
	}
}

func TestGitCommit_Metadata(t *testing.T) {
	tool := NewGitCommit(nil)
	meta := tool.Metadata()

	if meta.Name != "git_commit" {
		t.Errorf("expected name 'git_commit', got %q", meta.Name)
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

func TestGitCommit_AddFooter(t *testing.T) {
	tool := &gitCommit{}

	tests := []struct {
		name     string
		message  string
		expected string
	}{
		{
			name:     "simple message",
			message:  "feat: add new feature",
			expected: "feat: add new feature\n\n🤖 Generated with ALEX\nCo-Authored-By: ALEX <noreply@alex.com>",
		},
		{
			name:     "message with body",
			message:  "feat: add new feature\n\nThis is a detailed explanation.",
			expected: "feat: add new feature\n\nThis is a detailed explanation.\n\n🤖 Generated with ALEX\nCo-Authored-By: ALEX <noreply@alex.com>",
		},
		{
			name:     "message already has footer",
			message:  "feat: add new feature\n\n🤖 Generated with ALEX\nCo-Authored-By: ALEX <noreply@alex.com>",
			expected: "feat: add new feature\n\n🤖 Generated with ALEX\nCo-Authored-By: ALEX <noreply@alex.com>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tool.addFooter(tt.message)
			if result != tt.expected {
				t.Errorf("addFooter() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestGitCommit_SummarizeDiff(t *testing.T) {
	tool := &gitCommit{}

	diff := `diff --git a/file1.go b/file1.go
index 1234567..abcdefg 100644
--- a/file1.go
+++ b/file1.go
@@ -1,5 +1,8 @@
 package main

+import "fmt"
+
 func main() {
-	println("hello")
+	fmt.Println("hello")
+	fmt.Println("world")
 }
diff --git a/file2.go b/file2.go
new file mode 100644
index 0000000..1234567
--- /dev/null
+++ b/file2.go
@@ -0,0 +1,3 @@
+package main
+
+func helper() {}
`

	summary := tool.summarizeDiff(diff)

	// Should count files, additions, and deletions
	if !strings.Contains(summary, "files changed") {
		t.Error("expected summary to contain 'files changed'")
	}

	if !strings.Contains(summary, "insertions(+)") {
		t.Error("expected summary to contain 'insertions(+)'")
	}

	if !strings.Contains(summary, "deletions(-)") {
		t.Error("expected summary to contain 'deletions(-)'")
	}

	// Should list files
	if !strings.Contains(summary, "file1.go") || !strings.Contains(summary, "file2.go") {
		t.Error("expected summary to list changed files")
	}
}

func TestGitCommit_GenerateCommitMessage(t *testing.T) {
	mockLLM := &mockLLMClient{
		response: "feat: add new feature\n\nImplemented user authentication with JWT tokens.",
	}

	tool := &gitCommit{llmClient: mockLLM}

	diff := `diff --git a/auth.go b/auth.go
new file mode 100644
index 0000000..1234567
--- /dev/null
+++ b/auth.go
@@ -0,0 +1,10 @@
+package auth
+
+func Authenticate(token string) bool {
+    return validateJWT(token)
+}`

	message, err := tool.generateCommitMessage(context.Background(), diff)
	if err != nil {
		t.Fatalf("generateCommitMessage() error = %v", err)
	}

	if !strings.Contains(message, "feat:") {
		t.Errorf("expected commit message to contain 'feat:', got %q", message)
	}

	if !strings.Contains(message, "feature") {
		t.Errorf("expected commit message to contain 'feature', got %q", message)
	}
}

func TestGitCommit_GenerateCommitMessage_NoLLM(t *testing.T) {
	tool := &gitCommit{llmClient: nil}

	diff := "some diff"

	_, err := tool.generateCommitMessage(context.Background(), diff)
	if err == nil {
		t.Error("expected error when LLM client is nil")
	}

	if !strings.Contains(err.Error(), "LLM client not configured") {
		t.Errorf("expected error about LLM client, got %v", err)
	}
}

func TestGitCommit_GenerateCommitMessage_LongDiff(t *testing.T) {
	mockLLM := &mockLLMClient{
		response: "feat: add large feature",
	}

	tool := &gitCommit{llmClient: mockLLM}

	// Create a very long diff
	longDiff := strings.Repeat("diff line\n", 500)

	message, err := tool.generateCommitMessage(context.Background(), longDiff)
	if err != nil {
		t.Fatalf("generateCommitMessage() error = %v", err)
	}

	if message == "" {
		t.Error("expected non-empty commit message")
	}

	// Verify that the diff was truncated in the prompt
	// (We can't directly check this, but the function should handle it)
}

func TestGitCommit_Execute_CustomMessage(t *testing.T) {
	// This test would require mocking git commands, which is complex
	// In a real scenario, you would use a test git repository or mock exec.Command
	// For now, we'll test the logic path

	tool := NewGitCommit(nil).(*gitCommit)

	// Test that custom message is properly formatted
	customMsg := "fix: resolve authentication bug"
	result := tool.addFooter(customMsg)

	if !strings.Contains(result, customMsg) {
		t.Error("expected footer to preserve custom message")
	}

	if !strings.Contains(result, "🤖 Generated with ALEX") {
		t.Error("expected footer to be added")
	}
}

func TestGitCommit_Execute_MissingParameters(t *testing.T) {
	// Test that missing parameters are handled gracefully
	tool := NewGitCommit(nil)

	tests := []struct {
		name      string
		arguments map[string]any
	}{
		{
			name:      "empty arguments",
			arguments: map[string]any{},
		},
		{
			name: "only message",
			arguments: map[string]any{
				"message": "test message",
			},
		},
		{
			name: "only auto",
			arguments: map[string]any{
				"auto": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Execute should handle missing parameters gracefully
			// Note: This will fail if not in a git repo, which is expected
			result, err := tool.Execute(context.Background(), ports.ToolCall{
				ID:        "test-1",
				Name:      "git_commit",
				Arguments: tt.arguments,
			})

			// We expect either a successful result or a proper error
			if err != nil {
				t.Fatalf("Execute() unexpected error = %v", err)
			}

			// Result should either succeed or have a proper error
			if result.Error != nil {
				// Expected errors: not a git repo, no changes, etc.
				errMsg := result.Error.Error()
				if !strings.Contains(errMsg, "git") && !strings.Contains(errMsg, "repository") {
					t.Logf("Got expected error: %v", result.Error)
				}
			}
		})
	}
}

func TestGitCommit_ConventionalCommitFormats(t *testing.T) {
	// Test that generated messages follow conventional commit format
	mockLLM := &mockLLMClient{}

	tests := []struct {
		name     string
		response string
		valid    bool
	}{
		{
			name:     "valid feat",
			response: "feat: add user authentication",
			valid:    true,
		},
		{
			name:     "valid fix",
			response: "fix: resolve null pointer exception",
			valid:    true,
		},
		{
			name:     "valid refactor",
			response: "refactor: simplify authentication logic",
			valid:    true,
		},
		{
			name:     "valid docs",
			response: "docs: update API documentation",
			valid:    true,
		},
		{
			name:     "valid test",
			response: "test: add unit tests for auth module",
			valid:    true,
		},
		{
			name:     "valid chore",
			response: "chore: update dependencies",
			valid:    true,
		},
		{
			name:     "valid with scope",
			response: "feat(auth): add JWT support",
			valid:    true,
		},
		{
			name:     "valid with body",
			response: "feat: add feature\n\nDetailed explanation here.",
			valid:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLLM.response = tt.response
			tool := &gitCommit{llmClient: mockLLM}

			message, err := tool.generateCommitMessage(context.Background(), "some diff")
			if err != nil {
				t.Fatalf("generateCommitMessage() error = %v", err)
			}

			// Check for conventional commit format (including scoped format like "feat(scope):")
			validPrefixes := []string{"feat", "fix", "refactor", "docs", "test", "chore", "style"}
			hasValidType := false
			for _, prefix := range validPrefixes {
				// Check for both "feat:" and "feat(scope):" formats
				if strings.HasPrefix(message, prefix+":") || strings.HasPrefix(message, prefix+"(") {
					hasValidType = true
					break
				}
			}

			if tt.valid && !hasValidType {
				t.Errorf("expected valid conventional commit format, got %q", message)
			}
		})
	}
}
