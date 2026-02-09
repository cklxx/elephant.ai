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
	_ = ctx
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
	if !isProcessAlive(pid) {
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

	if tracked && mp.Cmd != nil && mp.Cmd.Process != nil {
		if isProcessAlive(mp.PID) {
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
	if !isProcessAlive(pid) {
		cleanupPIDState(pidFile, metaFile)
		return false, 0
	}
	if !identityMatches(metaFile, pid) {
		cleanupPIDState(pidFile, metaFile)
		return false, 0
	}
	if isProcessAlive(pid) {
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
	if !isProcessAlive(pid) {
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
	target := -pgid
	if pgid == 0 {
		target = pid
	}

	_ = syscall.Kill(target, syscall.SIGTERM)

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !isProcessAlive(pid) {
			cleanupPIDState(pidFile, metaFile)
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}

	_ = syscall.Kill(target, syscall.SIGKILL)
	cleanupPIDState(pidFile, metaFile)
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

func atomicWriteFile(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
