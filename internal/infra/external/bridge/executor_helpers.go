package bridge

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"alex/internal/shared/executioncontrol"
)

func (e *Executor) resolvePython() string {
	if e.cfg.PythonBinary != "" {
		return e.cfg.PythonBinary
	}
	if script := e.resolveBridgeScript(); script != "" {
		scriptDir := filepath.Dir(script)
		venvPython := filepath.Join(scriptDir, ".venv", "bin", "python3")
		if _, err := os.Stat(venvPython); err == nil {
			return venvPython
		}
		// Venv missing or broken — try auto-provisioning via setup.sh.
		if provisioned := e.ensureVenv(scriptDir); provisioned != "" {
			return provisioned
		}
	}
	return "python3"
}

// ensureVenv runs the setup.sh script in scriptDir to create the venv.
// Returns the venv python3 path on success, or empty string on failure.
func (e *Executor) ensureVenv(scriptDir string) string {
	setupScript := filepath.Join(scriptDir, "setup.sh")
	if _, err := os.Stat(setupScript); err != nil {
		return ""
	}
	e.logger.Info("Bridge venv missing, auto-provisioning via setup.sh", "dir", scriptDir)
	cmd := exec.Command("bash", setupScript)
	cmd.Dir = scriptDir
	if out, err := cmd.CombinedOutput(); err != nil {
		e.logger.Error("Bridge venv auto-setup failed", "err", err, "output", string(out))
		return ""
	}
	venvPython := filepath.Join(scriptDir, ".venv", "bin", "python3")
	if _, err := os.Stat(venvPython); err == nil {
		e.logger.Info("Bridge venv auto-provisioned successfully", "python", venvPython)
		return venvPython
	}
	return ""
}

func (e *Executor) resolveBridgeScript() string {
	if e.cfg.BridgeScript != "" {
		if abs, err := filepath.Abs(e.cfg.BridgeScript); err == nil {
			return abs
		}
		return e.cfg.BridgeScript
	}
	// Resolve relative to the running binary's directory.
	scriptDir := e.defaultScriptDir()
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "..", "scripts", scriptDir, scriptDir+".py")
		if abs, err := filepath.Abs(candidate); err == nil {
			if _, err := os.Stat(abs); err == nil {
				return abs
			}
		}
	}
	// Fallback: resolve relative path to absolute from CWD so it works
	// regardless of the subprocess WorkingDir.
	rel := filepath.Join("scripts", scriptDir, scriptDir+".py")
	if abs, err := filepath.Abs(rel); err == nil {
		return abs
	}
	return rel
}

// defaultScriptDir returns the bridge script directory name for the agent type.
func (e *Executor) defaultScriptDir() string {
	switch e.cfg.AgentType {
	case "kimi":
		return "kimi_bridge"
	case "codex":
		return "codex_bridge"
	default:
		return "cc_bridge"
	}
}

// maybeAppendAuthHint appends an agent-specific authentication hint when
// stderr output suggests authentication failure.
func (e *Executor) maybeAppendAuthHint(msg string, stderrTail string) string {
	switch e.cfg.AgentType {
	case "claude_code":
		if !containsAny(stderrTail, []string{"not logged", "unauthorized"}) {
			return msg
		}
		return fmt.Sprintf("%s Hint: ensure the Claude CLI is logged in (e.g. run `claude login`).", msg)
	case "codex":
		if !containsAny(stderrTail, []string{"api key", "unauthorized", "authentication"}) {
			return msg
		}
		return fmt.Sprintf("%s Hint: ensure Codex has a valid login or API key configured.", msg)
	case "kimi":
		if !containsAny(stderrTail, []string{"api key", "unauthorized", "authentication", "token"}) {
			return msg
		}
		return fmt.Sprintf("%s Hint: ensure Kimi CLI has valid authentication configured.", msg)
	default:
		return msg
	}
}

// validateWorktreePolicy returns an error if the working directory is on the
// main branch and not inside a git worktree. This enforces the worktree-based
// development workflow: agents must not write to files directly on main.
func validateWorktreePolicy(workDir string) error {
	if workDir == "" {
		workDir = "."
	}

	// Determine current branch.
	branchCmd := exec.Command("git", "-C", workDir, "branch", "--show-current")
	branchOut, err := branchCmd.Output()
	if err != nil {
		// Not a git repo or git not available — skip enforcement.
		return nil
	}
	branch := strings.TrimSpace(string(branchOut))
	if branch != "main" {
		return nil
	}

	// Check if we're in a worktree (git-dir differs from git-common-dir).
	gitDirCmd := exec.Command("git", "-C", workDir, "rev-parse", "--git-dir")
	gitDirOut, err := gitDirCmd.Output()
	if err != nil {
		return nil
	}
	gitCommonCmd := exec.Command("git", "-C", workDir, "rev-parse", "--git-common-dir")
	gitCommonOut, err := gitCommonCmd.Output()
	if err != nil {
		return nil
	}

	gitDir := strings.TrimSpace(string(gitDirOut))
	gitCommon := strings.TrimSpace(string(gitCommonOut))

	// Resolve to absolute for reliable comparison.
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(workDir, gitDir)
	}
	if !filepath.IsAbs(gitCommon) {
		gitCommon = filepath.Join(workDir, gitCommon)
	}
	gitDir = filepath.Clean(gitDir)
	gitCommon = filepath.Clean(gitCommon)

	if gitDir != gitCommon {
		// Inside a worktree — allow.
		return nil
	}

	return fmt.Errorf("worktree policy: refusing to execute on main branch (not a worktree). " +
		"Create a worktree first: git worktree add -b <branch> ../<dir> main")
}

// --- Helpers ---

func pickString(config map[string]string, key string, fallback string) string {
	if config == nil {
		return fallback
	}
	if val := strings.TrimSpace(config[key]); val != "" {
		return val
	}
	return fallback
}

func pickInt(config map[string]string, key string, fallback int) int {
	if config == nil {
		return fallback
	}
	if val := strings.TrimSpace(config[key]); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			return parsed
		}
	}
	return fallback
}

func pickFloat(config map[string]string, key string, fallback float64) float64 {
	if config == nil {
		return fallback
	}
	if val := strings.TrimSpace(config[key]); val != "" {
		if parsed, err := strconv.ParseFloat(val, 64); err == nil {
			return parsed
		}
	}
	return fallback
}

func splitList(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func normalizeExecutionMode(raw string, cfg map[string]string) string {
	mode := strings.TrimSpace(raw)
	if mode == "" && cfg != nil {
		mode = cfg["execution_mode"]
	}
	return executioncontrol.NormalizeExecutionMode(mode)
}

func normalizeAutonomyLevel(raw string, cfg map[string]string) string {
	level := strings.TrimSpace(raw)
	if level == "" && cfg != nil {
		level = cfg["autonomy_level"]
	}
	return executioncontrol.NormalizeAutonomyLevel(level)
}

func formatProcessError(agentName string, err error, stderrTail string) string {
	name := strings.TrimSpace(agentName)
	if name == "" {
		name = "external agent"
	}
	msg := fmt.Sprintf("%s exited: %v", name, err)
	if detail := exitDetail(err); detail != "" {
		msg = fmt.Sprintf("%s (%s)", msg, detail)
	}
	if tail := compactTail(stderrTail, 400); tail != "" {
		msg = fmt.Sprintf("%s | stderr tail: %s", msg, tail)
	}
	return msg
}

func containsAny(input string, needles []string) bool {
	lower := strings.ToLower(input)
	for _, needle := range needles {
		if needle == "" {
			continue
		}
		if strings.Contains(lower, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func compactTail(tail string, limit int) string {
	trimmed := strings.TrimSpace(tail)
	if trimmed == "" {
		return ""
	}
	compact := strings.Join(strings.Fields(trimmed), " ")
	if limit > 0 && len(compact) > limit {
		return compact[:limit]
	}
	return compact
}

type exitCoder interface {
	ExitCode() int
}

func exitDetail(err error) string {
	if err == nil {
		return ""
	}
	detail := ""
	var exitErr exitCoder
	if errors.As(err, &exitErr) {
		if code := exitErr.ExitCode(); code >= 0 {
			detail = fmt.Sprintf("exit=%d", code)
		}
	}
	if execErr := new(exec.ExitError); errors.As(err, &execErr) {
		if status, ok := execErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			if detail == "" {
				detail = fmt.Sprintf("signal=%s", status.Signal())
			} else {
				detail = fmt.Sprintf("%s signal=%s", detail, status.Signal())
			}
		}
	}
	return detail
}
