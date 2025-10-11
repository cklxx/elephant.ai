package builtin

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
)

// ensureGitRepo verifies the current directory is inside a git repository.
func ensureGitRepo(ctx context.Context) error {
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git CLI not installed. Install from https://git-scm.com/")
	}

	if _, err := runGitCommand(ctx, "rev-parse", "--is-inside-work-tree"); err != nil {
		return fmt.Errorf("not a git repository: %w", err)
	}
	return nil
}

// ensureGhCLI verifies that the GitHub CLI is installed and reachable.
func ensureGhCLI(ctx context.Context) error {
	if _, err := exec.LookPath("gh"); err != nil {
		return fmt.Errorf("gh CLI not installed. Install from https://cli.github.com/")
	}

	if _, err := runGhCommand(ctx, "--version"); err != nil {
		return fmt.Errorf("gh CLI not functional: %w", err)
	}
	return nil
}

// runGitCommand executes a git command and returns the trimmed combined output.
func runGitCommand(ctx context.Context, args ...string) (string, error) {
	return runCommand(ctx, true, "git", args...)
}

// runGitCommandRaw executes a git command and returns the raw combined output without trimming.
func runGitCommandRaw(ctx context.Context, args ...string) (string, error) {
	return runCommand(ctx, false, "git", args...)
}

// runGhCommand executes a gh command and returns the trimmed combined output.
func runGhCommand(ctx context.Context, args ...string) (string, error) {
	return runCommand(ctx, true, "gh", args...)
}

func runCommand(ctx context.Context, trim bool, binary string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Env = mergeEnv(os.Environ(), map[string]string{
		"GIT_PAGER":           "cat",
		"GIT_TERMINAL_PROMPT": "0",
		"GIT_SSH_COMMAND":     "ssh -oBatchMode=yes",
		"GH_PROMPT_DISABLED":  "1",
		"GH_PAGER":            "cat",
		"NO_COLOR":            "1",
	})
	output, err := cmd.CombinedOutput()
	result := string(output)
	if err != nil {
		cleaned := strings.TrimSpace(result)
		if cleaned != "" {
			return "", fmt.Errorf("%s %s failed: %s", binary, strings.Join(args, " "), cleaned)
		}
		return "", fmt.Errorf("%s %s failed: %w", binary, strings.Join(args, " "), err)
	}
	if trim {
		return strings.TrimSpace(result), nil
	}
	return result, nil
}

func mergeEnv(base []string, overrides map[string]string) []string {
	env := make(map[string]string, len(base)+len(overrides))
	for _, entry := range base {
		if idx := strings.Index(entry, "="); idx != -1 {
			key := entry[:idx]
			value := entry[idx+1:]
			env[key] = value
		}
	}

	for key, value := range overrides {
		env[key] = value
	}

	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	merged := make([]string, 0, len(env))
	for _, key := range keys {
		merged = append(merged, fmt.Sprintf("%s=%s", key, env[key]))
	}

	return merged
}
