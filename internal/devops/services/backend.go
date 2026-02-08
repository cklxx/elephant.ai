package services

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"alex/internal/devops"
	"alex/internal/devops/health"
	devlog "alex/internal/devops/log"
	"alex/internal/devops/port"
	"alex/internal/devops/process"
)

// BackendConfig holds backend service configuration.
type BackendConfig struct {
	Port       int
	OutputBin  string
	ProjectDir string
	LogDir     string
	CGOMode    string
	AutoStop   bool // auto-stop conflicting port listeners
}

// BackendService manages the Go backend server.
type BackendService struct {
	pm            *process.Manager
	ports         *port.Allocator
	health        *health.Checker
	section       *devlog.SectionWriter
	config        BackendConfig
	state         atomic.Value // devops.ServiceState
	skipNextBuild bool         // set by Promote, cleared by build
}

// NewBackendService creates a new backend service.
func NewBackendService(pm *process.Manager, pa *port.Allocator, hc *health.Checker, sw *devlog.SectionWriter, cfg BackendConfig) *BackendService {
	s := &BackendService{
		pm:      pm,
		ports:   pa,
		health:  hc,
		section: sw,
		config:  cfg,
	}
	s.state.Store(devops.StateStopped)
	return s
}

func (s *BackendService) Name() string { return "backend" }

func (s *BackendService) State() devops.ServiceState {
	return s.state.Load().(devops.ServiceState)
}

func (s *BackendService) Health(ctx context.Context) health.Result {
	return s.health.Check(ctx, "backend")
}

func (s *BackendService) Start(ctx context.Context) error {
	s.state.Store(devops.StateStarting)

	// Check if already running
	if running, pid := s.pm.IsRunning("backend"); running {
		s.section.Success("Backend already running (PID: %d)", pid)
		s.state.Store(devops.StateHealthy)
		return nil
	}

	// Port allocation
	actualPort, err := s.ports.Reserve("backend", s.config.Port)
	if err != nil {
		// Try auto-stop conflicting listeners
		if s.config.AutoStop {
			s.section.Warn("Backend port %d in use; stopping conflicting listeners", s.config.Port)
			s.ports.StopListeners(s.config.Port)
			time.Sleep(500 * time.Millisecond)
			actualPort, err = s.ports.Reserve("backend", s.config.Port)
		}
		if err != nil {
			s.state.Store(devops.StateFailed)
			return fmt.Errorf("reserve port %d: %w", s.config.Port, err)
		}
	}

	// Build
	if err := s.build(ctx); err != nil {
		s.state.Store(devops.StateFailed)
		return err
	}

	// Start process
	s.section.Info("Starting backend on :%d...", actualPort)
	cmd := exec.CommandContext(ctx, s.config.OutputBin)
	cmd.Dir = s.config.ProjectDir
	cmd.Env = s.buildEnv(actualPort)

	if _, err := s.pm.Start(ctx, "backend", cmd); err != nil {
		s.state.Store(devops.StateFailed)
		return fmt.Errorf("start backend: %w", err)
	}

	s.state.Store(devops.StateRunning)

	// Register and wait for health
	healthURL := fmt.Sprintf("http://localhost:%d/health", actualPort)
	s.health.Register("backend", health.Probe{
		Type:   health.ProbeHTTP,
		Target: healthURL,
	})
	if err := s.health.WaitHealthy(ctx, "backend", 30*time.Second); err != nil {
		s.section.Warn("Backend health check timed out: %v", err)
		// Non-fatal: process might still be starting
	}

	running, pid := s.pm.IsRunning("backend")
	if running {
		s.section.Success("Backend started (PID: %d)", pid)
		s.state.Store(devops.StateHealthy)
	}
	return nil
}

func (s *BackendService) Stop(ctx context.Context) error {
	s.state.Store(devops.StateStopping)
	if err := s.pm.Stop(ctx, "backend"); err != nil {
		s.state.Store(devops.StateFailed)
		return err
	}
	s.ports.Release("backend")
	s.state.Store(devops.StateStopped)
	return nil
}

// stagingPath returns the staging binary path derived from the production path.
func (s *BackendService) stagingPath() string {
	return s.config.OutputBin + ".staging"
}

// Build compiles the backend to a staging path without touching the running binary.
// Implements devops.Buildable.
func (s *BackendService) Build(ctx context.Context) (string, error) {
	staging := s.stagingPath()
	s.section.Info("Building backend (./cmd/alex-server) → %s ...", staging)

	cgoEnabled := s.detectCGO()
	if cgoEnabled {
		s.section.Info("CGO enabled for build (ALEX_CGO_MODE=%s)", s.config.CGOMode)
	} else {
		s.section.Info("CGO disabled for build (ALEX_CGO_MODE=%s)", s.config.CGOMode)
	}

	goToolchain := filepath.Join(s.config.ProjectDir, "scripts", "go-with-toolchain.sh")
	cmd := exec.CommandContext(ctx, goToolchain, "build", "-o", staging, "./cmd/alex-server")
	cmd.Dir = s.config.ProjectDir

	env := os.Environ()
	if cgoEnabled {
		env = append(env, "CGO_ENABLED=1")
	} else {
		env = append(env, "CGO_ENABLED=0")
	}
	cmd.Env = env

	out, err := cmd.CombinedOutput()
	if err != nil {
		os.Remove(staging) // clean up partial artifact
		return "", fmt.Errorf("build backend: %s: %w", string(out), err)
	}

	info, err := os.Stat(staging)
	if err != nil {
		return "", fmt.Errorf("backend build succeeded but %s not found", staging)
	}
	if info.Mode()&0o111 == 0 {
		os.Remove(staging)
		return "", fmt.Errorf("backend staging binary %s is not executable", staging)
	}

	s.section.Success("Backend staged: %s", staging)
	return staging, nil
}

// Promote atomically replaces the production binary with the staged one.
// Implements devops.Buildable.
func (s *BackendService) Promote(stagingPath string) error {
	if err := os.Rename(stagingPath, s.config.OutputBin); err != nil {
		return fmt.Errorf("promote backend: %w", err)
	}
	s.skipNextBuild = true
	s.section.Success("Backend promoted: %s → %s", stagingPath, s.config.OutputBin)
	return nil
}

// build compiles the backend to the production path.
// If Promote was called beforehand (via Orchestrator safe restart),
// the build step is skipped since the binary is already in place.
func (s *BackendService) build(ctx context.Context) error {
	if s.skipNextBuild {
		s.skipNextBuild = false
		s.section.Info("Backend build skipped (already promoted)")
		return nil
	}
	staging, err := s.Build(ctx)
	if err != nil {
		return err
	}
	return s.Promote(staging)
}

func (s *BackendService) detectCGO() bool {
	if v := os.Getenv("CGO_ENABLED"); v != "" {
		return v == "1"
	}

	switch s.config.CGOMode {
	case "on":
		return true
	case "off":
		return false
	default: // "auto"
		return s.cgoSQLiteReady()
	}
}

func (s *BackendService) cgoSQLiteReady() bool {
	// Check for C compiler
	_, err1 := exec.LookPath("clang")
	_, err2 := exec.LookPath("gcc")
	if err1 != nil && err2 != nil {
		return false
	}

	if runtime.GOOS == "darwin" {
		// Need Xcode CLT
		if _, err := exec.LookPath("xcode-select"); err != nil {
			return false
		}
		cmd := exec.Command("xcode-select", "-p")
		if err := cmd.Run(); err != nil {
			return false
		}
	}

	// Check for sqlite3 header
	return hasSQLiteHeader()
}

func hasSQLiteHeader() bool {
	paths := []string{"/usr/include/sqlite3.h", "/usr/local/include/sqlite3.h"}
	if runtime.GOOS == "darwin" {
		if out, err := exec.Command("xcrun", "--show-sdk-path").Output(); err == nil {
			sdkPath := strings.TrimSpace(string(out))
			paths = append(paths, filepath.Join(sdkPath, "usr/include/sqlite3.h"))
		}
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	// Try pkg-config
	if _, err := exec.LookPath("pkg-config"); err == nil {
		cmd := exec.Command("pkg-config", "--exists", "sqlite3")
		if err := cmd.Run(); err == nil {
			return true
		}
	}
	return false
}

func (s *BackendService) buildEnv(port int) []string {
	env := os.Environ()
	env = append(env,
		fmt.Sprintf("PORT=%d", port),
		fmt.Sprintf("ALEX_SERVER_PORT=%d", port),
		"ALEX_SERVER_MODE=deploy",
		fmt.Sprintf("ALEX_LOG_DIR=%s", s.config.LogDir),
	)
	return env
}
