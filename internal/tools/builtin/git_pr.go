package builtin

import (
	"alex/internal/agent/ports"
	"context"
	"fmt"
	"strings"
)

type gitPR struct {
	llmClient ports.LLMClient
}

// NewGitPR creates a new git PR tool
func NewGitPR(llmClient ports.LLMClient) ports.ToolExecutor {
	return &gitPR{llmClient: llmClient}
}

func (t *gitPR) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	// Check if we're in a git repository
	if err := t.validateGitRepo(ctx); err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: err}, nil
	}

	// Check if gh CLI is installed
	if err := t.validateGHCLI(ctx); err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: err}, nil
	}

	// Get parameters
	customTitle, _ := call.Arguments["title"].(string)
	baseBranch, _ := call.Arguments["base"].(string)
	if baseBranch == "" {
		var err error
		baseBranch, err = t.getDefaultBranch(ctx)
		if err != nil {
			baseBranch = "main" // Fallback to main
		}
	}

	// Get current branch
	currentBranch, err := t.getCurrentBranch(ctx)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("failed to get current branch: %w", err)}, nil
	}

	if currentBranch == baseBranch {
		return &ports.ToolResult{
			CallID: call.ID,
			Error:  fmt.Errorf("cannot create PR from base branch %s to itself", baseBranch),
		}, nil
	}

	// Check if current branch is pushed to remote
	isPushed, err := t.isBranchPushed(ctx, currentBranch)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("failed to check remote branch: %w", err)}, nil
	}

	if !isPushed {
		// Push the current branch
		if err := t.pushBranch(ctx, currentBranch); err != nil {
			return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("failed to push branch: %w", err)}, nil
		}
	}

	// Get commit history
	commits, err := t.getCommitHistory(ctx, baseBranch, currentBranch)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("failed to get commit history: %w", err)}, nil
	}

	if commits == "" {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("No commits found between %s and %s", baseBranch, currentBranch),
		}, nil
	}

	// Get full diff
	diff, err := t.getFullDiff(ctx, baseBranch, currentBranch)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("failed to get diff: %w", err)}, nil
	}

	// Generate PR title and description
	var title, description string
	if customTitle != "" {
		title = customTitle
		description, err = t.generatePRDescription(ctx, commits, diff)
		if err != nil {
			return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("failed to generate PR description: %w", err)}, nil
		}
	} else {
		title, description, err = t.generatePRContent(ctx, commits, diff)
		if err != nil {
			return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("failed to generate PR content: %w", err)}, nil
		}
	}

	// Add ALEX footer
	description = t.addFooter(description)

	// Create PR using gh CLI
	prURL, err := t.createPR(ctx, title, description, baseBranch)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("failed to create PR: %w", err)}, nil
	}

	output := "Successfully created pull request!\n\n"
	output += fmt.Sprintf("Title: %s\n", title)
	output += fmt.Sprintf("Base: %s <- %s\n", baseBranch, currentBranch)
	output += fmt.Sprintf("URL: %s\n\n", prURL)
	output += fmt.Sprintf("Description:\n%s", description)

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: output,
		Metadata: map[string]any{
			"pr_url":  prURL,
			"title":   title,
			"base":    baseBranch,
			"head":    currentBranch,
			"commits": strings.Count(commits, "\n") + 1,
		},
	}, nil
}

func (t *gitPR) validateGitRepo(ctx context.Context) error {
	return ensureGitRepo(ctx)
}

func (t *gitPR) validateGHCLI(ctx context.Context) error {
	return ensureGhCLI(ctx)
}

func (t *gitPR) getDefaultBranch(ctx context.Context) (string, error) {
	output, err := runGitCommand(ctx, "symbolic-ref", "refs/remotes/origin/HEAD")
	if err != nil {
		return "", err
	}
	// Output is like "refs/remotes/origin/main"
	branch := strings.TrimSpace(output)
	branch = strings.TrimPrefix(branch, "refs/remotes/origin/")
	return branch, nil
}

func (t *gitPR) getCurrentBranch(ctx context.Context) (string, error) {
	output, err := runGitCommand(ctx, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return output, nil
}

func (t *gitPR) isBranchPushed(ctx context.Context, branch string) (bool, error) {
	output, err := runGitCommandRaw(ctx, "ls-remote", "--heads", "origin", branch)
	if err != nil {
		return false, err
	}
	return len(strings.TrimSpace(output)) > 0, nil
}

func (t *gitPR) pushBranch(ctx context.Context, branch string) error {
	_, err := runGitCommandRaw(ctx, "push", "-u", "origin", branch)
	return err
}

func (t *gitPR) getCommitHistory(ctx context.Context, base, head string) (string, error) {
	return runGitCommand(ctx, "log", fmt.Sprintf("%s..%s", base, head), "--oneline")
}

func (t *gitPR) getFullDiff(ctx context.Context, base, head string) (string, error) {
	return runGitCommand(ctx, "diff", fmt.Sprintf("%s...%s", base, head), "--stat")
}

func (t *gitPR) generatePRDescription(ctx context.Context, commits, diff string) (string, error) {
	if t.llmClient == nil {
		return "", fmt.Errorf("LLM client not configured for PR description generation")
	}

	// Truncate if too long
	if len(commits) > 2000 {
		commits = commits[:2000] + "\n... (truncated)"
	}
	if len(diff) > 2000 {
		diff = diff[:2000] + "\n... (truncated)"
	}

	prompt := fmt.Sprintf(`Generate a comprehensive GitHub Pull Request description from the following commits and diff.

Commits:
%s

Diff:
%s

Format as markdown with sections:
## Summary
(2-3 bullet points highlighting key changes)

## Changes
(detailed breakdown by component/file, organized logically)

## Test Plan
(how to verify these changes work correctly)

Be concise but thorough. Focus on the impact and purpose of changes.`, commits, diff)

	req := ports.CompletionRequest{
		Messages: []ports.Message{
			{Role: "user", Content: prompt},
		},
		Temperature: 0.4,
		MaxTokens:   1000,
	}

	resp, err := t.llmClient.Complete(ctx, req)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(resp.Content), nil
}

func (t *gitPR) generatePRContent(ctx context.Context, commits, diff string) (string, string, error) {
	if t.llmClient == nil {
		return "", "", fmt.Errorf("LLM client not configured for PR generation")
	}

	// Truncate if too long
	if len(commits) > 2000 {
		commits = commits[:2000] + "\n... (truncated)"
	}
	if len(diff) > 2000 {
		diff = diff[:2000] + "\n... (truncated)"
	}

	prompt := fmt.Sprintf(`Generate a GitHub Pull Request title and description from the following commits and diff.

Commits:
%s

Diff:
%s

Output format:
TITLE: <concise PR title under 80 characters>

DESCRIPTION:
## Summary
(2-3 bullet points highlighting key changes)

## Changes
(detailed breakdown by component/file)

## Test Plan
(how to verify these changes)

Be concise but thorough.`, commits, diff)

	req := ports.CompletionRequest{
		Messages: []ports.Message{
			{Role: "user", Content: prompt},
		},
		Temperature: 0.4,
		MaxTokens:   1200,
	}

	resp, err := t.llmClient.Complete(ctx, req)
	if err != nil {
		return "", "", err
	}

	// Parse title and description
	content := strings.TrimSpace(resp.Content)
	parts := strings.SplitN(content, "DESCRIPTION:", 2)

	var title, description string
	if len(parts) == 2 {
		titlePart := strings.TrimSpace(parts[0])
		title = strings.TrimPrefix(titlePart, "TITLE:")
		title = strings.TrimSpace(title)
		description = strings.TrimSpace(parts[1])
	} else {
		// Fallback: use first line as title, rest as description
		lines := strings.Split(content, "\n")
		if len(lines) > 0 {
			title = strings.TrimSpace(lines[0])
			if len(lines) > 1 {
				description = strings.TrimSpace(strings.Join(lines[1:], "\n"))
			}
		}
	}

	return title, description, nil
}

func (t *gitPR) addFooter(description string) string {
	footer := "\n\n---\n🤖 Generated with ALEX"

	// Only add footer if not already present
	if !strings.Contains(description, "Generated with ALEX") {
		return description + footer
	}
	return description
}

func (t *gitPR) createPR(ctx context.Context, title, description, base string) (string, error) {
	output, err := runGhCommand(ctx, "pr", "create",
		"--title", title,
		"--body", description,
		"--base", base,
	)
	if err != nil {
		return "", err
	}

	// Extract URL from output (gh CLI returns the PR URL)
	url := strings.TrimSpace(output)
	return url, nil
}

func (t *gitPR) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name:        "git_pr",
		Description: "Create a GitHub pull request with AI-generated title and description. Analyzes commit history and changes to generate comprehensive PR content.",
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"title": {
					Type:        "string",
					Description: "Custom PR title (optional). If not provided, will generate using LLM.",
				},
				"base": {
					Type:        "string",
					Description: "Base branch to merge into (default: auto-detect main/master).",
				},
			},
			Required: []string{},
		},
	}
}

func (t *gitPR) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:      "git_pr",
		Version:   "1.0.0",
		Category:  "git",
		Tags:      []string{"git", "github", "pull-request", "pr"},
		Dangerous: true,
	}
}
