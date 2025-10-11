package builtin

import (
	"alex/internal/agent/ports"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestGitIntegration_FullWorkflow tests the complete Git workflow:
// 1. Make changes to files
// 2. Use git_commit to commit changes
// 3. Use git_pr to create a pull request
// 4. Use git_history to search the commit
//
// This test requires:
// - git to be installed
// - gh CLI to be installed (optional - will skip PR test if not available)
// - A real git repository (creates a temp one for testing)
func TestGitIntegration_FullWorkflow(t *testing.T) {
	// Skip if git is not installed
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed, skipping integration test")
	}

	// Create a temporary directory for the test repository
	tempDir, err := os.MkdirTemp("", "alex-git-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Initialize git repository
	if err := initTestRepo(t, tempDir); err != nil {
		t.Fatalf("failed to initialize test repo: %v", err)
	}

	// Change to temp directory
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}
	defer func() { _ = os.Chdir(originalDir) }()

	// Create mock LLM client
	mockLLM := &mockLLMClient{
		response: "feat: add test feature\n\nThis commit adds a test feature for integration testing.",
	}

	ctx := context.Background()

	// Step 1: Make changes to a file
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("Hello, Git!\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Stage the file
	if err := runGitCommandInDir(tempDir, "add", "test.txt"); err != nil {
		t.Fatalf("failed to stage file: %v", err)
	}

	// Step 2: Test git_commit tool
	t.Run("GitCommit", func(t *testing.T) {
		commitTool := NewGitCommit(mockLLM)

		// First, get the proposed commit message (without --auto)
		result, err := commitTool.Execute(ctx, ports.ToolCall{
			ID:   "test-commit-1",
			Name: "git_commit",
			Arguments: map[string]any{
				"auto": false,
			},
		})

		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		if result.Error != nil {
			t.Fatalf("Execute() result error = %v", result.Error)
		}

		// Check that we got a proposed message
		if !strings.Contains(result.Content, "Proposed commit message") {
			t.Errorf("expected proposed commit message, got %q", result.Content)
		}

		// Check metadata
		if result.Metadata == nil {
			t.Fatal("expected metadata")
		}

		requiresApproval, ok := result.Metadata["requires_approval"].(bool)
		if !ok || !requiresApproval {
			t.Error("expected requires_approval to be true")
		}

		// Now commit with auto=true
		result, err = commitTool.Execute(ctx, ports.ToolCall{
			ID:   "test-commit-2",
			Name: "git_commit",
			Arguments: map[string]any{
				"auto": true,
			},
		})

		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		if result.Error != nil {
			t.Fatalf("Execute() result error = %v", result.Error)
		}

		// Check that commit was successful
		if !strings.Contains(result.Content, "Successfully committed") {
			t.Errorf("expected successful commit, got %q", result.Content)
		}

		// Verify commit was actually created
		cmd := exec.Command("git", "log", "-1", "--oneline")
		cmd.Dir = tempDir
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("failed to check git log: %v", err)
		}

		if !strings.Contains(string(output), "feat:") {
			t.Errorf("expected commit to contain 'feat:', got %q", string(output))
		}
	})

	// Step 3: Test git_history tool
	t.Run("GitHistory", func(t *testing.T) {
		historyTool := NewGitHistory()

		// Search for the commit we just made
		result, err := historyTool.Execute(ctx, ports.ToolCall{
			ID:   "test-history-1",
			Name: "git_history",
			Arguments: map[string]any{
				"query": "feat",
				"type":  "message",
				"limit": 5.0,
			},
		})

		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		if result.Error != nil {
			t.Fatalf("Execute() result error = %v", result.Error)
		}

		// Should find our commit
		if !strings.Contains(result.Content, "feat:") {
			t.Errorf("expected to find 'feat:' in history, got %q", result.Content)
		}

		// Test file history
		result, err = historyTool.Execute(ctx, ports.ToolCall{
			ID:   "test-history-2",
			Name: "git_history",
			Arguments: map[string]any{
				"type": "file",
				"file": "test.txt",
			},
		})

		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		if result.Error != nil {
			t.Fatalf("Execute() result error = %v", result.Error)
		}

		// Should show history for test.txt
		if !strings.Contains(result.Content, "test.txt") {
			t.Errorf("expected file history for test.txt, got %q", result.Content)
		}
	})

	// Step 4: Test git_pr tool (only if gh CLI is available)
	t.Run("GitPR", func(t *testing.T) {
		// Check if gh CLI is installed
		if _, err := exec.LookPath("gh"); err != nil {
			t.Skip("gh CLI not installed, skipping PR test")
		}

		// Create a feature branch
		if err := runGitCommandInDir(tempDir, "checkout", "-b", "feature-test"); err != nil {
			t.Fatalf("failed to create feature branch: %v", err)
		}

		// Make another change
		testFile2 := filepath.Join(tempDir, "test2.txt")
		if err := os.WriteFile(testFile2, []byte("Another feature\n"), 0644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		// Commit the change
		if err := runGitCommandInDir(tempDir, "add", "test2.txt"); err != nil {
			t.Fatalf("failed to stage file: %v", err)
		}

		if err := runGitCommandInDir(tempDir, "commit", "-m", "feat: add another feature"); err != nil {
			t.Fatalf("failed to commit: %v", err)
		}

		mockLLM.response = `TITLE: Add test features

DESCRIPTION:
## Summary
- Added test feature
- Added another feature

## Changes
- Created test.txt with initial feature
- Created test2.txt with additional feature

## Test Plan
- Verify both files exist
- Verify content is correct`

		prTool := NewGitPR(mockLLM)

		// Note: This will fail if not connected to a GitHub repository
		// In a real integration test, you'd set up a test GitHub repo
		result, err := prTool.Execute(ctx, ports.ToolCall{
			ID:   "test-pr-1",
			Name: "git_pr",
			Arguments: map[string]any{
				"base": "main",
			},
		})

		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		// This may fail if not connected to GitHub, which is expected
		if result.Error != nil {
			// Log the error but don't fail the test
			t.Logf("PR creation failed (expected if not connected to GitHub): %v", result.Error)
		} else {
			// If it succeeded, check the output
			if !strings.Contains(result.Content, "pull request") {
				t.Logf("Unexpected PR output: %q", result.Content)
			}
		}
	})
}

// TestGitIntegration_CustomCommitMessage tests using a custom commit message
func TestGitIntegration_CustomCommitMessage(t *testing.T) {
	// Skip if git is not installed
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed, skipping integration test")
	}

	// Create a temporary directory for the test repository
	tempDir, err := os.MkdirTemp("", "alex-git-test-custom-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Initialize git repository
	if err := initTestRepo(t, tempDir); err != nil {
		t.Fatalf("failed to initialize test repo: %v", err)
	}

	// Change to temp directory
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}
	defer func() { _ = os.Chdir(originalDir) }()

	// Create and stage a file
	testFile := filepath.Join(tempDir, "custom.txt")
	if err := os.WriteFile(testFile, []byte("Custom message test\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	if err := runGitCommandInDir(tempDir, "add", "custom.txt"); err != nil {
		t.Fatalf("failed to stage file: %v", err)
	}

	// Test with custom message
	commitTool := NewGitCommit(nil) // No LLM needed for custom message
	customMessage := "fix: resolve custom issue\n\nThis is a custom commit message."

	result, err := commitTool.Execute(context.Background(), ports.ToolCall{
		ID:   "test-custom-1",
		Name: "git_commit",
		Arguments: map[string]any{
			"message": customMessage,
			"auto":    true,
		},
	})

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.Error != nil {
		t.Fatalf("Execute() result error = %v", result.Error)
	}

	// Verify the commit has the custom message
	cmd := exec.Command("git", "log", "-1", "--pretty=format:%B")
	cmd.Dir = tempDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to check git log: %v", err)
	}

	commitMsg := string(output)
	if !strings.Contains(commitMsg, "fix: resolve custom issue") {
		t.Errorf("expected custom message in commit, got %q", commitMsg)
	}

	if !strings.Contains(commitMsg, "Generated with ALEX") {
		t.Errorf("expected ALEX footer in commit, got %q", commitMsg)
	}
}

// Helper functions

func initTestRepo(t *testing.T, dir string) error {
	t.Helper()

	commands := [][]string{
		{"init"},
		{"config", "user.name", "Test User"},
		{"config", "user.email", "test@example.com"},
		{"commit", "--allow-empty", "-m", "Initial commit"},
	}

	for _, args := range commands {
		if err := runGitCommandInDir(dir, args...); err != nil {
			return err
		}
	}

	return nil
}

func runGitCommandInDir(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %v failed: %s", args, string(output))
	}
	return nil
}
