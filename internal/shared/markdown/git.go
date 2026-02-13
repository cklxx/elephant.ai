package markdown

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"alex/internal/shared/logging"
)

// CommitEntry represents a single git log entry.
type CommitEntry struct {
	Hash    string
	Date    time.Time
	Subject string
}

// gitOperations wraps git CLI calls for a specific directory.
type gitOperations struct {
	dir    string
	logger logging.Logger
}

func newGitOperations(dir string, logger logging.Logger) *gitOperations {
	return &gitOperations{dir: dir, logger: logging.OrNop(logger)}
}

// init initialises a git repository in the directory (idempotent).
func (g *gitOperations) init(ctx context.Context) error {
	if g.isRepo() {
		return nil
	}
	_, err := g.run(ctx, "init")
	return err
}

// isRepo returns true when the directory contains a .git folder.
func (g *gitOperations) isRepo() bool {
	info, err := os.Stat(filepath.Join(g.dir, ".git"))
	return err == nil && info.IsDir()
}

// add stages the given paths.
func (g *gitOperations) add(ctx context.Context, paths ...string) error {
	args := append([]string{"add", "--"}, paths...)
	_, err := g.run(ctx, args...)
	return err
}

// commit creates a commit with the given message.
// Returns an error if the commit fails (e.g. nothing to commit).
func (g *gitOperations) commit(ctx context.Context, msg string) error {
	_, err := g.run(ctx, "commit", "-m", msg)
	return err
}

// hasChanges reports whether the working tree has uncommitted changes.
func (g *gitOperations) hasChanges(ctx context.Context) (bool, error) {
	out, err := g.run(ctx, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

// log returns the N most recent commits for the given file.
// Format: hash ISO-date subject
// Returns an empty slice (no error) when the repo has no commits yet.
func (g *gitOperations) log(ctx context.Context, file string, n int) ([]CommitEntry, error) {
	args := []string{"log", "--format=%H %aI %s", fmt.Sprintf("-n%d", n)}
	if file != "" {
		args = append(args, "--", file)
	}
	out, err := g.run(ctx, args...)
	if err != nil {
		// Empty repo (no commits yet) is not an error.
		if strings.Contains(err.Error(), "does not have any commits") {
			return nil, nil
		}
		return nil, err
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return nil, nil
	}
	var entries []CommitEntry
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 3)
		if len(parts) < 2 {
			continue
		}
		hash := parts[0]
		date, _ := time.Parse(time.RFC3339, parts[1])
		subject := ""
		if len(parts) == 3 {
			subject = parts[2]
		}
		entries = append(entries, CommitEntry{Hash: hash, Date: date, Subject: subject})
	}
	return entries, nil
}

// run executes a git command in the store directory.
// It always sets user.name and user.email via -c flags to avoid
// depending on global git configuration.
func (g *gitOperations) run(ctx context.Context, args ...string) (string, error) {
	fullArgs := []string{
		"-C", g.dir,
		"-c", "user.name=elephant.ai",
		"-c", "user.email=kernel@elephant.ai",
	}
	fullArgs = append(fullArgs, args...)

	cmd := exec.CommandContext(ctx, "git", fullArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		stderrStr := strings.TrimSpace(stderr.String())
		g.logger.Debug("git %v failed: %s", args, stderrStr)
		return "", fmt.Errorf("git %s: %s", args[0], stderrStr)
	}
	return stdout.String(), nil
}
