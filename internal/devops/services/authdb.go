package services

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"

	"alex/internal/devops"
	"alex/internal/devops/health"
	devlog "alex/internal/devops/log"
)

// AuthDBConfig holds auth database configuration.
type AuthDBConfig struct {
	DatabaseURL string
	JWTSecret   string
	Skip        bool
	ProjectDir  string
	LogDir      string
}

// AuthDBService manages the local PostgreSQL auth database.
type AuthDBService struct {
	health  *health.Checker
	section *devlog.SectionWriter
	config  AuthDBConfig
	state   atomic.Value // devops.ServiceState
}

// NewAuthDBService creates a new auth DB service.
func NewAuthDBService(hc *health.Checker, sw *devlog.SectionWriter, cfg AuthDBConfig) *AuthDBService {
	s := &AuthDBService{
		health:  hc,
		section: sw,
		config:  cfg,
	}
	s.state.Store(devops.StateStopped)
	return s
}

func (s *AuthDBService) Name() string { return "authdb" }

func (s *AuthDBService) State() devops.ServiceState {
	return s.state.Load().(devops.ServiceState)
}

func (s *AuthDBService) Health(_ context.Context) health.Result {
	// Auth DB health is determined by the setup script success
	state := s.State()
	if state == devops.StateHealthy {
		return health.Result{Healthy: true, Message: "auth DB ready"}
	}
	return health.Result{Healthy: false, Message: fmt.Sprintf("state: %s", state)}
}

func (s *AuthDBService) Start(ctx context.Context) error {
	s.state.Store(devops.StateStarting)

	if s.config.Skip {
		s.section.Info("Skipping local auth DB auto-setup (SKIP_LOCAL_AUTH_DB=1)")
		s.state.Store(devops.StateHealthy)
		return nil
	}

	if s.config.DatabaseURL == "" {
		s.state.Store(devops.StateHealthy)
		return nil
	}

	// Only set up local auth DB if URL points to localhost
	host := authDBHost(s.config.DatabaseURL)
	if !isLocalHost(host) {
		s.state.Store(devops.StateHealthy)
		return nil
	}

	// Run the setup script
	scriptPath := filepath.Join(s.config.ProjectDir, "scripts", "setup_local_auth_db.sh")
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		s.section.Warn("Auth DB setup script not found: %s", scriptPath)
		s.state.Store(devops.StateHealthy)
		return nil
	}

	if err := os.MkdirAll(s.config.LogDir, 0o755); err != nil {
		s.section.Warn("Failed to create auth DB log dir: %v", err)
	}
	logFile := filepath.Join(s.config.LogDir, "setup_auth_db.log")

	s.section.Info("Setting up local auth DB...")
	cmd := exec.CommandContext(ctx, scriptPath)
	cmd.Dir = s.config.ProjectDir

	f, err := os.Create(logFile)
	if err == nil {
		cmd.Stdout = f
		cmd.Stderr = f
		defer f.Close()
	}

	if err := cmd.Run(); err != nil {
		s.section.Warn("Local auth DB setup failed; auth may be disabled")
		s.section.Warn("See %s for details", logFile)
		// Non-fatal: auth can degrade to memory stores
		s.state.Store(devops.StateHealthy)
		return nil
	}

	s.section.Success("Local auth DB ready")
	s.state.Store(devops.StateHealthy)
	return nil
}

func (s *AuthDBService) Stop(_ context.Context) error {
	// Auth DB container is managed externally via docker-compose;
	// we don't stop it during dev down
	s.state.Store(devops.StateStopped)
	return nil
}

func authDBHost(dbURL string) string {
	// Parse postgres://user:pass@host:port/db
	rest := dbURL
	if idx := strings.Index(rest, "://"); idx >= 0 {
		rest = rest[idx+3:]
	}
	// Remove auth portion
	if idx := strings.LastIndex(rest, "@"); idx >= 0 {
		rest = rest[idx+1:]
	}
	// Remove path and query
	if idx := strings.IndexAny(rest, "/?"); idx >= 0 {
		rest = rest[:idx]
	}
	// Handle IPv6
	if strings.HasPrefix(rest, "[") {
		if idx := strings.Index(rest, "]"); idx >= 0 {
			return rest[1:idx]
		}
	}
	// Remove port
	if idx := strings.LastIndex(rest, ":"); idx >= 0 {
		return rest[:idx]
	}
	return rest
}

func isLocalHost(host string) bool {
	switch host {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}
