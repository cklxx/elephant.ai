package coding

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"alex/internal/shared/utils"
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
	{id: "kimi", agentType: "kimi", binaries: []string{"kimi", "kimi-cli", "k2"}, supported: true},
}

// DetectLocalCLIs reports available local coding CLIs discovered on the host.
// Binary lookups run in parallel to reduce wall-clock time.
func DetectLocalCLIs() []LocalCLIDetection {
	type indexedResult struct {
		det LocalCLIDetection
		ok  bool
		idx int
	}
	ch := make(chan indexedResult, len(defaultCLICandidates))
	for i, c := range defaultCLICandidates {
		go func(idx int, cand cliCandidate) {
			path, binary, ok := detectFirstBinary(cand.binaries)
			if ok {
				ch <- indexedResult{
					det: LocalCLIDetection{
						ID:             cand.id,
						Binary:         binary,
						Path:           path,
						AgentType:      cand.agentType,
						AdapterSupport: cand.supported,
					},
					ok:  true,
					idx: idx,
				}
			} else {
				ch <- indexedResult{idx: idx}
			}
		}(i, c)
	}
	results := make(map[int]indexedResult, len(defaultCLICandidates))
	for range defaultCLICandidates {
		r := <-ch
		results[r.idx] = r
	}
	detected := make([]LocalCLIDetection, 0, len(defaultCLICandidates))
	for i := range defaultCLICandidates {
		if r, ok := results[i]; ok && r.ok {
			detected = append(detected, r.det)
		}
	}
	return detected
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
	if err == nil && utils.HasContent(home) {
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
