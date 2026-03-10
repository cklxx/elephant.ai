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
	CyclePhase       string `json:"cycle_phase"`
	CycleResult      string `json:"cycle_result"`
	LastError        string `json:"last_error"`
	MainSHA          string `json:"-"`
	LastProcessedSHA string `json:"-"`
	LastValidatedSHA string `json:"-"`
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
	mu           sync.Mutex     // protects failCounts, lastUpgradeAt, loopState
	failCounts   map[string]int
	restartLocks sync.Map // map[string]*sync.Mutex — per-component restart guard
	loopState    LoopState
	tmpDir       string

	// lastUpgradeAt tracks the last SHA drift upgrade time per component to
	// enforce a minimum interval between upgrades, preventing rapid restart
	// storms when commits land in quick succession.
	lastUpgradeAt map[string]time.Time
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
		policy:        policy,
		statusFile:    statusFile,
		logFile:       filepath.Join(cfg.LogDir, "lark-supervisor.log"),
		interval:      cfg.TickInterval,
		lockDir:       filepath.Join(cfg.TmpDir, "lark-supervisor.lock"),
		pidFile:       filepath.Join(cfg.PIDDir, "lark-supervisor.pid"),
		mainRoot:      cfg.MainRoot,
		testRoot:      cfg.TestRoot,
		logger:        logger,
		failCounts:    make(map[string]int),
		tmpDir:        cfg.TmpDir,
		lastUpgradeAt: make(map[string]time.Time),
	}
}

// RegisterComponent adds a component to be supervised.
// Must be called before Run starts the tick loop.
func (s *Supervisor) RegisterComponent(comp *Component) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.components = append(s.components, comp)
}

// Run starts the supervisor tick loop. Blocks until context is cancelled.
func (s *Supervisor) Run(ctx context.Context) error {
	if err := s.ensureDirs(); err != nil {
		return err
	}

	if err := s.acquireLock(); err != nil {
		return err
	}
	defer s.cleanup()

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
	s.readLoopState()
	s.healthCheckRestarts(ctx)
	s.maybeUpgradeForSHADrift(ctx)
	s.readLoopState()
	s.writeStatus()
}

// minUpgradeInterval is the minimum time between SHA drift upgrades for a
// component. This prevents rapid restart storms when multiple commits land
// on main in quick succession (e.g. from the devops loop).
const minUpgradeInterval = 5 * time.Minute

// healthCheckRestarts checks each component and restarts unhealthy ones.
func (s *Supervisor) healthCheckRestarts(ctx context.Context) {
	now := time.Now()
	for _, comp := range s.components {
		healthState := comp.HealthFn()

		if !s.needsRestart(comp.Name, healthState) {
			s.mu.Lock()
			s.failCounts[comp.Name] = 0
			s.mu.Unlock()
			continue
		}

		if s.policy.InCooldown(comp.Name, now) {
			continue
		}

		s.mu.Lock()
		s.failCounts[comp.Name]++
		delay := 1 << (s.failCounts[comp.Name] - 1)
		if delay > 60 {
			delay = 60
		}
		attempt := s.failCounts[comp.Name]
		s.mu.Unlock()

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
			"attempt", attempt,
			"window_count", count)

		s.restartComponentAfterDelay(ctx, comp, mu, time.Duration(delay)*time.Second)
	}
}

// maybeUpgradeForSHADrift auto-restarts healthy components whose deployed
// SHA differs from the latest main SHA.
func (s *Supervisor) maybeUpgradeForSHADrift(ctx context.Context) {
	now := time.Now()
	if s.policy.InCooldown("", now) {
		return
	}

	s.mu.Lock()
	mainSHA := s.loopState.MainSHA
	s.mu.Unlock()
	if mainSHA == "" || mainSHA == "unknown" {
		return
	}

	for _, comp := range s.components {
		if comp.Name != "main" || comp.HealthFn() != "healthy" {
			continue
		}
		if s.policy.InCooldown(comp.Name, now) {
			continue
		}
		if s.withinUpgradeInterval(comp.Name, now) {
			continue
		}

		deployedSHA := readFileString(comp.SHAFile)
		if deployedSHA == "" || deployedSHA == "unknown" || deployedSHA == mainSHA {
			continue
		}

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
			s.mu.Lock()
			s.lastUpgradeAt[comp.Name] = now
			s.mu.Unlock()
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

// readLoopState reads devops loop state from files in tmpDir.
// All reads are graceful — missing files leave fields at zero value.
func (s *Supervisor) readLoopState() {
	var ls LoopState

	stateFile := filepath.Join(s.tmpDir, "lark-loop.state.json")
	if data, err := os.ReadFile(stateFile); err == nil {
		_ = json.Unmarshal(data, &ls)
	}

	ls.LastProcessedSHA = readFileString(filepath.Join(s.tmpDir, "lark-loop.last"))
	ls.LastValidatedSHA = readFileString(filepath.Join(s.tmpDir, "lark-loop.last-validated"))
	ls.MainSHA = s.getMainSHA()

	s.mu.Lock()
	s.loopState = ls
	s.mu.Unlock()
}

func (s *Supervisor) writeStatus() {
	now := time.Now()

	s.mu.Lock()
	ls := s.loopState
	s.mu.Unlock()

	status := Status{
		Timestamp:          now.UTC().Format(time.RFC3339),
		Mode:               s.currentMode(now),
		Components:         make(map[string]ComponentStatus),
		RestartCountWindow: s.policy.TotalRestartCount(now),
		CyclePhase:         ls.CyclePhase,
		CycleResult:        ls.CycleResult,
		LastError:          ls.LastError,
		MainSHA:            ls.MainSHA,
		LastProcessedSHA:   ls.LastProcessedSHA,
		LastValidatedSHA:   ls.LastValidatedSHA,
	}

	for _, comp := range s.components {
		status.Components[comp.Name] = ComponentStatus{
			PID:         readFilePID(comp.PIDFile),
			Health:      comp.HealthFn(),
			DeployedSHA: readFileString(comp.SHAFile),
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
	for _, comp := range s.components {
		if s.needsRestart(comp.Name, comp.HealthFn()) {
			return "degraded"
		}
	}
	return "healthy"
}

func (s *Supervisor) getMainSHA() string {
	cmd := exec.Command("git", "-C", s.mainRoot, "rev-parse", "main")
	cmd.Env = append(os.Environ(), "GIT_CEILING_DIRECTORIES="+filepath.Dir(s.mainRoot))
	out, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

// Start launches the supervisor, cleaning stale locks if needed.
func (s *Supervisor) Start(ctx context.Context) error {
	if err := s.checkAlreadyRunning(); err != nil {
		return err
	}
	s.cleanStaleLock()
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

// --- helpers ---

func (s *Supervisor) ensureDirs() error {
	for _, dir := range []string{
		filepath.Dir(s.pidFile),
		filepath.Dir(s.logFile),
		filepath.Dir(s.lockDir),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create dir %s: %w", dir, err)
		}
	}
	return nil
}

func (s *Supervisor) acquireLock() error {
	if err := os.Mkdir(s.lockDir, 0o755); err != nil {
		return fmt.Errorf("supervisor already running (lock: %s)", s.lockDir)
	}
	ownerFile := filepath.Join(s.lockDir, "owner")
	return os.WriteFile(ownerFile, []byte(fmt.Sprintf("pid=%d started_at=%s\n",
		os.Getpid(), time.Now().UTC().Format(time.RFC3339))), 0o644)
}

func (s *Supervisor) cleanup() {
	os.RemoveAll(s.lockDir)
	if pid := readFilePID(s.pidFile); pid == os.Getpid() {
		os.Remove(s.pidFile)
	}
}

func (s *Supervisor) checkAlreadyRunning() error {
	pid := readFilePID(s.pidFile)
	if pid == 0 {
		return nil
	}
	if processAlive(pid) {
		return fmt.Errorf("supervisor already running (PID: %d)", pid)
	}
	return nil
}

func (s *Supervisor) cleanStaleLock() {
	ownerFile := filepath.Join(s.lockDir, "owner")
	data, err := os.ReadFile(ownerFile)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "pid=") {
			continue
		}
		pidStr := strings.Split(strings.TrimPrefix(line, "pid="), " ")[0]
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}
		if !processAlive(pid) {
			os.RemoveAll(s.lockDir)
		}
		return
	}
}

func (s *Supervisor) withinUpgradeInterval(name string, now time.Time) bool {
	s.mu.Lock()
	lastUpgrade, ok := s.lastUpgradeAt[name]
	s.mu.Unlock()
	return ok && now.Sub(lastUpgrade) < minUpgradeInterval
}

// readFileString reads a file and returns its trimmed content, or "" on error.
func readFileString(path string) string {
	if path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// readFilePID reads a PID from a file, returning 0 on error.
func readFilePID(path string) int {
	s := readFileString(path)
	if s == "" {
		return 0
	}
	pid, _ := strconv.Atoi(s)
	return pid
}

// processAlive reports whether a process with the given PID is running.
func processAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(os.Signal(nil)) == nil
}

func truncSHA(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}
