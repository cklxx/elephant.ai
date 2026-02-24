package services

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"alex/internal/devops"
	"alex/internal/devops/health"
	devlog "alex/internal/devops/log"
	"alex/internal/devops/port"
	"alex/internal/devops/process"
)

// WebConfig holds web service configuration.
type WebConfig struct {
	Port       int
	WebDir     string
	ServerPort int // backend port for NEXT_PUBLIC_API_URL
	AutoStop   bool
	AutoHeal   bool // auto-heal Next.js .next/dev ENOENT corruption
}

// WebService manages the Next.js development server.
type WebService struct {
	pm      *process.Manager
	ports   *port.Allocator
	health  *health.Checker
	section *devlog.SectionWriter
	config  WebConfig
	state   atomic.Value // devops.ServiceState
}

// NewWebService creates a new web service.
func NewWebService(pm *process.Manager, pa *port.Allocator, hc *health.Checker, sw *devlog.SectionWriter, cfg WebConfig) *WebService {
	s := &WebService{
		pm:      pm,
		ports:   pa,
		health:  hc,
		section: sw,
		config:  cfg,
	}
	s.state.Store(devops.StateStopped)
	return s
}

func (s *WebService) Name() string { return "web" }

func (s *WebService) State() devops.ServiceState {
	return s.state.Load().(devops.ServiceState)
}

func (s *WebService) Health(ctx context.Context) health.Result {
	return s.health.Check(ctx, "web")
}

func (s *WebService) Start(ctx context.Context) error {
	s.state.Store(devops.StateStarting)

	// Clean up orphan npm/next processes before starting
	s.cleanupOrphanWebProcesses()

	// Check if already running
	if running, pid := s.pm.IsRunning("web"); running {
		s.section.Success("Web already running (PID: %d)", pid)
		s.state.Store(devops.StateHealthy)
		return nil
	}

	// Clean up stale Next.js dev lock
	lockFile := filepath.Join(s.config.WebDir, ".next", "dev", "lock")
	if _, err := os.Stat(lockFile); err == nil {
		s.section.Warn("Removing stale Next.js dev lock: %s", lockFile)
		os.Remove(lockFile)
	}

	// Port allocation
	actualPort, err := s.ports.Reserve("web", s.config.Port)
	if err != nil {
		if s.config.AutoStop {
			s.section.Warn("Web port %d in use; stopping conflicting listeners", s.config.Port)
			if stopErr := s.ports.StopListeners(s.config.Port); stopErr != nil {
				s.section.Warn("Failed stopping conflicting listeners on %d: %v", s.config.Port, stopErr)
			}
			time.Sleep(500 * time.Millisecond)
			actualPort, err = s.ports.Reserve("web", s.config.Port)
		}
		if err != nil {
			s.state.Store(devops.StateFailed)
			return fmt.Errorf("reserve port %d: %w", s.config.Port, err)
		}
	}

	// Check node_modules
	nodeModules := filepath.Join(s.config.WebDir, "node_modules")
	if _, err := os.Stat(nodeModules); os.IsNotExist(err) {
		s.section.Warn("web/node_modules not found; run: (cd web && npm install)")
	}

	// Start web dev server
	s.section.Info("Starting web on :%d...", actualPort)
	npmPath, err := exec.LookPath("npm")
	if err != nil {
		s.state.Store(devops.StateFailed)
		return fmt.Errorf("npm not found in PATH: %w", err)
	}

	cmd := exec.CommandContext(ctx, npmPath, "--prefix", s.config.WebDir, "run", "dev")
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("PORT=%d", actualPort),
		fmt.Sprintf("NEXT_PUBLIC_API_URL=http://localhost:%d", s.config.ServerPort),
	)

	if _, err := s.pm.Start(ctx, "web", cmd); err != nil {
		s.state.Store(devops.StateFailed)
		return fmt.Errorf("start web: %w", err)
	}

	s.state.Store(devops.StateRunning)

	// Register and wait for health
	healthURL := fmt.Sprintf("http://localhost:%d", actualPort)
	s.health.Register("web", health.Probe{
		Type:   health.ProbeHTTP,
		Target: healthURL,
	})
	if err := s.health.WaitHealthy(ctx, "web", 30*time.Second); err != nil {
		s.section.Warn("Web health check timed out: %v", err)
	}

	running, pid := s.pm.IsRunning("web")
	if running {
		s.section.Success("Web started (PID: %d)", pid)
		s.state.Store(devops.StateHealthy)
	}
	return nil
}

func (s *WebService) Stop(ctx context.Context) error {
	s.state.Store(devops.StateStopping)

	if err := s.pm.Stop(ctx, "web"); err != nil {
		s.state.Store(devops.StateFailed)
		return err
	}

	// Clean up Next.js lock
	lockFile := filepath.Join(s.config.WebDir, ".next", "dev", "lock")
	os.Remove(lockFile)

	// Clean up orphan npm/next processes
	s.cleanupOrphanWebProcesses()

	s.ports.Release("web")
	s.state.Store(devops.StateStopped)
	return nil
}

// cleanupOrphanWebProcesses kills npm/next dev processes that are not tracked
// by the process manager. This prevents orphan child processes after restarts.
func (s *WebService) cleanupOrphanWebProcesses() {
	_, trackedPID := s.pm.IsRunning("web")

	out, err := exec.Command("ps", "-axo", "pid=,command=").Output()
	if err != nil {
		return
	}

	webDir := s.config.WebDir
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Match npm --prefix <webDir> run dev OR <webDir>...next dev
		isNpmDev := strings.Contains(line, "npm --prefix "+webDir+" run dev")
		isNextDev := strings.Contains(line, webDir) && strings.Contains(line, "next dev")
		if !isNpmDev && !isNextDev {
			continue
		}

		fields := strings.SplitN(line, " ", 2)
		if len(fields) < 1 {
			continue
		}
		pid, parseErr := strconv.Atoi(strings.TrimSpace(fields[0]))
		if parseErr != nil || pid <= 0 {
			continue
		}
		if pid == trackedPID {
			continue
		}

		s.section.Warn("Stopping orphan web process PID %d", pid)
		_ = syscall.Kill(pid, syscall.SIGTERM)
	}
}

// clearNextDevArtifacts removes the .next/dev directory to recover from
// corrupted Next.js dev state (ENOENT errors).
func (s *WebService) clearNextDevArtifacts() {
	nextDevDir := filepath.Join(s.config.WebDir, ".next", "dev")
	if info, err := os.Stat(nextDevDir); err == nil && info.IsDir() {
		s.section.Warn("Clearing corrupted Next.js dev artifacts: %s", nextDevDir)
		os.RemoveAll(nextDevDir)
	}
}
