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
	RunMode    string // "sandbox" or "host"
	Enabled    bool   // START_ACP_WITH_SANDBOX
	ProjectDir string
	LogDir     string
	PIDDir     string
}

// ACPService manages the ACP daemon (host mode only).
// Sandbox-mode ACP is managed by SandboxService.
type ACPService struct {
	pm      *process.Manager
	ports   *port.Allocator
	health  *health.Checker
	section *devlog.SectionWriter
	config  ACPConfig
	state   atomic.Value // devops.ServiceState
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

	// ACP in sandbox mode is handled by SandboxService
	if s.config.RunMode == "sandbox" {
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
	os.MkdirAll(filepath.Dir(portFile), 0o755)
	os.WriteFile(portFile, []byte(fmt.Sprintf("%d", acpPort)), 0o644)

	s.state.Store(devops.StateHealthy)
	s.section.Success("ACP started on %s:%d", s.config.Host, acpPort)
	return nil
}

func (s *ACPService) Stop(ctx context.Context) error {
	if !s.config.Enabled || s.config.RunMode == "sandbox" {
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

func (s *ACPService) resolveAlexBinary(ctx context.Context) (string, error) {
	// Check for pre-built alex binary
	alexBin := filepath.Join(s.config.ProjectDir, "alex")
	if _, err := os.Stat(alexBin); err == nil {
		return alexBin, nil
	}

	// Build with toolchain
	toolchain := filepath.Join(s.config.ProjectDir, "scripts", "go-with-toolchain.sh")
	if _, err := os.Stat(toolchain); err == nil {
		s.section.Info("Building CLI (./cmd/alex) with toolchain...")
		cmd := exec.CommandContext(ctx, toolchain, "build", "-o", alexBin, "./cmd/alex")
		cmd.Dir = s.config.ProjectDir
		if out, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("build alex CLI: %s: %w", string(out), err)
		}
		return alexBin, nil
	}

	return "", fmt.Errorf("alex binary not found and no build toolchain available")
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
