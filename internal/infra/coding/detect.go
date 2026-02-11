package coding

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var detectLookPath = exec.LookPath
var detectUserHomeDir = os.UserHomeDir

// LocalCLIDetection captures startup-time local coding CLI availability.
type LocalCLIDetection struct {
	ID             string
	Binary         string
	Path           string
	AgentType      string
	AdapterSupport bool
}

type cliCandidate struct {
	id        string
	agentType string
	binaries  []string
	supported bool
}

var defaultCLICandidates = []cliCandidate{
	{id: "codex", agentType: "codex", binaries: []string{"codex"}, supported: true},
	{id: "claude", agentType: "claude_code", binaries: []string{"claude", "claude-code"}, supported: true},
	{id: "kimi", binaries: []string{"kimi", "kimi-cli", "k2"}, supported: false},
}

// DetectLocalCLIs reports available local coding CLIs, including unsupported
// binaries for visibility (e.g. kimi).
func DetectLocalCLIs() []LocalCLIDetection {
	detected := make([]LocalCLIDetection, 0, len(defaultCLICandidates))
	for _, candidate := range defaultCLICandidates {
		path, binary, ok := detectFirstBinary(candidate.binaries)
		if !ok {
			continue
		}
		detected = append(detected, LocalCLIDetection{
			ID:             candidate.id,
			Binary:         binary,
			Path:           path,
			AgentType:      candidate.agentType,
			AdapterSupport: candidate.supported,
		})
	}
	return detected
}

// DetectLocalAdapters reports locally available coding adapters that are
// currently wired into the external-agent execution path.
func DetectLocalAdapters() []string {
	available := make([]string, 0, 2)
	for _, item := range DetectLocalCLIs() {
		if !item.AdapterSupport || strings.TrimSpace(item.AgentType) == "" {
			continue
		}
		available = append(available, item.AgentType)
	}
	return available
}

func detectFirstBinary(binaries []string) (path string, binary string, ok bool) {
	for _, name := range binaries {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		resolved, err := resolveLocalBinaryPath(trimmed)
		if err == nil {
			return resolved, trimmed, true
		}
	}
	return "", "", false
}

func resolveLocalBinaryPath(binary string) (string, error) {
	if filepath.IsAbs(binary) {
		if isExecutableFile(binary) {
			return binary, nil
		}
		return "", os.ErrNotExist
	}
	if path, err := detectLookPath(binary); err == nil {
		return path, nil
	}
	for _, dir := range fallbackCLIPaths() {
		candidate := filepath.Join(dir, binary)
		if isExecutableFile(candidate) {
			return candidate, nil
		}
	}
	return "", os.ErrNotExist
}

func fallbackCLIPaths() []string {
	dirs := []string{
		"/usr/local/bin",
		"/opt/homebrew/bin",
	}
	home, err := detectUserHomeDir()
	if err == nil && strings.TrimSpace(home) != "" {
		dirs = append([]string{
			filepath.Join(home, ".local", "bin"),
			filepath.Join(home, ".bun", "bin"),
			filepath.Join(home, ".npm", "bin"),
		}, dirs...)
	}
	return dirs
}

func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Mode()&0o111 != 0
}
