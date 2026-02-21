package devops

import (
	"context"
	"fmt"
	"io"
	"os"

	"alex/internal/devops/health"
	devlog "alex/internal/devops/log"
	"alex/internal/devops/port"
	"alex/internal/devops/process"
)

// ServiceStatus holds the status of a single service.
type ServiceStatus struct {
	Name    string
	State   ServiceState
	PID     int
	Healthy bool
	Message string
}

// Orchestrator coordinates the startup and shutdown of all dev services.
type Orchestrator struct {
	services []Service
	health   *health.Checker
	logMgr   *devlog.Manager
	ports    *port.Allocator
	procMgr  *process.Manager
	config   *DevConfig
	section  *devlog.SectionWriter
}

// NewOrchestrator creates a new orchestrator with all dependencies.
func NewOrchestrator(cfg *DevConfig) *Orchestrator {
	section := devlog.NewSectionWriter(os.Stdout, true)
	hc := health.NewChecker()
	pa := port.NewAllocator()
	pm := process.NewManager(cfg.PIDDir, cfg.LogDir)
	lm := devlog.NewManager(cfg.LogDir)

	return &Orchestrator{
		health:  hc,
		logMgr:  lm,
		ports:   pa,
		procMgr: pm,
		config:  cfg,
		section: section,
	}
}

// Health returns the health checker.
func (o *Orchestrator) Health() *health.Checker { return o.health }

// Ports returns the port allocator.
func (o *Orchestrator) Ports() *port.Allocator { return o.ports }

// ProcessManager returns the process manager.
func (o *Orchestrator) ProcessManager() *process.Manager { return o.procMgr }

// Section returns the section writer.
func (o *Orchestrator) Section() *devlog.SectionWriter { return o.section }

// Config returns the dev config.
func (o *Orchestrator) Config() *DevConfig { return o.config }

// RegisterServices sets the ordered list of services to manage.
func (o *Orchestrator) RegisterServices(services ...Service) {
	o.services = services
}

// Up starts all registered services in order.
func (o *Orchestrator) Up(ctx context.Context) error {
	if err := os.MkdirAll(o.config.PIDDir, 0o755); err != nil {
		return fmt.Errorf("create pid dir %s: %w", o.config.PIDDir, err)
	}
	if err := os.MkdirAll(o.config.LogDir, 0o755); err != nil {
		return fmt.Errorf("create log dir %s: %w", o.config.LogDir, err)
	}

	for _, svc := range o.services {
		o.section.Section(svc.Name())
		if err := svc.Start(ctx); err != nil {
			o.section.Error("Failed to start %s: %v", svc.Name(), err)
			return fmt.Errorf("start %s: %w", svc.Name(), err)
		}
	}

	return nil
}

// Down stops registered services in reverse order.
func (o *Orchestrator) Down(ctx context.Context) error {
	var lastErr error
	for i := len(o.services) - 1; i >= 0; i-- {
		svc := o.services[i]
		if err := svc.Stop(ctx); err != nil {
			o.section.Error("Failed to stop %s: %v", svc.Name(), err)
			lastErr = err
		}
	}
	return lastErr
}

// Status returns the status of all services.
func (o *Orchestrator) Status(ctx context.Context) []ServiceStatus {
	var statuses []ServiceStatus
	for _, svc := range o.services {
		hr := svc.Health(ctx)
		_, pid := o.procMgr.IsRunning(svc.Name())
		statuses = append(statuses, ServiceStatus{
			Name:    svc.Name(),
			State:   svc.State(),
			PID:     pid,
			Healthy: hr.Healthy,
			Message: hr.Message,
		})
	}
	return statuses
}

// resolveTargets returns the services matching the given names, or all if empty.
func (o *Orchestrator) resolveTargets(names []string) []Service {
	if len(names) == 0 {
		return o.services
	}
	nameSet := make(map[string]bool, len(names))
	for _, n := range names {
		nameSet[n] = true
	}
	var targets []Service
	for _, svc := range o.services {
		if nameSet[svc.Name()] {
			targets = append(targets, svc)
		}
	}
	return targets
}

// Restart restarts the named services (or all if none specified).
// For Buildable services, it compiles a new binary first. If the build fails,
// the old process is preserved and an error is returned (no downtime).
func (o *Orchestrator) Restart(ctx context.Context, names ...string) error {
	targets := o.resolveTargets(names)

	for _, svc := range targets {
		o.section.Section("Restart " + svc.Name())

		if buildable, ok := svc.(Buildable); ok {
			// Safe path: build before stopping the old process
			staging, err := buildable.Build(ctx)
			if err != nil {
				o.section.Error("Build failed for %s: %v (old process preserved)", svc.Name(), err)
				return fmt.Errorf("restart %s: build failed: %w", svc.Name(), err)
			}
			if err := svc.Stop(ctx); err != nil {
				o.section.Warn("Stop %s: %v", svc.Name(), err)
			}
			if err := buildable.Promote(staging); err != nil {
				return fmt.Errorf("restart %s: promote: %w", svc.Name(), err)
			}
			if err := svc.Start(ctx); err != nil {
				return fmt.Errorf("restart %s: %w", svc.Name(), err)
			}
		} else {
			// Non-buildable services: simple stop + start
			if err := svc.Stop(ctx); err != nil {
				o.section.Warn("Stop %s: %v", svc.Name(), err)
			}
			if err := svc.Start(ctx); err != nil {
				return fmt.Errorf("restart %s: %w", svc.Name(), err)
			}
		}
	}
	return nil
}

// Logs tails log files for the given target.
func (o *Orchestrator) Logs(ctx context.Context, target string, follow bool) error {
	var services []string
	switch target {
	case "", "all":
		// All services
	case "server", "backend":
		services = []string{"server", "backend"}
	case "web":
		services = []string{"web"}
	default:
		services = []string{target}
	}
	return o.logMgr.Tail(ctx, services, follow, io.Writer(os.Stdout))
}
