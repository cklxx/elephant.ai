package supervisor

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// AutofixStateFileData corresponds to the state JSON written by autofix.sh.
type AutofixStateFileData struct {
	State           string `json:"autofix_state"`
	IncidentID      string `json:"autofix_incident_id"`
	LastReason      string `json:"autofix_last_reason"`
	LastStartedAt   string `json:"autofix_last_started_at"`
	LastFinishedAt  string `json:"autofix_last_finished_at"`
	LastCommit      string `json:"autofix_last_commit"`
	RestartRequired string `json:"autofix_restart_required"`
}

// AutofixConfig holds autofix runner configuration.
type AutofixConfig struct {
	Enabled       bool
	Trigger       string // "cooldown"
	Timeout       time.Duration
	MaxInWindow   int
	Window        time.Duration
	Cooldown      time.Duration
	Scope         string // "repo"
	ScriptPath    string // path to autofix.sh
	HistoryFile   string
	SignatureFile string
	AppliedFile   string
	StateFile     string
	LockDir       string
}

// AutofixRunner manages autofix triggering and history.
type AutofixRunner struct {
	config        AutofixConfig
	logger        *slog.Logger
	history       []time.Time
	cooldownUntil time.Time
	mu            sync.Mutex
}

// NewAutofixRunner creates a new autofix runner.
func NewAutofixRunner(cfg AutofixConfig, logger *slog.Logger) *AutofixRunner {
	r := &AutofixRunner{
		config: cfg,
		logger: logger,
	}
	r.loadHistory()
	return r
}

// TryTrigger attempts to trigger an autofix run.
func (r *AutofixRunner) TryTrigger(component, reason, mainSHA string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.config.Enabled || r.config.Trigger != "cooldown" {
		return false
	}

	if _, err := os.Stat(r.config.ScriptPath); err != nil {
		r.logger.Warn("autofix script not found", "path", r.config.ScriptPath)
		return false
	}

	now := time.Now()

	// Check autofix cooldown
	if now.Before(r.cooldownUntil) {
		return false
	}

	// Check runs in window
	r.pruneHistory(now)
	if len(r.history) >= r.config.MaxInWindow {
		r.cooldownUntil = now.Add(r.config.Cooldown)
		r.logger.Info("autofix run limit reached, entering cooldown",
			"runs", len(r.history),
			"max", r.config.MaxInWindow,
			"cooldown", r.config.Cooldown)
		return false
	}

	// Check signature dedup
	signature := fmt.Sprintf("%s|%s|%s", component, reason, mainSHA)
	if prev := r.readSignature(); prev == signature {
		r.logger.Info("autofix duplicate signature, skip", "sig", signature)
		return false
	}

	// Check lock
	if r.isLocked() {
		r.logger.Info("autofix already running (locked)")
		return false
	}

	// Record and trigger
	incidentID := fmt.Sprintf("afx-%s-%s-%s",
		now.UTC().Format("20060102T150405Z"),
		component,
		truncSHA(mainSHA))

	r.writeSignature(signature)
	r.history = append(r.history, now)
	r.saveHistory()

	r.logger.Info("triggering autofix",
		"incident", incidentID,
		"reason", reason)

	// Launch in background
	go r.runAutofix(incidentID, reason, signature, mainSHA)

	return true
}

// RunsInWindow returns the number of autofix runs in the current window.
func (r *AutofixRunner) RunsInWindow() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pruneHistory(time.Now())
	return len(r.history)
}

// State returns the current autofix state string.
func (r *AutofixRunner) State() string {
	if r.isLocked() {
		return "running"
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if time.Now().Before(r.cooldownUntil) {
		return "cooldown"
	}
	return "idle"
}

// ReadStateFile reads the autofix state file written by autofix.sh.
// Returns zero-value data and nil error if the file does not exist.
func (r *AutofixRunner) ReadStateFile() (AutofixStateFileData, error) {
	data, err := os.ReadFile(r.config.StateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return AutofixStateFileData{}, nil
		}
		return AutofixStateFileData{}, fmt.Errorf("read autofix state: %w", err)
	}
	var state AutofixStateFileData
	if err := json.Unmarshal(data, &state); err != nil {
		return AutofixStateFileData{}, fmt.Errorf("parse autofix state: %w", err)
	}
	return state, nil
}

func (r *AutofixRunner) runAutofix(incidentID, reason, signature, mainSHA string) {
	cmd := exec.Command(r.config.ScriptPath, "trigger",
		"--incident-id", incidentID,
		"--reason", reason,
		"--signature", signature,
		"--main-sha", mainSHA)

	cmd.Env = append(os.Environ(),
		fmt.Sprintf("LARK_SUPERVISOR_AUTOFIX_TIMEOUT_SECONDS=%d", int(r.config.Timeout.Seconds())),
		fmt.Sprintf("LARK_SUPERVISOR_AUTOFIX_SCOPE=%s", r.config.Scope))

	out, err := cmd.CombinedOutput()
	if err != nil {
		r.logger.Error("autofix failed",
			"incident", incidentID,
			"error", err,
			"output", string(out))
	} else {
		r.logger.Info("autofix completed", "incident", incidentID)
	}
}

func (r *AutofixRunner) pruneHistory(now time.Time) {
	cutoff := now.Add(-r.config.Window)
	pruned := r.history[:0]
	for _, t := range r.history {
		if !t.Before(cutoff) {
			pruned = append(pruned, t)
		}
	}
	r.history = pruned
}

func (r *AutofixRunner) loadHistory() {
	data, err := os.ReadFile(r.config.HistoryFile)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if epoch, err := strconv.ParseInt(line, 10, 64); err == nil {
			r.history = append(r.history, time.Unix(epoch, 0))
		}
	}
	r.pruneHistory(time.Now())
}

func (r *AutofixRunner) saveHistory() {
	var lines []string
	for _, t := range r.history {
		lines = append(lines, strconv.FormatInt(t.Unix(), 10))
	}
	if err := os.MkdirAll(filepath.Dir(r.config.HistoryFile), 0o755); err != nil {
		r.logger.Warn("create autofix history dir failed", "path", r.config.HistoryFile, "error", err)
		return
	}
	if err := os.WriteFile(r.config.HistoryFile, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		r.logger.Warn("write autofix history failed", "path", r.config.HistoryFile, "error", err)
	}
}

func (r *AutofixRunner) readSignature() string {
	data, err := os.ReadFile(r.config.SignatureFile)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func (r *AutofixRunner) writeSignature(sig string) {
	if err := os.MkdirAll(filepath.Dir(r.config.SignatureFile), 0o755); err != nil {
		r.logger.Warn("create autofix signature dir failed", "path", r.config.SignatureFile, "error", err)
		return
	}
	if err := os.WriteFile(r.config.SignatureFile, []byte(sig+"\n"), 0o644); err != nil {
		r.logger.Warn("write autofix signature failed", "path", r.config.SignatureFile, "error", err)
	}
}

func (r *AutofixRunner) isLocked() bool {
	info, err := os.Stat(r.config.LockDir)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func truncSHA(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}
