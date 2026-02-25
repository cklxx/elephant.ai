// Package buildinfo provides build fingerprinting to skip unnecessary rebuilds.
package buildinfo

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Fingerprint represents a snapshot of the build inputs.
type Fingerprint struct {
	HeadSHA   string
	DiffHash  string
	Staged    string
}

// String returns a deterministic string representation.
func (f Fingerprint) String() string {
	return fmt.Sprintf("head=%s\ndiff=%s\nstaged=%s", f.HeadSHA, f.DiffHash, f.Staged)
}

// Compute computes the build fingerprint for a Go project at root.
// It captures HEAD SHA + working tree diff hash + staged diff hash.
func Compute(root string) Fingerprint {
	head := gitOutput(root, "rev-parse", "HEAD")
	diff := hashString(gitOutput(root, "diff", "--no-ext-diff", "--", "."))
	staged := hashString(gitOutput(root, "diff", "--cached", "--no-ext-diff", "--", "."))

	return Fingerprint{
		HeadSHA:  head,
		DiffHash: diff,
		Staged:   staged,
	}
}

// ReadStamp reads a previously written fingerprint stamp file.
func ReadStamp(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// WriteStamp writes a fingerprint to a stamp file.
func WriteStamp(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

// IsStale returns true if the current fingerprint differs from the stamp file.
func IsStale(stampPath string, current Fingerprint) bool {
	prev := ReadStamp(stampPath)
	if prev == "" {
		return true
	}
	return prev != current.String()
}

func gitOutput(dir string, args ...string) string {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func hashString(s string) string {
	if s == "" {
		return "empty"
	}
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h[:8])
}
