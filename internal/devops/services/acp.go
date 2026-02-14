package services

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"

	"alex/internal/devops"
	"alex/internal/devops/health"
	devlog "alex/internal/devops/log"
	"alex/internal/devops/port"
	"alex/internal/devops/process"
)

// ACPConfig holds ACP daemon configuration.
type ACPConfig struct {
	Port       int
	Host       string
	RunMode    string // "host" (only mode after sandbox removal)
	Enabled    bool
	ProjectDir string
	LogDir     string
	PIDDir     string
}

// ACPService manages the ACP daemon.
type ACPService struct {
	pm            *process.Manager
	ports         *port.Allocator
	health        *health.Checker
	section       *devlog.SectionWriter
	config        ACPConfig
	state         atomic.Value // devops.ServiceState
	skipNextBuild bool         // set by Promote, cleared by resolveAlexBinary
}

// NewACPService creates a new ACP service for host-mode operation.
func NewACPService(pm *process.Manager, pa *port.Allocator, hc *health.Checker, sw *devlog.SectionWriter, cfg ACPConfig) *ACPService {
	s := &ACPService{
		pm:      pm,
		ports:   pa,
		health:  hc,
		section: sw,
		config:  cfg,
	}
	s.state.Store(devops.StateStopped)
	return s
}

func (s *ACPService) Name() string { return "acp" }

func (s *ACPService) State() devops.ServiceState {
	return s.state.Load().(devops.ServiceState)
}

func (s *ACPService) Health(_ context.Context) health.Result {
	state := s.State()
	if state == devops.StateHealthy {
		return health.Result{Healthy: true, Message: "ACP running"}
	}
	return health.Result{Healthy: false, Message: fmt.Sprintf("state: %s", state)}
}

func (s *ACPService) Start(ctx context.Context) error {
	if !s.config.Enabled {
		s.state.Store(devops.StateHealthy)
		return nil
	}

	s.state.Store(devops.StateStarting)

	// Check if already running
	if running, pid := s.pm.IsRunning("acp"); running {
		s.section.Info("ACP already running (PID: %d)", pid)
		s.state.Store(devops.StateHealthy)
		return nil
	}

	// Allocate port
	acpPort, err := s.ports.Reserve("acp", s.config.Port)
	if err != nil {
		s.state.Store(devops.StateFailed)
		return fmt.Errorf("reserve ACP port: %w", err)
	}

	// Find or build alex binary
	alexBin, err := s.resolveAlexBinary(ctx)
	if err != nil {
		s.state.Store(devops.StateFailed)
		return fmt.Errorf("resolve alex binary: %w", err)
	}

	// Start ACP daemon
	s.section.Info("Starting ACP daemon on %s:%d...", s.config.Host, acpPort)

	cmd := exec.CommandContext(ctx, alexBin, "acp", "serve",
		"--host", s.config.Host,
		"--port", fmt.Sprintf("%d", acpPort))
	cmd.Dir = s.config.ProjectDir

	if _, err := s.pm.Start(ctx, "acp", cmd); err != nil {
		s.state.Store(devops.StateFailed)
		return fmt.Errorf("start ACP: %w", err)
	}

	// Save port for other services to discover
	portFile := filepath.Join(s.config.PIDDir, "acp.port")
	if err := os.MkdirAll(filepath.Dir(portFile), 0o755); err != nil {
		s.state.Store(devops.StateFailed)
		return fmt.Errorf("create ACP port file dir: %w", err)
	}
	if err := os.WriteFile(portFile, []byte(fmt.Sprintf("%d", acpPort)), 0o644); err != nil {
		s.state.Store(devops.StateFailed)
		return fmt.Errorf("write ACP port file: %w", err)
	}

	s.state.Store(devops.StateHealthy)
	s.section.Success("ACP started on %s:%d", s.config.Host, acpPort)
	return nil
}

func (s *ACPService) Stop(ctx context.Context) error {
	if !s.config.Enabled {
		s.state.Store(devops.StateStopped)
		return nil
	}

	s.state.Store(devops.StateStopping)

	if err := s.pm.Stop(ctx, "acp"); err != nil {
		s.state.Store(devops.StateFailed)
		return err
	}

	s.ports.Release("acp")
	s.state.Store(devops.StateStopped)
	return nil
}

// alexBinPath returns the production alex binary path.
func (s *ACPService) alexBinPath() string {
	return filepath.Join(s.config.ProjectDir, "alex")
}

// stagingPath returns the staging alex binary path.
func (s *ACPService) stagingPath() string {
	return s.alexBinPath() + ".staging"
}

// Build compiles the alex CLI to a staging path without touching the running binary.
// Implements devops.Buildable.
func (s *ACPService) Build(ctx context.Context) (string, error) {
	staging := s.stagingPath()
	toolchain := filepath.Join(s.config.ProjectDir, "scripts", "go-with-toolchain.sh")
	if _, err := os.Stat(toolchain); err != nil {
		return "", fmt.Errorf("alex build toolchain not found: %w", err)
	}

	s.section.Info("Building CLI (./cmd/alex) → %s ...", staging)
	cmd := exec.CommandContext(ctx, toolchain, "build", "-o", staging, "./cmd/alex")
	cmd.Dir = s.config.ProjectDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		os.Remove(staging)
		return "", fmt.Errorf("build alex CLI: %s: %w", string(out), err)
	}

	info, err := os.Stat(staging)
	if err != nil {
		return "", fmt.Errorf("alex CLI build succeeded but %s not found", staging)
	}
	if info.Mode()&0o111 == 0 {
		os.Remove(staging)
		return "", fmt.Errorf("alex staging binary %s is not executable", staging)
	}

	s.section.Success("ACP CLI staged: %s", staging)
	return staging, nil
}

// Promote atomically replaces the production alex binary with the staged one.
// Implements devops.Buildable.
func (s *ACPService) Promote(stagingPath string) error {
	if err := os.Rename(stagingPath, s.alexBinPath()); err != nil {
		return fmt.Errorf("promote acp: %w", err)
	}
	s.skipNextBuild = true
	s.section.Success("ACP promoted: %s → %s", stagingPath, s.alexBinPath())
	return nil
}

func (s *ACPService) resolveAlexBinary(ctx context.Context) (string, error) {
	alexBin := s.alexBinPath()

	// If Promote already placed the binary, skip build
	if s.skipNextBuild {
		s.skipNextBuild = false
		if _, err := os.Stat(alexBin); err == nil {
			s.section.Info("ACP build skipped (already promoted)")
			return alexBin, nil
		}
	}

	// Check for pre-built alex binary
	if _, err := os.Stat(alexBin); err == nil {
		return alexBin, nil
	}

	// Build via staged path then promote
	staging, err := s.Build(ctx)
	if err != nil {
		return "", err
	}
	if err := s.Promote(staging); err != nil {
		return "", err
	}
	return alexBin, nil
}

// ResolveACPAddr returns the ACP executor address if available.
func ResolveACPAddr(host string, pidDir string) string {
	portFile := filepath.Join(pidDir, "acp.port")
	data, err := os.ReadFile(portFile)
	if err != nil {
		return ""
	}
	port := string(data)
	if port == "" || port == "0" {
		return ""
	}
	if host == "" {
		host = "127.0.0.1"
	}
	return fmt.Sprintf("http://%s:%s", host, port)
}
