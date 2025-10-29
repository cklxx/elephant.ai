package tools

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	sandboxDockerImage           = "ghcr.io/agent-infra/sandbox:latest"
	sandboxDockerImageChina      = "enterprise-public-cn-beijing.cr.volces.com/vefaas-public/all-in-one-sandbox:latest"
	customSandboxDockerImage     = "alex-sandbox:latest"
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

	chinaNetworkOnce sync.Once
	chinaNetwork     bool
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
	skillsMount := sandboxSkillsMountArgs()

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
	}

	if len(skillsMount) > 0 {
		runArgs = append(runArgs, skillsMount...)
	}

	runArgs = append(runArgs, image)

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
	if custom := strings.TrimSpace(os.Getenv("ALEX_SANDBOX_IMAGE")); custom != "" {
		return custom
	}

	if c.imageAvailable(ctx, customSandboxDockerImage) {
		return customSandboxDockerImage
	}

	inChina := c.isChinaNetwork(ctx)
	if !inChina && c.canReachHost(ctx, "ghcr.io:443") {
		return sandboxDockerImage
	}
	if c.canReachHost(ctx, sandboxChinaRegistryEndpoint) {
		return sandboxDockerImageChina
	}
	if inChina {
		return sandboxDockerImageChina
	}
	return sandboxDockerImage
}

func (c *execSandboxDockerController) imageAvailable(ctx context.Context, image string) bool {
	image = strings.TrimSpace(image)
	if image == "" || c.cli == nil {
		return false
	}

	if _, err := c.runDocker(ctx, dockerListTimeout, "image", "inspect", image); err != nil {
		return false
	}
	return true
}

func sandboxSkillsMountArgs() []string {
	dir := sandboxSkillsDir()
	if dir == "" {
		return nil
	}
	if strings.ContainsAny(dir, " \t\n") {
		return nil
	}
	mount := fmt.Sprintf("type=bind,source=%s,destination=/workspace/skills,readonly", dir)
	return []string{"--mount", mount}
}

func sandboxSkillsDir() string {
	if value, ok := os.LookupEnv("ALEX_SKILLS_DIR"); ok {
		value = strings.TrimSpace(value)
		if value == "" {
			return ""
		}
		return validateSkillsDir(value)
	}

	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return validateSkillsDir(filepath.Join(wd, "skills"))
}

func validateSkillsDir(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return ""
	}

	info, err := os.Stat(abs)
	if err != nil || !info.IsDir() {
		return ""
	}

	return abs
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

func (c *execSandboxDockerController) isChinaNetwork(ctx context.Context) bool {
	c.chinaNetworkOnce.Do(func() {
		c.chinaNetwork = detectChinaNetwork(ctx, c.dialFn)
		applyChinaNetworkEnv(c.chinaNetwork)
	})
	return c.chinaNetwork
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

func detectChinaNetwork(ctx context.Context, dialFn dialContextFunc) bool {
	if value, ok := chinaNetworkEnvOverride(); ok {
		return value
	}

	if dialFn == nil {
		return false
	}

	if probeHost(ctx, dialFn, "ghcr.io:443") || probeHost(ctx, dialFn, "api.openai.com:443") {
		return false
	}

	chinaEndpoints := []string{
		sandboxChinaRegistryEndpoint,
		"aliyun.com:443",
		"baidu.com:443",
	}
	for _, endpoint := range chinaEndpoints {
		if probeHost(ctx, dialFn, endpoint) {
			return true
		}
	}

	return false
}

func probeHost(ctx context.Context, dialFn dialContextFunc, address string) bool {
	if dialFn == nil || address == "" {
		return false
	}

	dialCtx, cancel := context.WithTimeout(ctx, dockerHostProbeDeadline)
	defer cancel()

	conn, err := dialFn(dialCtx, "tcp", address)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func chinaNetworkEnvOverride() (bool, bool) {
	if value, ok := os.LookupEnv("ALEX_IN_CHINA"); ok {
		if parsed, valid := parseBoolEnv(value); valid {
			return parsed, true
		}
	}

	if value, ok := os.LookupEnv("ALEX_NETWORK_REGION"); ok {
		lower := strings.ToLower(strings.TrimSpace(value))
		switch lower {
		case "china", "cn", "zh", "zh-cn":
			return true, true
		case "global", "intl", "international", "world":
			return false, true
		}
	}

	return false, false
}

func parseBoolEnv(value string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "t", "true", "yes", "y", "on":
		return true, true
	case "0", "f", "false", "no", "n", "off":
		return false, true
	default:
		return false, false
	}
}

func applyChinaNetworkEnv(inChina bool) {
	region := "global"
	chinaValue := "false"
	if inChina {
		region = "china"
		chinaValue = "true"
	}

	setEnvIfUnset("ALEX_NETWORK_REGION", region)
	setEnvIfUnset("ALEX_IN_CHINA", chinaValue)
}

func setEnvIfUnset(key, value string) {
	if value == "" {
		return
	}
	if _, ok := os.LookupEnv(key); ok {
		return
	}
	_ = os.Setenv(key, value)
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
