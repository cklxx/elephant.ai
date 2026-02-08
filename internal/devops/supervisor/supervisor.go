package supervisor

import (
	"context"
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
	ComponentAutofix  ComponentState = "autofix"
)

// Component represents a supervised process.
type Component struct {
	Name     string
	StartFn func(ctx context.Context) error
	StopFn  func(ctx context.Context) error
	HealthFn func() string // returns "healthy", "down", "alive", etc.
	PIDFile  string
	SHAFile  string // file containing deployed SHA
}

// Supervisor manages multiple supervised components with restart policies.
type Supervisor struct {
	components   []*Component
	policy       *RestartPolicy
	autofix      *AutofixRunner
	statusFile   *StatusFile
	logFile      string
	ticker       *time.Ticker
	interval     time.Duration
	lockDir      string
	pidFile      string
	mainRoot     string
	testRoot     string
	logger       *slog.Logger
	mu           sync.Mutex
	failCounts   map[string]int
	restartLocks sync.Map // map[string]*sync.Mutex — per-component restart guard
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
	AutofixConfig      AutofixConfig
}

// New creates a new Supervisor.
func New(cfg Config) *Supervisor {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	policy := NewRestartPolicy(cfg.RestartMaxInWindow, cfg.RestartWindow, cfg.CooldownDuration)

	statusPath := filepath.Join(cfg.TmpDir, "lark-supervisor.status.json")
	statusFile := NewStatusFile(statusPath)

	autofix := NewAutofixRunner(cfg.AutofixConfig, logger)

	return &Supervisor{
		policy:     policy,
		autofix:    autofix,
		statusFile: statusFile,
		logFile:    filepath.Join(cfg.LogDir, "lark-supervisor.log"),
		interval:   cfg.TickInterval,
		lockDir:    filepath.Join(cfg.TmpDir, "lark-supervisor.lock"),
		pidFile:    filepath.Join(cfg.PIDDir, "lark-supervisor.pid"),
		mainRoot:   cfg.MainRoot,
		testRoot:   cfg.TestRoot,
		logger:     logger,
		failCounts: make(map[string]int),
	}
}

// RegisterComponent adds a component to be supervised.
func (s *Supervisor) RegisterComponent(comp *Component) {
	s.components = append(s.components, comp)
}

// Run starts the supervisor tick loop. Blocks until context is cancelled.
func (s *Supervisor) Run(ctx context.Context) error {
	// Ensure directories
	os.MkdirAll(filepath.Dir(s.pidFile), 0o755)
	os.MkdirAll(filepath.Dir(s.logFile), 0o755)
	os.MkdirAll(filepath.Dir(s.lockDir), 0o755)

	// Acquire lock
	if err := os.Mkdir(s.lockDir, 0o755); err != nil {
		return fmt.Errorf("supervisor already running (lock: %s)", s.lockDir)
	}
	// Write lock owner
	ownerFile := filepath.Join(s.lockDir, "owner")
	os.WriteFile(ownerFile, []byte(fmt.Sprintf("pid=%d started_at=%s\n",
		os.Getpid(), time.Now().UTC().Format(time.RFC3339))), 0o644)

	defer s.cleanup()

	// Write PID file
	os.WriteFile(s.pidFile, []byte(strconv.Itoa(os.Getpid())), 0o644)

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

func (s *Supervisor) tick(ctx context.Context) {
	now := time.Now()

	for _, comp := range s.components {
		healthState := comp.HealthFn()

		if !s.needsRestart(comp.Name, healthState) {
			s.failCounts[comp.Name] = 0
			continue
		}

		if !s.policy.ShouldRestart(comp.Name, now) {
			if s.policy.InCooldown(comp.Name, now) {
				// Trigger autofix during cooldown
				mainSHA := s.getMainSHA()
				s.autofix.TryTrigger(comp.Name,
					fmt.Sprintf("restart storm: %s", comp.Name),
					mainSHA)
			}
			continue
		}

		// Exponential backoff
		s.failCounts[comp.Name]++
		delay := 1 << (s.failCounts[comp.Name] - 1)
		if delay > 60 {
			delay = 60
		}

		count := s.policy.RecordRestart(comp.Name)
		if count > s.policy.MaxInWindow {
			s.policy.EnterCooldown("")
			s.logger.Warn("restart storm detected, entering cooldown",
				"component", comp.Name,
				"count", count,
				"window", s.policy.WindowDuration)
			mainSHA := s.getMainSHA()
			s.autofix.TryTrigger(comp.Name,
				fmt.Sprintf("restart storm: %d restarts in %s", count, s.policy.WindowDuration),
				mainSHA)
			continue
		}

		// Acquire per-component lock to prevent overlapping restarts
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

		time.Sleep(time.Duration(delay) * time.Second)

		if err := comp.StartFn(ctx); err != nil {
			s.logger.Error("restart failed",
				"component", comp.Name,
				"error", err)
		} else {
			s.failCounts[comp.Name] = 0
			s.logger.Info("restart succeeded", "component", comp.Name)
		}
		mu.Unlock()
	}

	// Write status
	s.writeStatus()
}

func (s *Supervisor) needsRestart(name, healthState string) bool {
	switch name {
	case "main", "test":
		return healthState != "healthy"
	case "loop":
		return healthState != "alive"
	default:
		return healthState == "down"
	}
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

func (s *Supervisor) writeStatus() {
	now := time.Now()
	status := Status{
		Timestamp:          now.UTC().Format(time.RFC3339),
		Mode:               s.currentMode(now),
		Components:         make(map[string]ComponentStatus),
		RestartCountWindow: s.policy.TotalRestartCount(now),
		Autofix: AutofixStatus{
			State:      s.autofix.State(),
			RunsWindow: s.autofix.RunsInWindow(),
		},
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
