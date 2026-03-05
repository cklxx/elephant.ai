package process

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	proclive "alex/internal/infra/process"
)

// Shutdown timeout hierarchy (keep in sync with scripts/lib/common/process.sh):
//   - Process-level SIGTERM grace period: 5s
//   - Service-level shutdown: 10s
//   - Orchestrator total: 30s
const (
	gracePeriod       = 5 * time.Second
	gracePollInterval = 250 * time.Millisecond
)

// ManagedProcess represents a process tracked by the manager.
type ManagedProcess struct {
	Name      string
	PIDFile   string
	MetaFile  string
	LogFile   string
	Cmd       *exec.Cmd
	PID       int
	PGID      int
	StartedAt time.Time

	logHandle *os.File
	handle    proclive.ProcessHandle // non-nil when started via Controller
}

// Manager tracks running processes with PID files and process groups.
type Manager struct {
	pidDir    string
	logDir    string
	ctrl      *proclive.Controller // optional; enables tmux-backed process management
	processes map[string]*ManagedProcess
	mu        sync.Mutex
}

// NewManager creates a new process manager.
func NewManager(pidDir, logDir string) *Manager {
	return &Manager{
		pidDir:    pidDir,
		logDir:    logDir,
		processes: make(map[string]*ManagedProcess),
	}
}

// WithController sets the unified process controller, enabling tmux-backed
// process management for human observability (tmux -L elephant attach).
func (m *Manager) WithController(ctrl *proclive.Controller) {
	m.ctrl = ctrl
}

// Start launches a command and tracks it. When a Controller is configured
// and tmux is available, the process runs inside a tmux session for human
// observability (tmux -L elephant attach -t elephant-dev-<name>).
func (m *Manager) Start(ctx context.Context, name string, cmd *exec.Cmd) (*ManagedProcess, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := EnsureRuntimeDirs(m.pidDir, m.logDir); err != nil {
		return nil, err
	}

	logFile := filepath.Join(m.logDir, name+".log")

	// Try tmux-backed start via Controller when available.
	if m.ctrl != nil && m.ctrl.TmuxAvailable() {
		return m.startViaTmux(ctx, name, cmd, logFile)
	}

	return m.startDirect(ctx, name, cmd, logFile)
}

// startViaTmux launches the process inside a tmux session via Controller.
func (m *Manager) startViaTmux(ctx context.Context, name string, cmd *exec.Cmd, logFile string) (*ManagedProcess, error) {
	env := make(map[string]string)
	for _, entry := range cmd.Env {
		if idx := strings.IndexByte(entry, '='); idx >= 0 {
			env[entry[:idx]] = entry[idx+1:]
		}
	}

	cfg := proclive.ProcessConfig{
		Name:       "dev-" + name,
		Command:    cmd.Path,
		Args:       cmd.Args[1:], // cmd.Args[0] is the command itself
		Env:        env,
		WorkingDir: cmd.Dir,
	}

	h, err := m.ctrl.StartTmux(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("start %s via tmux: %w", name, err)
	}

	pid := h.PID()
	identity, idErr := processCommandLine(pid)
	if idErr != nil || identity == "" {
		identity = commandIdentityFromCmd(cmd)
	}

	mp := &ManagedProcess{
		Name:      name,
		PIDFile:   filepath.Join(m.pidDir, name+".pid"),
		MetaFile:  pidMetaFile(filepath.Join(m.pidDir, name+".pid")),
		LogFile:   logFile,
		Cmd:       cmd,
		PID:       pid,
		StartedAt: time.Now(),
		handle:    h,
	}

	if err := writePIDState(mp.PIDFile, mp.MetaFile, pid, identity); err != nil {
		_ = h.Stop()
		return nil, fmt.Errorf("write pid state for %s: %w", name, err)
	}
	m.processes[name] = mp

	go func() {
		<-h.Done()
		removePIDFiles := false
		m.mu.Lock()
		if current := m.processes[name]; current == mp {
			delete(m.processes, name)
			removePIDFiles = true
		}
		m.mu.Unlock()
		if removePIDFiles {
			cleanupPIDState(mp.PIDFile, mp.MetaFile)
		}
	}()

	return mp, nil
}

// startDirect launches the process via os/exec directly (legacy path).
func (m *Manager) startDirect(_ context.Context, name string, cmd *exec.Cmd, logFile string) (*ManagedProcess, error) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true

	var logHandle *os.File
	if cmd.Stdout == nil {
		f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return nil, fmt.Errorf("open log file: %w", err)
		}
		cmd.Stdout = f
		cmd.Stderr = f
		logHandle = f
	}

	if err := cmd.Start(); err != nil {
		if logHandle != nil {
			_ = logHandle.Close()
		}
		return nil, fmt.Errorf("start %s: %w", name, err)
	}

	pid := cmd.Process.Pid
	pgid, _ := syscall.Getpgid(pid)
	identity, err := processCommandLine(pid)
	if err != nil || identity == "" {
		identity = commandIdentityFromCmd(cmd)
	}

	mp := &ManagedProcess{
		Name:      name,
		PIDFile:   filepath.Join(m.pidDir, name+".pid"),
		MetaFile:  pidMetaFile(filepath.Join(m.pidDir, name+".pid")),
		LogFile:   logFile,
		Cmd:       cmd,
		PID:       pid,
		PGID:      pgid,
		StartedAt: time.Now(),
		logHandle: logHandle,
	}

	if err := writePIDState(mp.PIDFile, mp.MetaFile, pid, identity); err != nil {
		_ = cmd.Process.Kill()
		if logHandle != nil {
			_ = logHandle.Close()
		}
		return nil, fmt.Errorf("write pid state for %s: %w", name, err)
	}
	m.processes[name] = mp

	go func() {
		_ = cmd.Wait()
		if mp.logHandle != nil {
			_ = mp.logHandle.Close()
		}

		removePIDFiles := false
		m.mu.Lock()
		if current := m.processes[name]; current == mp {
			delete(m.processes, name)
			removePIDFiles = true
		}
		m.mu.Unlock()
		if removePIDFiles {
			cleanupPIDState(mp.PIDFile, mp.MetaFile)
		}
	}()

	return mp, nil
}

// Stop stops a process by name with graceful shutdown.
func (m *Manager) Stop(_ context.Context, name string) error {
	m.mu.Lock()
	mp, tracked := m.processes[name]
	m.mu.Unlock()

	// Tmux-backed processes: delegate to handle.Stop().
	if tracked && mp.handle != nil {
		err := mp.handle.Stop()
		cleanupPIDState(mp.PIDFile, mp.MetaFile)
		return err
	}

	if tracked && mp.Cmd != nil && mp.Cmd.Process != nil {
		return m.killProcess(mp.PGID, mp.PID, mp.PIDFile)
	}

	pidFile := filepath.Join(m.pidDir, name+".pid")
	metaFile := pidMetaFile(pidFile)
	pid, err := readPIDFile(pidFile)
	if err != nil {
		cleanupPIDState(pidFile, metaFile)
		return nil
	}
	if !proclive.IsAlive(pid) {
		cleanupPIDState(pidFile, metaFile)
		return nil
	}
	if !identityMatches(metaFile, pid) {
		cleanupPIDState(pidFile, metaFile)
		return nil
	}

	pgid, err := syscall.Getpgid(pid)
	if err != nil {
		pgid = pid
	}

	return m.killProcess(pgid, pid, pidFile)
}

// StopAll stops all tracked processes.
func (m *Manager) StopAll(_ context.Context) error {
	m.mu.Lock()
	names := make([]string, 0, len(m.processes))
	for name := range m.processes {
		names = append(names, name)
	}
	m.mu.Unlock()

	var lastErr error
	for _, name := range names {
		if err := m.Stop(context.Background(), name); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// IsRunning checks if a named process is alive.
func (m *Manager) IsRunning(name string) (bool, int) {
	m.mu.Lock()
	mp, tracked := m.processes[name]
	m.mu.Unlock()

	// Tmux-backed processes.
	if tracked && mp.handle != nil {
		if mp.handle.Alive() {
			return true, mp.PID
		}
		return false, 0
	}

	if tracked && mp.Cmd != nil && mp.Cmd.Process != nil {
		if proclive.IsAlive(mp.PID) {
			return true, mp.PID
		}
		return false, 0
	}

	pidFile := filepath.Join(m.pidDir, name+".pid")
	metaFile := pidMetaFile(pidFile)
	pid, err := readPIDFile(pidFile)
	if err != nil {
		return false, 0
	}
	if !proclive.IsAlive(pid) {
		cleanupPIDState(pidFile, metaFile)
		return false, 0
	}
	if !identityMatches(metaFile, pid) {
		cleanupPIDState(pidFile, metaFile)
		return false, 0
	}
	if proclive.IsAlive(pid) {
		return true, pid
	}
	cleanupPIDState(pidFile, metaFile)
	return false, 0
}

// Recover attempts to recover process tracking from a PID file.
func (m *Manager) Recover(name string) (*ManagedProcess, error) {
	pidFile := filepath.Join(m.pidDir, name+".pid")
	metaFile := pidMetaFile(pidFile)
	pid, err := readPIDFile(pidFile)
	if err != nil {
		return nil, fmt.Errorf("read pid file for %s: %w", name, err)
	}
	if !proclive.IsAlive(pid) {
		cleanupPIDState(pidFile, metaFile)
		return nil, fmt.Errorf("process %s (pid %d) not running", name, pid)
	}
	if !identityMatches(metaFile, pid) {
		cleanupPIDState(pidFile, metaFile)
		return nil, fmt.Errorf("process %s (pid %d) identity mismatch", name, pid)
	}

	pgid, _ := syscall.Getpgid(pid)
	mp := &ManagedProcess{
		Name:     name,
		PIDFile:  pidFile,
		MetaFile: metaFile,
		LogFile:  filepath.Join(m.logDir, name+".log"),
		PID:      pid,
		PGID:     pgid,
	}

	m.mu.Lock()
	m.processes[name] = mp
	m.mu.Unlock()

	return mp, nil
}

func (m *Manager) killProcess(pgid, pid int, pidFile string) error {
	metaFile := pidMetaFile(pidFile)

	target := pgid
	if target == 0 {
		target = pid
	}

	proclive.GracefulStop(target, nil, proclive.ShutdownPolicy{
		Grace:           gracePeriod,
		PollInterval:    gracePollInterval,
		UseProcessGroup: pgid != 0,
	})

	cleanupPIDState(pidFile, metaFile)
	return nil
}

func isProcessAlive(pid int) bool {
	return proclive.IsAlive(pid)
}

func readPIDFile(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	firstLine := strings.SplitN(strings.TrimSpace(string(data)), "\n", 2)[0]
	firstLine = strings.TrimPrefix(strings.TrimSpace(firstLine), "pid=")
	return strconv.Atoi(firstLine)
}

type pidMetadata struct {
	Command string `json:"command"`
}

func pidMetaFile(pidFile string) string {
	return pidFile + ".meta"
}

func writePIDState(pidFile, metaFile string, pid int, identity string) error {
	if err := atomicWriteFile(pidFile, []byte(strconv.Itoa(pid))); err != nil {
		return err
	}
	if strings.TrimSpace(identity) == "" {
		return nil
	}
	return writePIDMetadata(metaFile, identity)
}

func writePIDMetadata(path, identity string) error {
	meta := pidMetadata{Command: normalizeCommandLine(identity)}
	data, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	return atomicWriteFile(path, data)
}

func readPIDMetadata(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var meta pidMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return "", err
	}
	return normalizeCommandLine(meta.Command), nil
}

func cleanupPIDState(pidFile, metaFile string) {
	_ = os.Remove(pidFile)
	_ = os.Remove(metaFile)
}

func identityMatches(metaFile string, pid int) bool {
	actual, err := processCommandLine(pid)
	if err != nil {
		return false
	}

	expected, err := readPIDMetadata(metaFile)
	if err != nil {
		// Legacy PID files had no metadata. Adopt identity to avoid duplicate starts.
		_ = writePIDMetadata(metaFile, actual)
		return true
	}

	return normalizeCommandLine(expected) == normalizeCommandLine(actual)
}

func processCommandLine(pid int) (string, error) {
	out, err := exec.Command("ps", "-ww", "-o", "command=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return "", err
	}
	line := normalizeCommandLine(string(out))
	if line == "" {
		return "", fmt.Errorf("empty command line for pid %d", pid)
	}
	return line, nil
}

func commandIdentityFromCmd(cmd *exec.Cmd) string {
	if cmd == nil {
		return ""
	}
	if len(cmd.Args) > 0 {
		return normalizeCommandLine(strings.Join(cmd.Args, " "))
	}
	return normalizeCommandLine(cmd.Path)
}

func normalizeCommandLine(command string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(command)), " ")
}

// OrphanProcess represents a process found in PID files that is either dead
// or has a mismatched identity (PID recycled to a different process).
type OrphanProcess struct {
	Name    string
	PIDFile string
	PID     int
	Reason  string // "dead", "identity_mismatch", "untracked"
}

// ScanOrphans scans the PID directory for orphan PID files — processes that
// are either dead or have recycled PIDs. Returns the list of orphans found.
func (m *Manager) ScanOrphans() []OrphanProcess {
	entries, err := os.ReadDir(m.pidDir)
	if err != nil {
		return nil
	}

	m.mu.Lock()
	trackedNames := make(map[string]bool, len(m.processes))
	for name := range m.processes {
		trackedNames[name] = true
	}
	m.mu.Unlock()

	var orphans []OrphanProcess
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".pid") {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".pid")
		pidFile := filepath.Join(m.pidDir, entry.Name())
		metaFile := pidMetaFile(pidFile)

		pid, err := readPIDFile(pidFile)
		if err != nil {
			orphans = append(orphans, OrphanProcess{
				Name:    name,
				PIDFile: pidFile,
				PID:     0,
				Reason:  "unreadable",
			})
			continue
		}

		if !proclive.IsAlive(pid) {
			orphans = append(orphans, OrphanProcess{
				Name:    name,
				PIDFile: pidFile,
				PID:     pid,
				Reason:  "dead",
			})
			continue
		}

		if !trackedNames[name] && !identityMatches(metaFile, pid) {
			orphans = append(orphans, OrphanProcess{
				Name:    name,
				PIDFile: pidFile,
				PID:     pid,
				Reason:  "identity_mismatch",
			})
		}
	}

	return orphans
}

// CleanupOrphans removes PID files for dead/mismatched processes.
// Returns the number of cleaned up entries.
func (m *Manager) CleanupOrphans() int {
	orphans := m.ScanOrphans()
	count := 0
	for _, o := range orphans {
		metaFile := pidMetaFile(o.PIDFile)
		cleanupPIDState(o.PIDFile, metaFile)
		count++
	}
	return count
}

func atomicWriteFile(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
