package supervisor

import (
	"context"
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

// ComponentState represents the state of a supervised component.
type ComponentState string

const (
	ComponentUp       ComponentState = "up"
	ComponentDown     ComponentState = "down"
	ComponentCooldown ComponentState = "cooldown"
)

// Component represents a supervised process.
type Component struct {
	Name     string
	StartFn  func(ctx context.Context) error
	StopFn   func(ctx context.Context) error
	HealthFn func() string // returns "healthy", "down", "alive", etc.
	PIDFile  string
	SHAFile  string // file containing deployed SHA
}

// LoopState holds observed devops loop state from state files.
type LoopState struct {
	CyclePhase       string
	CycleResult      string
	LastError        string
	MainSHA          string
	LastProcessedSHA string
	LastValidatedSHA string
}

// Supervisor manages multiple supervised components with restart policies.
type Supervisor struct {
	components   []*Component
	policy       *RestartPolicy
	statusFile   *StatusFile
	logFile      string
	ticker       *time.Ticker
	interval     time.Duration
	lockDir      string
	pidFile      string
	mainRoot     string
	testRoot     string
	logger       *slog.Logger
	failCounts   map[string]int
	restartLocks sync.Map // map[string]*sync.Mutex — per-component restart guard
	loopState    LoopState
	tmpDir       string
}

// Config holds supervisor configuration.
type Config struct {
	TickInterval       time.Duration
	RestartMaxInWindow int
	RestartWindow      time.Duration
	CooldownDuration   time.Duration
	MainRoot           string
	TestRoot           string
	PIDDir             string
	LogDir             string
	TmpDir             string
}

// New creates a new Supervisor.
func New(cfg Config) *Supervisor {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	policy := NewRestartPolicy(cfg.RestartMaxInWindow, cfg.RestartWindow, cfg.CooldownDuration)

	statusPath := filepath.Join(cfg.TmpDir, "lark-supervisor.status.json")
	statusFile := NewStatusFile(statusPath)

	return &Supervisor{
		policy:     policy,
		statusFile: statusFile,
		logFile:    filepath.Join(cfg.LogDir, "lark-supervisor.log"),
		interval:   cfg.TickInterval,
		lockDir:    filepath.Join(cfg.TmpDir, "lark-supervisor.lock"),
		pidFile:    filepath.Join(cfg.PIDDir, "lark-supervisor.pid"),
		mainRoot:   cfg.MainRoot,
		testRoot:   cfg.TestRoot,
		logger:     logger,
		failCounts: make(map[string]int),
		tmpDir:     cfg.TmpDir,
	}
}

// RegisterComponent adds a component to be supervised.
func (s *Supervisor) RegisterComponent(comp *Component) {
	s.components = append(s.components, comp)
}

// Run starts the supervisor tick loop. Blocks until context is cancelled.
func (s *Supervisor) Run(ctx context.Context) error {
	// Ensure directories
	if err := os.MkdirAll(filepath.Dir(s.pidFile), 0o755); err != nil {
		return fmt.Errorf("create supervisor pid dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.logFile), 0o755); err != nil {
		return fmt.Errorf("create supervisor log dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.lockDir), 0o755); err != nil {
		return fmt.Errorf("create supervisor lock dir: %w", err)
	}

	// Acquire lock
	if err := os.Mkdir(s.lockDir, 0o755); err != nil {
		return fmt.Errorf("supervisor already running (lock: %s)", s.lockDir)
	}
	// Write lock owner
	ownerFile := filepath.Join(s.lockDir, "owner")
	if err := os.WriteFile(ownerFile, []byte(fmt.Sprintf("pid=%d started_at=%s\n",
		os.Getpid(), time.Now().UTC().Format(time.RFC3339))), 0o644); err != nil {
		return fmt.Errorf("write supervisor lock owner: %w", err)
	}

	defer s.cleanup()

	// Write PID file
	if err := os.WriteFile(s.pidFile, []byte(strconv.Itoa(os.Getpid())), 0o644); err != nil {
		return fmt.Errorf("write supervisor pid file: %w", err)
	}

	s.logger.Info("supervisor started",
		"tick", s.interval,
		"window", s.policy.WindowDuration,
		"max", s.policy.MaxInWindow,
		"cooldown", s.policy.CooldownDuration)

	s.ticker = time.NewTicker(s.interval)
	defer s.ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.stopAll(ctx)
			return nil
		case <-s.ticker.C:
			s.tick(ctx)
		}
	}
}

// tick runs a single supervision cycle:
//  1. Observe loop state
//  2. Health-check restarts
//  3. SHA drift auto-upgrade
//  4. Re-observe + write status
func (s *Supervisor) tick(ctx context.Context) {
	// 1. Observe
	s.readLoopState()

	// 2. Health-check restarts
	now := time.Now()
	for _, comp := range s.components {
		healthState := comp.HealthFn()

		if !s.needsRestart(comp.Name, healthState) {
			s.failCounts[comp.Name] = 0
			continue
		}

		if s.policy.InCooldown(comp.Name, now) {
			continue
		}

		// Exponential backoff
		s.failCounts[comp.Name]++
		delay := 1 << (s.failCounts[comp.Name] - 1)
		if delay > 60 {
			delay = 60
		}

		count := s.policy.RecordRestart(comp.Name)
		if count >= s.policy.MaxInWindow {
			s.policy.EnterCooldown("")
			s.logger.Warn("restart storm detected, entering cooldown",
				"component", comp.Name,
				"count", count,
				"window", s.policy.WindowDuration)
			continue
		}

		mu := s.componentMu(comp.Name)
		if !mu.TryLock() {
			s.logger.Info("restart in progress, skipping", "component", comp.Name)
			continue
		}

		s.logger.Info("restarting component",
			"component", comp.Name,
			"health", healthState,
			"delay", delay,
			"attempt", s.failCounts[comp.Name],
			"window_count", count)

		s.restartComponentAfterDelay(ctx, comp, mu, time.Duration(delay)*time.Second)
	}

	// 3. SHA drift auto-upgrade
	s.maybeUpgradeForSHADrift(ctx)

	// 4. Re-observe + write status
	s.readLoopState()
	s.writeStatus()
}

// maybeUpgradeForSHADrift auto-restarts healthy components whose deployed
// SHA differs from the latest main SHA. Corresponds to supervisor.sh
// maybe_upgrade_for_sha_drift.
func (s *Supervisor) maybeUpgradeForSHADrift(ctx context.Context) {
	now := time.Now()

	// Skip during global cooldown
	if s.policy.InCooldown("", now) {
		return
	}

	mainSHA := s.loopState.MainSHA
	if mainSHA == "" || mainSHA == "unknown" {
		return
	}

	for _, comp := range s.components {
		// Keep runtime components that should track main SHA aligned.
		if comp.Name != "main" {
			continue
		}

		health := comp.HealthFn()
		if health != "healthy" {
			continue
		}

		// Skip during per-component cooldown
		if s.policy.InCooldown(comp.Name, now) {
			continue
		}

		// Read deployed SHA
		if comp.SHAFile == "" {
			continue
		}
		shaData, err := os.ReadFile(comp.SHAFile)
		if err != nil {
			continue
		}
		deployedSHA := strings.TrimSpace(string(shaData))
		if deployedSHA == "" || deployedSHA == "unknown" {
			continue
		}

		if deployedSHA == mainSHA {
			continue
		}

		// SHA differs — record restart and check limits
		count := s.policy.RecordRestart(comp.Name)
		if count >= s.policy.MaxInWindow {
			s.policy.EnterCooldown(comp.Name)
			s.logger.Warn("upgrade storm detected, entering cooldown",
				"component", comp.Name,
				"count", count)
			continue
		}

		mu := s.componentMu(comp.Name)
		if !mu.TryLock() {
			continue
		}

		s.logger.Info("upgrading component for SHA drift",
			"component", comp.Name,
			"deployed", truncSHA(deployedSHA),
			"latest", truncSHA(mainSHA))

		if err := comp.StartFn(ctx); err != nil {
			s.logger.Error("upgrade restart failed",
				"component", comp.Name,
				"error", err)
		} else {
			s.logger.Info("upgrade restart succeeded", "component", comp.Name)
		}
		mu.Unlock()
	}
}

func (s *Supervisor) needsRestart(name, healthState string) bool {
	switch name {
	case "main":
		return healthState != "healthy"
	case "loop":
		return healthState != "alive"
	default:
		return healthState == "down"
	}
}

func (s *Supervisor) restartComponentAfterDelay(ctx context.Context, comp *Component, mu *sync.Mutex, delay time.Duration) {
	go func() {
		defer mu.Unlock()

		if delay > 0 {
			timer := time.NewTimer(delay)
			defer timer.Stop()

			select {
			case <-ctx.Done():
				return
			case <-timer.C:
			}
		}

		if err := comp.StartFn(ctx); err != nil {
			s.logger.Error("restart failed",
				"component", comp.Name,
				"error", err)
			return
		}

		s.logger.Info("restart succeeded", "component", comp.Name)
	}()
}

func (s *Supervisor) stopAll(ctx context.Context) {
	for _, comp := range s.components {
		if comp.StopFn != nil {
			if err := comp.StopFn(ctx); err != nil {
				s.logger.Error("failed to stop component",
					"component", comp.Name,
					"error", err)
			}
		}
	}
}

func (s *Supervisor) cleanup() {
	os.RemoveAll(s.lockDir)
	// Only remove PID file if it's ours
	data, err := os.ReadFile(s.pidFile)
	if err == nil {
		pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
		if pid == os.Getpid() {
			os.Remove(s.pidFile)
		}
	}
}

// readLoopState reads devops loop state from files in tmpDir.
// Corresponds to supervisor.sh observe_states (lines 482-502).
// All reads are graceful — missing files leave fields at zero value.
func (s *Supervisor) readLoopState() {
	var ls LoopState

	// Read loop state JSON (cycle_phase, cycle_result, last_error)
	stateFile := filepath.Join(s.tmpDir, "lark-loop.state.json")
	if data, err := os.ReadFile(stateFile); err == nil {
		var raw map[string]json.RawMessage
		if json.Unmarshal(data, &raw) == nil {
			if v, ok := raw["cycle_phase"]; ok {
				var s string
				if json.Unmarshal(v, &s) == nil {
					ls.CyclePhase = s
				}
			}
			if v, ok := raw["cycle_result"]; ok {
				var s string
				if json.Unmarshal(v, &s) == nil {
					ls.CycleResult = s
				}
			}
			if v, ok := raw["last_error"]; ok {
				var s string
				if json.Unmarshal(v, &s) == nil {
					ls.LastError = s
				}
			}
		}
	}

	// Read last processed SHA
	lastFile := filepath.Join(s.tmpDir, "lark-loop.last")
	if data, err := os.ReadFile(lastFile); err == nil {
		ls.LastProcessedSHA = strings.TrimSpace(string(data))
	}

	// Read last validated SHA
	validatedFile := filepath.Join(s.tmpDir, "lark-loop.last-validated")
	if data, err := os.ReadFile(validatedFile); err == nil {
		ls.LastValidatedSHA = strings.TrimSpace(string(data))
	}

	// Read main SHA from git
	ls.MainSHA = s.getMainSHA()

	s.loopState = ls
}

func (s *Supervisor) writeStatus() {
	now := time.Now()
	status := Status{
		Timestamp:          now.UTC().Format(time.RFC3339),
		Mode:               s.currentMode(now),
		Components:         make(map[string]ComponentStatus),
		RestartCountWindow: s.policy.TotalRestartCount(now),
		CyclePhase:         s.loopState.CyclePhase,
		CycleResult:        s.loopState.CycleResult,
		LastError:          s.loopState.LastError,
		MainSHA:            s.loopState.MainSHA,
		LastProcessedSHA:   s.loopState.LastProcessedSHA,
		LastValidatedSHA:   s.loopState.LastValidatedSHA,
	}

	for _, comp := range s.components {
		pid := 0
		if pidData, err := os.ReadFile(comp.PIDFile); err == nil {
			pid, _ = strconv.Atoi(strings.TrimSpace(string(pidData)))
		}
		sha := ""
		if shaData, err := os.ReadFile(comp.SHAFile); err == nil {
			sha = strings.TrimSpace(string(shaData))
		}
		status.Components[comp.Name] = ComponentStatus{
			PID:         pid,
			Health:      comp.HealthFn(),
			DeployedSHA: sha,
			RunsWindow:  s.policy.RestartCount(comp.Name, now),
		}
	}

	if err := s.statusFile.Write(status); err != nil {
		s.logger.Error("failed to write status", "error", err)
	}
}

func (s *Supervisor) currentMode(now time.Time) string {
	if s.policy.InCooldown("", now) {
		return "cooldown"
	}
	allHealthy := true
	for _, comp := range s.components {
		health := comp.HealthFn()
		if comp.Name == "loop" {
			if health != "alive" {
				allHealthy = false
			}
		} else {
			if health != "healthy" {
				allHealthy = false
			}
		}
	}
	if allHealthy {
		return "healthy"
	}
	return "degraded"
}

func (s *Supervisor) getMainSHA() string {
	cmd := exec.Command("git", "-C", s.mainRoot, "rev-parse", "main")
	// Prevent git from traversing above mainRoot to a parent repository.
	cmd.Env = append(os.Environ(), "GIT_CEILING_DIRECTORIES="+filepath.Dir(s.mainRoot))
	out, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

// Start launches the supervisor in the background.
func (s *Supervisor) Start(ctx context.Context) error {
	// Check if already running
	if data, err := os.ReadFile(s.pidFile); err == nil {
		pidStr := strings.TrimSpace(string(data))
		if pid, err := strconv.Atoi(pidStr); err == nil {
			if proc, err := os.FindProcess(pid); err == nil {
				// kill -0 to check if process exists
				if err := proc.Signal(os.Signal(nil)); err == nil {
					return fmt.Errorf("supervisor already running (PID: %d)", pid)
				}
			}
		}
	}

	// Clean stale lock
	if info, err := os.Stat(s.lockDir); err == nil && info.IsDir() {
		ownerFile := filepath.Join(s.lockDir, "owner")
		if data, err := os.ReadFile(ownerFile); err == nil {
			// Try to extract PID from owner file
			for _, line := range strings.Split(string(data), "\n") {
				if strings.HasPrefix(line, "pid=") {
					pidStr := strings.TrimPrefix(line, "pid=")
					pidStr = strings.Split(pidStr, " ")[0]
					if pid, err := strconv.Atoi(pidStr); err == nil {
						if proc, _ := os.FindProcess(pid); proc != nil {
							if proc.Signal(os.Signal(nil)) != nil {
								// Process not running, clean lock
								os.RemoveAll(s.lockDir)
							}
						}
					}
				}
			}
		}
	}

	return s.Run(ctx)
}

// componentMu returns the per-component mutex, creating it on first access.
func (s *Supervisor) componentMu(name string) *sync.Mutex {
	v, _ := s.restartLocks.LoadOrStore(name, &sync.Mutex{})
	return v.(*sync.Mutex)
}

// StatusReport returns the current status for display.
func (s *Supervisor) StatusReport() (Status, error) {
	return s.statusFile.Read()
}

func truncSHA(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}
