package applescript

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Runner abstracts osascript execution for testability.
type Runner interface {
	RunScript(ctx context.Context, script string) (string, error)
}

// ExecRunner executes AppleScript via osascript.
type ExecRunner struct{}

func (ExecRunner) RunScript(ctx context.Context, script string) (string, error) {
	cmd := exec.CommandContext(ctx, "osascript", "-e", script)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = err.Error()
		}
		return "", fmt.Errorf("osascript: %s", errMsg)
	}
	return strings.TrimSpace(stdout.String()), nil
}

// escapeAppleScriptString escapes a Go string for safe interpolation into
// an AppleScript string literal (between double quotes). Handles backslashes,
// quotes, and control characters that could break the string boundary.
func escapeAppleScriptString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	return s
}

// isAppRunning checks whether a process with the given name is active.
func isAppRunning(ctx context.Context, r Runner, processName string) (bool, error) {
	script := fmt.Sprintf(
		`tell application "System Events" to (name of processes) contains "%s"`,
		escapeAppleScriptString(processName),
	)
	out, err := r.RunScript(ctx, script)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) == "true", nil
}
