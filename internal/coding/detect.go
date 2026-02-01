package coding

import "os/exec"

// DetectLocalAdapters reports locally available coding agent CLIs.
func DetectLocalAdapters() []string {
	available := []string{}
	if _, err := exec.LookPath("codex"); err == nil {
		available = append(available, "codex")
	}
	if _, err := exec.LookPath("claude"); err == nil {
		available = append(available, "claude_code")
	}
	return available
}
