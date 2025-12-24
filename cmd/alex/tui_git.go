package main

import (
	"os"
	"path/filepath"
	"strings"
)

// currentGitBranch returns the current git branch name, or empty if the working
// directory is not inside a git worktree/repository.
func currentGitBranch() string {
	gitPath := ".git"
	headPath := filepath.Join(gitPath, "HEAD")

	stat, err := os.Stat(gitPath)
	if err != nil {
		return ""
	}

	// Git worktrees store ".git" as a file containing the actual gitdir.
	if !stat.IsDir() {
		content, err := os.ReadFile(gitPath)
		if err != nil {
			return ""
		}

		// Format: "gitdir: <path>"
		line := strings.TrimSpace(string(content))
		if strings.HasPrefix(line, "gitdir:") {
			gitdir := strings.TrimSpace(strings.TrimPrefix(line, "gitdir:"))
			if gitdir != "" && !filepath.IsAbs(gitdir) {
				gitdir = filepath.Clean(filepath.Join(".", gitdir))
			}
			if gitdir != "" {
				headPath = filepath.Join(gitdir, "HEAD")
			}
		}
	}

	content, err := os.ReadFile(headPath)
	if err != nil {
		return ""
	}

	line := strings.TrimSpace(string(content))
	const prefix = "ref: refs/heads/"
	if strings.HasPrefix(line, prefix) {
		branch := strings.TrimPrefix(line, prefix)
		if len(branch) > 28 {
			branch = branch[:25] + "..."
		}
		return branch
	}

	// Detached head: show short commit if present.
	if line != "" {
		if len(line) > 12 {
			return line[:12]
		}
		return line
	}

	return ""
}
