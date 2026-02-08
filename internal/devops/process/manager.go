package process

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// ManagedProcess represents a process tracked by the manager.
type ManagedProcess struct {
	Name      string
	PIDFile   string
	LogFile   string
	Cmd       *exec.Cmd
	PID       int
	PGID      int
	StartedAt time.Time
}

// Manager tracks running processes with PID files and process groups.
type Manager struct {
	pidDir    string
	logDir    string
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

// Start launches a command and tracks it.
func (m *Manager) Start(ctx context.Context, name string, cmd *exec.Cmd) (*ManagedProcess, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := os.MkdirAll(m.pidDir, 0o755); err != nil {
		return nil, fmt.Errorf("create pid dir: %w", err)
	}
	if err := os.MkdirAll(m.logDir, 0o755); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}

	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true

	logFile := filepath.Join(m.logDir, name+".log")
	if cmd.Stdout == nil {
		f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return nil, fmt.Errorf("open log file: %w", err)
		}
		cmd.Stdout = f
		cmd.Stderr = f
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", name, err)
	}

	pid := cmd.Process.Pid
	pgid, _ := syscall.Getpgid(pid)

	mp := &ManagedProcess{
		Name:      name,
		PIDFile:   filepath.Join(m.pidDir, name+".pid"),
		LogFile:   logFile,
		Cmd:       cmd,
		PID:       pid,
		PGID:      pgid,
		StartedAt: time.Now(),
	}

	_ = atomicWriteFile(mp.PIDFile, []byte(strconv.Itoa(pid)))
	m.processes[name] = mp

	go func() {
		_ = cmd.Wait()
		m.mu.Lock()
		delete(m.processes, name)
		m.mu.Unlock()
		os.Remove(mp.PIDFile)
	}()

	return mp, nil
}

// Stop stops a process by name with graceful shutdown.
func (m *Manager) Stop(_ context.Context, name string) error {
	m.mu.Lock()
	mp, tracked := m.processes[name]
	m.mu.Unlock()

	if tracked && mp.Cmd != nil && mp.Cmd.Process != nil {
		return m.killProcess(mp.PGID, mp.PID, mp.PIDFile)
	}

	pidFile := filepath.Join(m.pidDir, name+".pid")
	pid, err := readPIDFile(pidFile)
	if err != nil || !isProcessAlive(pid) {
		os.Remove(pidFile)
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

	if tracked && mp.Cmd != nil && mp.Cmd.Process != nil {
		if isProcessAlive(mp.PID) {
			return true, mp.PID
		}
		return false, 0
	}

	pidFile := filepath.Join(m.pidDir, name+".pid")
	pid, err := readPIDFile(pidFile)
	if err != nil {
		return false, 0
	}
	if isProcessAlive(pid) {
		return true, pid
	}
	os.Remove(pidFile)
	return false, 0
}

// Recover attempts to recover process tracking from a PID file.
func (m *Manager) Recover(name string) (*ManagedProcess, error) {
	pidFile := filepath.Join(m.pidDir, name+".pid")
	pid, err := readPIDFile(pidFile)
	if err != nil {
		return nil, fmt.Errorf("read pid file for %s: %w", name, err)
	}
	if !isProcessAlive(pid) {
		os.Remove(pidFile)
		return nil, fmt.Errorf("process %s (pid %d) not running", name, pid)
	}

	pgid, _ := syscall.Getpgid(pid)
	mp := &ManagedProcess{
		Name:    name,
		PIDFile: pidFile,
		LogFile: filepath.Join(m.logDir, name+".log"),
		PID:     pid,
		PGID:    pgid,
	}

	m.mu.Lock()
	m.processes[name] = mp
	m.mu.Unlock()

	return mp, nil
}

func (m *Manager) killProcess(pgid, pid int, pidFile string) error {
	target := -pgid
	if pgid == 0 {
		target = pid
	}

	_ = syscall.Kill(target, syscall.SIGTERM)

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !isProcessAlive(pid) {
			os.Remove(pidFile)
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}

	_ = syscall.Kill(target, syscall.SIGKILL)
	os.Remove(pidFile)
	return nil
}

func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	return syscall.Kill(pid, 0) == nil
}

func readPIDFile(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

func atomicWriteFile(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
