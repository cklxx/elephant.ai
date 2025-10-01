package builtin

import (
	"alex/internal/agent/ports"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type gitCommit struct {
	llmClient ports.LLMClient
}

// NewGitCommit creates a new git commit tool
func NewGitCommit(llmClient ports.LLMClient) ports.ToolExecutor {
	return &gitCommit{llmClient: llmClient}
}

func (t *gitCommit) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	// Check if we're in a git repository
	if err := t.validateGitRepo(ctx); err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: err}, nil
	}

	// Get custom message if provided
	customMessage, _ := call.Arguments["message"].(string)
	autoCommit, _ := call.Arguments["auto"].(bool)

	// Get git status to detect changes
	status, err := t.getGitStatus(ctx)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("failed to get git status: %w", err)}, nil
	}

	if status == "" {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "No changes to commit. Working directory is clean.",
		}, nil
	}

	// Get git diff for changed files
	diff, err := t.getGitDiff(ctx)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("failed to get git diff: %w", err)}, nil
	}

	if diff == "" {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "No staged changes to commit. Use 'git add' to stage files first.",
		}, nil
	}

	// Generate or use custom commit message
	var commitMessage string
	if customMessage != "" {
		commitMessage = customMessage
	} else {
		// Generate commit message using LLM
		commitMessage, err = t.generateCommitMessage(ctx, diff)
		if err != nil {
			return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("failed to generate commit message: %w", err)}, nil
		}
	}

	// Add ALEX footer
	commitMessage = t.addFooter(commitMessage)

	// If not auto-commit, return the message for approval
	if !autoCommit {
		output := fmt.Sprintf("Proposed commit message:\n\n%s\n\n", commitMessage)
		output += "Changed files:\n" + status + "\n\n"
		output += "Diff summary:\n" + t.summarizeDiff(diff) + "\n\n"
		output += "To commit, run: alex commit --auto --message \"<message>\""

		return &ports.ToolResult{
			CallID:  call.ID,
			Content: output,
			Metadata: map[string]any{
				"commit_message":    commitMessage,
				"status":            status,
				"requires_approval": true,
			},
		}, nil
	}

	// Execute the commit
	if err := t.executeCommit(ctx, commitMessage); err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("failed to commit: %w", err)}, nil
	}

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: fmt.Sprintf("Successfully committed changes:\n\n%s", commitMessage),
		Metadata: map[string]any{
			"commit_message": commitMessage,
			"status":         status,
		},
	}, nil
}

func (t *gitCommit) validateGitRepo(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--git-dir")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("not a git repository")
	}
	return nil
}

func (t *gitCommit) getGitStatus(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "status", "--short")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func (t *gitCommit) getGitDiff(ctx context.Context) (string, error) {
	// Try to get staged changes first
	cmd := exec.CommandContext(ctx, "git", "diff", "--cached")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}

	staged := strings.TrimSpace(string(output))

	// If no staged changes, get unstaged changes
	if staged == "" {
		cmd = exec.CommandContext(ctx, "git", "diff")
		output, err = cmd.CombinedOutput()
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(output)), nil
	}

	return staged, nil
}

func (t *gitCommit) generateCommitMessage(ctx context.Context, diff string) (string, error) {
	if t.llmClient == nil {
		return "", fmt.Errorf("LLM client not configured for commit message generation")
	}

	// Truncate diff if too long (keep first 4000 chars)
	if len(diff) > 4000 {
		diff = diff[:4000] + "\n... (diff truncated)"
	}

	prompt := fmt.Sprintf(`Analyze the following git diff and generate a concise conventional commit message.

Rules:
- Use conventional commit format: <type>: <description>
- Types: feat, fix, refactor, docs, test, chore, style
- Description should be imperative mood: "add" not "added"
- Keep the first line under 72 characters
- Add a detailed body (2-4 lines) if changes are complex
- Focus on WHY the change was made, not just WHAT changed

Diff:
%s

Generate only the commit message, no explanations or markdown formatting.`, diff)

	req := ports.CompletionRequest{
		Messages: []ports.Message{
			{Role: "user", Content: prompt},
		},
		Temperature: 0.3,
		MaxTokens:   500,
	}

	resp, err := t.llmClient.Complete(ctx, req)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(resp.Content), nil
}

func (t *gitCommit) addFooter(message string) string {
	footer := "\n\n🤖 Generated with ALEX\nCo-Authored-By: ALEX <noreply@alex.com>"

	// Only add footer if not already present
	if !strings.Contains(message, "Generated with ALEX") {
		return message + footer
	}
	return message
}

func (t *gitCommit) summarizeDiff(diff string) string {
	lines := strings.Split(diff, "\n")
	additions := 0
	deletions := 0
	filesChanged := make(map[string]bool)

	for _, line := range lines {
		if strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---") {
			// Extract filename
			parts := strings.Fields(line)
			if len(parts) > 1 {
				file := strings.TrimPrefix(parts[1], "b/")
				file = strings.TrimPrefix(file, "a/")
				if file != "/dev/null" {
					filesChanged[file] = true
				}
			}
		} else if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			additions++
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			deletions++
		}
	}

	summary := fmt.Sprintf("%d files changed, %d insertions(+), %d deletions(-)\n",
		len(filesChanged), additions, deletions)

	if len(filesChanged) > 0 {
		summary += "Files: "
		i := 0
		for file := range filesChanged {
			if i > 0 {
				summary += ", "
			}
			summary += file
			i++
			if i >= 5 { // Limit to 5 files
				if len(filesChanged) > 5 {
					summary += fmt.Sprintf(" and %d more", len(filesChanged)-5)
				}
				break
			}
		}
	}

	return summary
}

func (t *gitCommit) executeCommit(ctx context.Context, message string) error {
	cmd := exec.CommandContext(ctx, "git", "commit", "-m", message)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git commit failed: %s", string(output))
	}
	return nil
}

func (t *gitCommit) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name:        "git_commit",
		Description: "Create a git commit with AI-generated message following Conventional Commits format. Analyzes staged changes and generates appropriate commit message.",
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"message": {
					Type:        "string",
					Description: "Custom commit message (optional). If not provided, will generate using LLM.",
				},
				"auto": {
					Type:        "boolean",
					Description: "Auto-commit without approval (default: false). If false, shows proposed message for review.",
				},
			},
			Required: []string{},
		},
	}
}

func (t *gitCommit) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:      "git_commit",
		Version:   "1.0.0",
		Category:  "git",
		Tags:      []string{"git", "version-control", "commit"},
		Dangerous: true,
	}
}
