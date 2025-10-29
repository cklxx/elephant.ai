package tools

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const (
	sandboxDockerImage           = "ghcr.io/agent-infra/sandbox:latest"
	sandboxDockerImageChina      = "enterprise-public-cn-beijing.cr.volces.com/vefaas-public/all-in-one-sandbox:latest"
	sandboxChinaRegistryEndpoint = "enterprise-public-cn-beijing.cr.volces.com:443"
	sandboxContainerName         = "alex-sandbox"
	sandboxManagedLabel          = "alex.sandbox.managed=true"
	dockerListTimeout            = 5 * time.Second
	dockerStartTimeout           = 10 * time.Second
	dockerRunTimeout             = 45 * time.Second
	dockerPortProbeDeadline      = 750 * time.Millisecond
	dockerHostProbeDeadline      = 1 * time.Second
)

// SandboxDockerResult conveys the outcome of ensuring a sandbox container is running.
type SandboxDockerResult struct {
	Started bool
	Reused  bool
	Image   string
}

// SandboxDockerController prepares a local sandbox Docker container when required.
type SandboxDockerController interface {
	EnsureRunning(ctx context.Context, baseURL string) (SandboxDockerResult, error)
}

type dockerCLI interface {
	LookPath(file string) (string, error)
	Run(ctx context.Context, args ...string) (string, error)
}

type dialContextFunc func(ctx context.Context, network, address string) (net.Conn, error)

type execSandboxDockerController struct {
	cli    dockerCLI
	dialFn dialContextFunc
}

// newExecSandboxDockerController constructs a Docker-backed controller for production use.
func newExecSandboxDockerController() SandboxDockerController {
	return &execSandboxDockerController{
		cli:    execDockerCLI{},
		dialFn: (&net.Dialer{}).DialContext,
	}
}

// EnsureRunning validates the base URL, reuses an existing sandbox container when possible,
// and starts a managed container if required.
func (c *execSandboxDockerController) EnsureRunning(ctx context.Context, baseURL string) (SandboxDockerResult, error) {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return SandboxDockerResult{}, fmt.Errorf("sandbox base URL is required")
	}

	parsed, err := url.Parse(baseURL)
	if err != nil {
		return SandboxDockerResult{}, fmt.Errorf("parse sandbox base URL %q: %w", baseURL, err)
	}

	host := parsed.Hostname()
	if !isLocalHost(host) {
		return SandboxDockerResult{}, nil
	}

	port := parsed.Port()
	if port == "" {
		port = defaultPort(parsed.Scheme)
	}
	if port == "" {
		return SandboxDockerResult{}, fmt.Errorf("sandbox base URL %q is missing a port", baseURL)
	}

	portValue, err := strconv.Atoi(port)
	if err != nil {
		return SandboxDockerResult{}, fmt.Errorf("invalid sandbox port %q: %w", port, err)
	}

	if c.portOpen(ctx, portValue) {
		return SandboxDockerResult{Reused: true}, nil
	}

	if _, err := c.cli.LookPath("docker"); err != nil {
		return SandboxDockerResult{}, fmt.Errorf("docker CLI not found: %w", err)
	}

	image := c.resolveSandboxImage(ctx)

	if output, err := c.runDocker(ctx, dockerListTimeout, "ps", "--filter", managedLabelFilter(), "--format", "{{.ID}}"); err == nil && strings.TrimSpace(output) != "" {
		return SandboxDockerResult{Reused: true, Image: image}, nil
	}

	if output, err := c.runDocker(ctx, dockerListTimeout, "ps", "-a", "--filter", managedLabelFilter(), "--format", "{{.ID}}"); err == nil && strings.TrimSpace(output) != "" {
		if _, err := c.runDocker(ctx, dockerStartTimeout, "start", sandboxContainerName); err == nil {
			return SandboxDockerResult{Started: true, Image: image}, nil
		}
	}

	runArgs := []string{
		"run",
		"--pull=missing",
		"-d",
		"--name", sandboxContainerName,
		"--label", sandboxManagedLabel,
		"--restart", "unless-stopped",
		"--security-opt", "seccomp=unconfined",
		"-p", fmt.Sprintf("%d:8080", portValue),
		image,
	}

	output, err := c.runDocker(ctx, dockerRunTimeout, runArgs...)
	if err != nil {
		if isPortAlreadyAllocated(output) {
			return SandboxDockerResult{Reused: true, Image: image}, nil
		}
		if strings.TrimSpace(output) == "" {
			return SandboxDockerResult{}, fmt.Errorf("start sandbox docker container: %w", err)
		}
		return SandboxDockerResult{}, fmt.Errorf("start sandbox docker container: %s", strings.TrimSpace(output))
	}

	return SandboxDockerResult{Started: true, Image: image}, nil
}

func (c *execSandboxDockerController) runDocker(ctx context.Context, timeout time.Duration, args ...string) (string, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return c.cli.Run(cmdCtx, args...)
}

func (c *execSandboxDockerController) portOpen(ctx context.Context, port int) bool {
	if c.dialFn == nil {
		return false
	}

	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	dialCtx, cancel := context.WithTimeout(ctx, dockerPortProbeDeadline)
	defer cancel()

	conn, err := c.dialFn(dialCtx, "tcp", addr)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func (c *execSandboxDockerController) resolveSandboxImage(ctx context.Context) string {
	if c.canReachHost(ctx, "ghcr.io:443") {
		return sandboxDockerImage
	}
	if c.canReachHost(ctx, sandboxChinaRegistryEndpoint) {
		return sandboxDockerImageChina
	}
	return sandboxDockerImage
}

func (c *execSandboxDockerController) canReachHost(ctx context.Context, address string) bool {
	if c.dialFn == nil {
		return false
	}

	dialCtx, cancel := context.WithTimeout(ctx, dockerHostProbeDeadline)
	defer cancel()

	conn, err := c.dialFn(dialCtx, "tcp", address)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func managedLabelFilter() string {
	return fmt.Sprintf("label=%s", sandboxManagedLabel)
}

func isPortAlreadyAllocated(output string) bool {
	lower := strings.ToLower(output)
	return strings.Contains(lower, "address already in use") || strings.Contains(lower, "port is already allocated")
}

func isLocalHost(host string) bool {
	if host == "" {
		return false
	}

	if strings.EqualFold(host, "localhost") || host == "0.0.0.0" {
		return true
	}
	if host == "::1" {
		return true
	}
	if strings.HasPrefix(host, "127.") {
		return true
	}
	return false
}

func defaultPort(scheme string) string {
	switch strings.ToLower(scheme) {
	case "http":
		return "80"
	case "https":
		return "443"
	default:
		return ""
	}
}

func shouldManageSandboxDocker(baseURL string) bool {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return false
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return false
	}
	if !isLocalHost(parsed.Hostname()) {
		return false
	}
	port := parsed.Port()
	if port == "" {
		port = defaultPort(parsed.Scheme)
	}
	return port != ""
}

type execDockerCLI struct{}

func (execDockerCLI) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

func (execDockerCLI) Run(ctx context.Context, args ...string) (string, error) {
	if len(args) == 0 {
		return "", errors.New("docker command requires arguments")
	}
	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}
