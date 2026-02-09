package services

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"alex/internal/devops"
	"alex/internal/devops/docker"
	"alex/internal/devops/health"
	devlog "alex/internal/devops/log"
	"alex/internal/devops/port"
)

// SandboxConfig holds sandbox-specific configuration.
type SandboxConfig struct {
	ContainerName     string
	Image             string
	Port              int
	BaseURL           string
	WorkspaceDir      string
	AutoInstallCLI    bool
	SandboxConfigPath string // path inside container

	// ACP
	ACPPort             int
	ACPHost             string
	ACPRunMode          string
	StartACPWithSandbox bool

	// Project paths
	ProjectDir string
	PIDDir     string
	LogDir     string
}

// SandboxService manages the sandbox Docker container lifecycle.
type SandboxService struct {
	docker  docker.Client
	ports   *port.Allocator
	health  *health.Checker
	section *devlog.SectionWriter
	config  SandboxConfig
	state   atomic.Value // devops.ServiceState
}

// NewSandboxService creates a new sandbox service.
func NewSandboxService(dc docker.Client, ports *port.Allocator, hc *health.Checker, sw *devlog.SectionWriter, cfg SandboxConfig) *SandboxService {
	s := &SandboxService{
		docker:  dc,
		ports:   ports,
		health:  hc,
		section: sw,
		config:  cfg,
	}
	s.state.Store(devops.StateStopped)
	return s
}

func (s *SandboxService) Name() string { return "sandbox" }

func (s *SandboxService) State() devops.ServiceState {
	return s.state.Load().(devops.ServiceState)
}

func (s *SandboxService) Health(ctx context.Context) health.Result {
	return s.health.Check(ctx, "sandbox")
}

func (s *SandboxService) Start(ctx context.Context) error {
	s.state.Store(devops.StateStarting)

	if !s.isLocalSandboxURL() {
		// Remote sandbox: just wait for health
		s.section.Info("Using remote sandbox at %s", s.config.BaseURL)
		if err := s.health.WaitHealthy(ctx, "sandbox", 30*time.Second); err != nil {
			s.state.Store(devops.StateFailed)
			return err
		}
		s.state.Store(devops.StateHealthy)
		return nil
	}

	running, _ := s.docker.ContainerRunning(ctx, s.config.ContainerName)
	exists, _ := s.docker.ContainerExists(ctx, s.config.ContainerName)

	// Determine ACP port — reuse existing container's port when possible
	acpPort := s.config.ACPPort
	if s.acpShouldRunInSandbox() && acpPort == 0 {
		if exists {
			if existing := s.detectExistingACPPort(ctx); existing > 0 {
				acpPort = existing
			}
		}
		if acpPort == 0 {
			p, err := s.ports.Reserve("acp", 0)
			if err != nil {
				s.state.Store(devops.StateFailed)
				return fmt.Errorf("allocate ACP port: %w", err)
			}
			acpPort = p
		}
		s.config.ACPPort = acpPort
	}

	// Check if existing container needs recreation
	if exists {
		needsRecreate := s.needsRecreate(ctx)
		if needsRecreate {
			s.section.Warn("Sandbox container needs recreation")
			if running {
				if stopErr := s.docker.ContainerStop(ctx, s.config.ContainerName, 10*time.Second); stopErr != nil {
					s.section.Warn("Failed to stop sandbox before recreate: %v", stopErr)
				}
			}
			if removeErr := s.docker.ContainerRemove(ctx, s.config.ContainerName); removeErr != nil {
				s.section.Warn("Failed to remove sandbox before recreate: %v", removeErr)
			}
			exists = false
			running = false
		}
	}

	if running {
		s.section.Success("Sandbox running (container %s)", s.config.ContainerName)
		s.ensureCLITools(ctx)
		if s.acpShouldRunInSandbox() {
			s.startACPInSandbox(ctx)
		}
		s.state.Store(devops.StateHealthy)
		return nil
	}

	if exists {
		// Container exists but not running -> start it
		s.section.Info("Starting sandbox container %s...", s.config.ContainerName)
		if err := s.docker.ContainerStart(ctx, s.config.ContainerName); err != nil {
			s.state.Store(devops.StateFailed)
			return fmt.Errorf("start sandbox: %w", err)
		}
	} else {
		// Create new container
		s.section.Info("Creating sandbox container %s on :%d...", s.config.ContainerName, s.config.Port)
		opts := s.buildCreateOpts(acpPort)
		if err := s.docker.ContainerCreate(ctx, opts); err != nil {
			s.state.Store(devops.StateFailed)
			return fmt.Errorf("create sandbox: %w", err)
		}
	}

	s.state.Store(devops.StateRunning)

	// Wait for health
	healthURL := fmt.Sprintf("http://localhost:%d/v1/docs", s.config.Port)
	s.health.Register("sandbox", health.Probe{
		Type:   health.ProbeHTTP,
		Target: healthURL,
	})
	if err := s.health.WaitHealthy(ctx, "sandbox", 30*time.Second); err != nil {
		s.state.Store(devops.StateFailed)
		return err
	}

	s.ensureCLITools(ctx)
	if s.acpShouldRunInSandbox() {
		s.startACPInSandbox(ctx)
	}

	s.state.Store(devops.StateHealthy)
	s.section.Success("Sandbox ready (container %s)", s.config.ContainerName)
	return nil
}

func (s *SandboxService) Stop(ctx context.Context) error {
	s.state.Store(devops.StateStopping)

	if !s.isLocalSandboxURL() {
		s.state.Store(devops.StateStopped)
		return nil
	}

	// Stop ACP first
	if s.acpShouldRunInSandbox() {
		s.stopACPInSandbox(ctx)
	}

	running, _ := s.docker.ContainerRunning(ctx, s.config.ContainerName)
	if running {
		s.section.Info("Stopping sandbox container %s...", s.config.ContainerName)
		if err := s.docker.ContainerStop(ctx, s.config.ContainerName, 10*time.Second); err != nil {
			s.state.Store(devops.StateFailed)
			return err
		}
	}

	s.state.Store(devops.StateStopped)
	return nil
}

func (s *SandboxService) needsRecreate(ctx context.Context) bool {
	info, err := s.docker.ContainerInspect(ctx, s.config.ContainerName)
	if err != nil {
		return true
	}

	// Check workspace mount
	if s.config.WorkspaceDir != "" {
		hasMount := false
		for _, m := range info.Mounts {
			if m.Destination == "/workspace" && m.Source == s.config.WorkspaceDir {
				hasMount = true
				break
			}
		}
		if !hasMount {
			return true
		}
	}

	// Check ACP port mapping
	if s.acpShouldRunInSandbox() && s.config.ACPPort > 0 {
		hasPort := false
		for _, p := range info.Ports {
			if p.HostPort == s.config.ACPPort {
				hasPort = true
				break
			}
		}
		if !hasPort {
			return true
		}
	}

	return false
}

// detectExistingACPPort finds the ACP host port from an existing container's
// port mappings, skipping the sandbox port itself.
func (s *SandboxService) detectExistingACPPort(ctx context.Context) int {
	info, err := s.docker.ContainerInspect(ctx, s.config.ContainerName)
	if err != nil {
		return 0
	}
	for _, p := range info.Ports {
		if p.HostPort != s.config.Port && p.HostPort > 0 {
			return p.HostPort
		}
	}
	return 0
}

func (s *SandboxService) buildCreateOpts(acpPort int) docker.CreateOpts {
	ports := map[int]int{
		8080: s.config.Port, // sandbox internal -> host
	}
	if s.acpShouldRunInSandbox() && acpPort > 0 {
		ports[acpPort] = acpPort
	}

	volumes := make(map[string]string)
	if s.config.WorkspaceDir != "" {
		volumes[s.config.WorkspaceDir] = "/workspace"
	}

	env := map[string]string{}
	acpContainerHost := "host.docker.internal"
	if acpPort > 0 {
		env["ACP_SERVER_HOST"] = acpContainerHost
		env["ACP_SERVER_PORT"] = fmt.Sprintf("%d", acpPort)
		env["ACP_SERVER_ADDR"] = fmt.Sprintf("%s:%d", acpContainerHost, acpPort)
	}

	return docker.CreateOpts{
		Name:       s.config.ContainerName,
		Image:      s.config.Image,
		Ports:      ports,
		Volumes:    volumes,
		Env:        env,
		ExtraHosts: []string{"host.docker.internal:host-gateway"},
	}
}

func (s *SandboxService) isLocalSandboxURL() bool {
	u, err := url.Parse(s.config.BaseURL)
	if err != nil {
		return false
	}
	host := u.Hostname()
	return host == "localhost" || host == "127.0.0.1" || host == "0.0.0.0"
}

func (s *SandboxService) acpShouldRunInSandbox() bool {
	return s.config.StartACPWithSandbox &&
		s.config.ACPRunMode == "sandbox" &&
		s.isLocalSandboxURL()
}

func (s *SandboxService) ensureCLITools(ctx context.Context) {
	if !s.config.AutoInstallCLI {
		return
	}

	s.section.Info("Ensuring Codex + Claude Code inside sandbox...")
	_, err := s.docker.Exec(ctx, s.config.ContainerName, []string{
		"sh", "-lc",
		`fail=0
if ! command -v codex >/dev/null 2>&1; then npm i -g @openai/codex || fail=1; fi
if ! command -v claude >/dev/null 2>&1; then npm i -g @anthropic-ai/claude-code || fail=1; fi
exit "$fail"`,
	}, docker.ExecOpts{})
	if err != nil {
		s.section.Warn("Sandbox CLI install failed; verify npm/Node connectivity")
	}
}

func (s *SandboxService) ensureACPBinary(ctx context.Context) error {
	// Detect container arch
	arch, _ := s.docker.Exec(ctx, s.config.ContainerName, []string{"uname", "-m"}, docker.ExecOpts{})
	goarch := "amd64"
	switch strings.TrimSpace(arch) {
	case "aarch64", "arm64":
		goarch = "arm64"
	}

	outBin := filepath.Join(s.config.PIDDir, fmt.Sprintf("alex-linux-%s", goarch))

	// Check if rebuild needed
	rebuild := false
	if _, err := os.Stat(outBin); os.IsNotExist(err) {
		rebuild = true
	} else {
		info, _ := os.Stat(outBin)
		if info != nil {
			// Check if any Go source is newer
			newer := false
			for _, dir := range []string{"cmd", "internal"} {
				srcDir := filepath.Join(s.config.ProjectDir, dir)
				if err := filepath.Walk(srcDir, func(path string, fi os.FileInfo, err error) error {
					if err != nil {
						return nil
					}
					if strings.HasSuffix(path, ".go") && fi.ModTime().After(info.ModTime()) {
						newer = true
					}
					return nil
				}); err != nil {
					s.section.Warn("Walking source tree failed for %s: %v", srcDir, err)
				}
			}
			if newer {
				rebuild = true
			}
		}
	}

	if rebuild {
		s.section.Info("Building linux alex (%s) for sandbox...", goarch)
		goToolchain := filepath.Join(s.config.ProjectDir, "scripts", "go-with-toolchain.sh")
		cmd := exec.CommandContext(ctx, goToolchain, "build", "-o", outBin, "./cmd/alex")
		cmd.Dir = s.config.ProjectDir
		cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH="+goarch, "CGO_ENABLED=0")
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("build linux alex: %s: %w", string(out), err)
		}
	}

	// Copy into sandbox
	s.section.Info("Copying alex CLI into sandbox...")
	if _, err := s.docker.Exec(ctx, s.config.ContainerName, []string{"mkdir", "-p", "/usr/local/bin"}, docker.ExecOpts{}); err != nil {
		return fmt.Errorf("prepare sandbox bin dir: %w", err)
	}
	if err := s.docker.CopyTo(ctx, s.config.ContainerName, outBin, "/usr/local/bin/alex"); err != nil {
		return fmt.Errorf("copy alex to sandbox: %w", err)
	}
	if _, err := s.docker.Exec(ctx, s.config.ContainerName, []string{"chmod", "+x", "/usr/local/bin/alex"}, docker.ExecOpts{}); err != nil {
		return fmt.Errorf("chmod alex in sandbox: %w", err)
	}

	return nil
}

func (s *SandboxService) ensureSandboxConfig(ctx context.Context) (bool, error) {
	hostConfig := ""
	if v, ok := os.LookupEnv("ALEX_CONFIG_PATH"); ok {
		hostConfig = v
	}
	if strings.TrimSpace(hostConfig) == "" {
		home, _ := os.UserHomeDir()
		hostConfig = filepath.Join(home, ".alex", "config.yaml")
	}

	if _, err := os.Stat(hostConfig); os.IsNotExist(err) {
		return false, nil
	}

	// Create dir, copy config, check if different
	if _, err := s.docker.Exec(ctx, s.config.ContainerName, []string{"sh", "-lc", "mkdir -p /root/.alex"}, docker.ExecOpts{}); err != nil {
		return false, err
	}
	if err := s.docker.CopyTo(ctx, s.config.ContainerName, hostConfig, "/tmp/alex-config.yaml"); err != nil {
		return false, err
	}

	result, _ := s.docker.Exec(ctx, s.config.ContainerName, []string{"sh", "-lc", fmt.Sprintf(`
if command -v cmp >/dev/null 2>&1 && [ -f %s ]; then
  if cmp -s %s /tmp/alex-config.yaml; then
    rm -f /tmp/alex-config.yaml
    echo same
    exit 0
  fi
fi
mv /tmp/alex-config.yaml %s
chmod 600 %s
echo updated`, s.config.SandboxConfigPath, s.config.SandboxConfigPath, s.config.SandboxConfigPath, s.config.SandboxConfigPath)}, docker.ExecOpts{})

	updated := strings.TrimSpace(result) == "updated"
	if updated {
		s.section.Info("Updated sandbox alex config from host")
	}
	return updated, nil
}

func (s *SandboxService) startACPInSandbox(ctx context.Context) {
	if err := s.ensureACPBinary(ctx); err != nil {
		s.section.Warn("Failed to ensure ACP binary: %v", err)
		return
	}

	configUpdated, _ := s.ensureSandboxConfig(ctx)

	// Collect LLM env vars to pass into sandbox
	envFlags := s.collectSandboxEnvFlags()

	forceRestart := "0"
	if configUpdated {
		forceRestart = "1"
	}

	envFlags["ACP_PORT"] = fmt.Sprintf("%d", s.config.ACPPort)
	envFlags["ACP_FORCE_RESTART"] = forceRestart

	s.section.Info("Starting ACP daemon inside sandbox on 0.0.0.0:%d...", s.config.ACPPort)

	_, _ = s.docker.Exec(ctx, s.config.ContainerName, []string{"sh", "-lc", fmt.Sprintf(`
if [ -f /tmp/acp.pid ] && kill -0 $(cat /tmp/acp.pid) 2>/dev/null; then
  if [ "${ACP_FORCE_RESTART}" = "1" ]; then
    kill "$(cat /tmp/acp.pid)" 2>/dev/null || true
    rm -f /tmp/acp.pid
  else
    exit 0
  fi
fi
if [ -d /workspace ]; then cd /workspace; fi
nohup /usr/local/bin/alex acp serve --host 0.0.0.0 --port "%d" >/tmp/acp.log 2>&1 &
echo $! >/tmp/acp.pid
`, s.config.ACPPort)}, docker.ExecOpts{Env: envFlags})
}

func (s *SandboxService) stopACPInSandbox(ctx context.Context) {
	running, _ := s.docker.ContainerRunning(ctx, s.config.ContainerName)
	if !running {
		return
	}
	if _, err := s.docker.Exec(ctx, s.config.ContainerName, []string{"sh", "-lc", `
if [ -f /tmp/acp.pid ]; then
  pid=$(cat /tmp/acp.pid)
  kill "$pid" 2>/dev/null || true
  rm -f /tmp/acp.pid
fi`}, docker.ExecOpts{}); err != nil {
		s.section.Warn("Failed to stop ACP daemon inside sandbox: %v", err)
	}
}

func (s *SandboxService) collectSandboxEnvFlags() map[string]string {
	env := make(map[string]string)
	keys := []string{
		"LLM_PROVIDER", "LLM_MODEL",
		"LLM_SMALL_PROVIDER", "LLM_SMALL_MODEL",
		"LLM_VISION_MODEL", "LLM_BASE_URL",
		"OPENAI_API_KEY", "OPENAI_BASE_URL",
		"ANTHROPIC_API_KEY", "ANTHROPIC_BASE_URL",
		"CODEX_API_KEY", "CODEX_BASE_URL",
		"ARK_API_KEY",
	}
	for _, k := range keys {
		if v, ok := os.LookupEnv(k); ok && v != "" {
			if strings.HasSuffix(k, "_BASE_URL") || k == "LLM_BASE_URL" {
				v = rewriteBaseURLForSandbox(v)
			}
			env[k] = v
		}
	}
	return env
}

func rewriteBaseURLForSandbox(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	host := u.Hostname()
	if host == "localhost" || host == "127.0.0.1" || host == "0.0.0.0" {
		u.Host = strings.Replace(u.Host, host, "host.docker.internal", 1)
	}
	return u.String()
}

// DetectGoArch returns the GOARCH for the current machine.
func DetectGoArch() string {
	switch runtime.GOARCH {
	case "arm64":
		return "arm64"
	default:
		return "amd64"
	}
}
