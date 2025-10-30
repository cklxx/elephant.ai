package tools

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestShouldManageSandboxDocker(t *testing.T) {
	t.Parallel()

	cases := []struct {
		url      string
		expected bool
	}{
		{"http://localhost:8090", true},
		{"http://127.0.0.1:9000", true},
		{"http://[::1]:8080", true},
		{"http://example.com:8090", false},
		{"", false},
	}

	for _, tc := range cases {
		if got := shouldManageSandboxDocker(tc.url); got != tc.expected {
			t.Fatalf("shouldManageSandboxDocker(%q) = %v, expected %v", tc.url, got, tc.expected)
		}
	}
}

func TestExecSandboxDockerControllerReusesOpenPort(t *testing.T) {
	t.Parallel()

	dialer := newStubDialer()
	dialer.set("127.0.0.1:8090", dialResponse{conn: stubNetConn{}})
	dialer.set("ghcr.io:443", dialResponse{conn: stubNetConn{}})
	dialer.set("api.openai.com:443", dialResponse{conn: stubNetConn{}})
	dialer.set(sandboxChinaRegistryEndpoint, dialResponse{err: errors.New("network unreachable")})
	dialer.set("aliyun.com:443", dialResponse{err: errors.New("network unreachable")})
	dialer.set("baidu.com:443", dialResponse{err: errors.New("network unreachable")})
	cli := &stubDockerCLI{}
	controller := &execSandboxDockerController{
		cli:    cli,
		dialFn: dialer.DialContext,
	}

	ctx := context.Background()
	result, err := controller.EnsureRunning(ctx, "http://localhost:8090")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Reused || result.Started {
		t.Fatalf("expected reused result, got %+v", result)
	}
	if len(cli.calls) != 0 {
		t.Fatalf("expected no docker commands when port already open, got %d", len(cli.calls))
	}
	if len(dialer.calls) == 0 {
		t.Fatalf("expected dialer to be invoked")
	}
}

func TestExecSandboxDockerControllerStartsContainer(t *testing.T) {
	t.Setenv("ALEX_SKILLS_DIR", "")
	t.Setenv("ALEX_SANDBOX_IMAGE", sandboxDockerImage)

	dialer := newStubDialer()
	dialer.set("127.0.0.1:8090", dialResponse{err: errors.New("connection refused")})
	dialer.set("ghcr.io:443", dialResponse{conn: stubNetConn{}})
	dialer.set("api.openai.com:443", dialResponse{conn: stubNetConn{}})
	dialer.set(sandboxChinaRegistryEndpoint, dialResponse{err: errors.New("network unreachable")})
	dialer.set("aliyun.com:443", dialResponse{err: errors.New("network unreachable")})
	dialer.set("baidu.com:443", dialResponse{err: errors.New("network unreachable")})
	cli := &stubDockerCLI{results: []commandResult{
		{output: ""},             // docker ps
		{output: ""},             // docker ps -a
		{output: "container-id"}, // docker run
	}}

	controller := &execSandboxDockerController{
		cli:    cli,
		dialFn: dialer.DialContext,
	}

	ctx := context.Background()
	result, err := controller.EnsureRunning(ctx, "http://localhost:8090")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Started || result.Reused {
		t.Fatalf("expected started result, got %+v", result)
	}
	if len(cli.calls) != 3 {
		t.Fatalf("expected three docker commands, got %d", len(cli.calls))
	}
	if cmd := cli.calls[0]; len(cmd) == 0 || cmd[0] != "ps" {
		t.Fatalf("expected first command to be docker ps, got %v", cmd)
	}
	if cmd := cli.calls[2]; len(cmd) == 0 || cmd[0] != "run" {
		t.Fatalf("expected final command to be docker run, got %v", cmd)
	}
	if cmd := cli.calls[2]; len(cmd) < 1 || cmd[len(cmd)-1] != sandboxDockerImage {
		t.Fatalf("expected sandbox image %q, got %v", sandboxDockerImage, cmd)
	}
	if result.Image != sandboxDockerImage {
		t.Fatalf("expected docker image %q, got %q", sandboxDockerImage, result.Image)
	}
}

func TestExecSandboxDockerControllerStartsContainerChinaMirror(t *testing.T) {
	t.Setenv("ALEX_SKILLS_DIR", "")
	t.Setenv("ALEX_SANDBOX_IMAGE", "")

	dialer := newStubDialer()
	dialer.set("127.0.0.1:8090", dialResponse{err: errors.New("connection refused")})
	dialer.set("ghcr.io:443", dialResponse{err: errors.New("network unreachable")})
	dialer.set("api.openai.com:443", dialResponse{err: errors.New("network unreachable")})
	dialer.set(sandboxChinaRegistryEndpoint, dialResponse{conn: stubNetConn{}})
	dialer.set("aliyun.com:443", dialResponse{conn: stubNetConn{}})
	dialer.set("baidu.com:443", dialResponse{conn: stubNetConn{}})
	cli := &stubDockerCLI{results: []commandResult{
		{output: "", err: errors.New("not found")}, // docker image inspect
		{output: ""},             // docker ps
		{output: ""},             // docker ps -a
		{output: "container-id"}, // docker run
	}}

	controller := &execSandboxDockerController{
		cli:    cli,
		dialFn: dialer.DialContext,
	}

	ctx := context.Background()
	result, err := controller.EnsureRunning(ctx, "http://localhost:8090")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Started || result.Reused {
		t.Fatalf("expected started result, got %+v", result)
	}
	if result.Image != sandboxDockerImageChina {
		t.Fatalf("expected docker image %q, got %q", sandboxDockerImageChina, result.Image)
	}
	runCmd := cli.calls[len(cli.calls)-1]
	if len(runCmd) == 0 || runCmd[0] != "run" {
		t.Fatalf("expected final command to be docker run, got %v", runCmd)
	}
	if runCmd[len(runCmd)-1] != sandboxDockerImageChina {
		t.Fatalf("expected sandbox image %q, got %v", sandboxDockerImageChina, runCmd)
	}
}

func TestExecSandboxDockerControllerIncludesSkillsMount(t *testing.T) {
	dir := t.TempDir()
	skills := filepath.Join(dir, "skills")
	if err := os.Mkdir(skills, 0o755); err != nil {
		t.Fatalf("failed to create skills dir: %v", err)
	}
	t.Setenv("ALEX_SKILLS_DIR", skills)
	t.Setenv("ALEX_SANDBOX_IMAGE", sandboxDockerImage)

	dialer := newStubDialer()
	dialer.set("127.0.0.1:7777", dialResponse{err: errors.New("connection refused")})
	dialer.set("ghcr.io:443", dialResponse{conn: stubNetConn{}})
	dialer.set("api.openai.com:443", dialResponse{conn: stubNetConn{}})
	dialer.set(sandboxChinaRegistryEndpoint, dialResponse{err: errors.New("network unreachable")})
	dialer.set("aliyun.com:443", dialResponse{err: errors.New("network unreachable")})
	dialer.set("baidu.com:443", dialResponse{err: errors.New("network unreachable")})
	cli := &stubDockerCLI{results: []commandResult{
		{output: ""},
		{output: ""},
		{output: "container-id"},
	}}

	controller := &execSandboxDockerController{
		cli:    cli,
		dialFn: dialer.DialContext,
	}

	ctx := context.Background()
	if _, err := controller.EnsureRunning(ctx, "http://localhost:7777"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cli.calls) < 1 {
		t.Fatalf("expected docker commands to be issued")
	}
	run := cli.calls[len(cli.calls)-1]
	if !containsMount(run, skills) {
		t.Fatalf("expected run command to include skills mount for %s, got %v", skills, run)
	}
}

func TestResolveSandboxImagePrefersEnvOverride(t *testing.T) {
	const custom = "example.com/custom-sandbox:latest"
	t.Setenv("ALEX_SANDBOX_IMAGE", custom)

	controller := &execSandboxDockerController{cli: &stubDockerCLI{}}
	image := controller.resolveSandboxImage(context.Background())
	if image != custom {
		t.Fatalf("expected custom sandbox image %q, got %q", custom, image)
	}
}

func TestResolveSandboxImageUsesLocalImage(t *testing.T) {
	t.Setenv("ALEX_SANDBOX_IMAGE", "")

	cli := &stubDockerCLI{results: []commandResult{{output: "[]"}}}
	dialer := newStubDialer()
	dialer.set("ghcr.io:443", dialResponse{conn: stubNetConn{}})
	dialer.set("api.openai.com:443", dialResponse{conn: stubNetConn{}})
	dialer.set(sandboxChinaRegistryEndpoint, dialResponse{err: errors.New("network unreachable")})
	dialer.set("aliyun.com:443", dialResponse{err: errors.New("network unreachable")})
	dialer.set("baidu.com:443", dialResponse{err: errors.New("network unreachable")})
	controller := &execSandboxDockerController{cli: cli, dialFn: dialer.DialContext}

	image := controller.resolveSandboxImage(context.Background())
	if image != customSandboxDockerImage {
		t.Fatalf("expected local sandbox image %q, got %q", customSandboxDockerImage, image)
	}
	if len(cli.calls) != 1 || len(cli.calls[0]) < 2 || cli.calls[0][0] != "image" || cli.calls[0][1] != "inspect" {
		t.Fatalf("expected docker image inspect call, got %v", cli.calls)
	}
}

func TestResolveSandboxImageDetectsChinaNetworkViaEnv(t *testing.T) {
	unsetEnv(t, "ALEX_IN_CHINA")
	unsetEnv(t, "ALEX_NETWORK_REGION")
	t.Setenv("ALEX_SANDBOX_IMAGE", "")
	t.Setenv("ALEX_SKILLS_DIR", "")
	t.Setenv("ALEX_IN_CHINA", "true")

	dialer := newStubDialer()
	dialer.set("ghcr.io:443", dialResponse{err: errors.New("network unreachable")})
	dialer.set("api.openai.com:443", dialResponse{err: errors.New("network unreachable")})
	dialer.set(sandboxChinaRegistryEndpoint, dialResponse{conn: stubNetConn{}})
	cli := &stubDockerCLI{results: []commandResult{{err: errors.New("not found")}}}
	controller := &execSandboxDockerController{cli: cli, dialFn: dialer.DialContext}

	image := controller.resolveSandboxImage(context.Background())
	if image != sandboxDockerImageChina {
		t.Fatalf("expected china sandbox image, got %q", image)
	}
}

func TestChinaDetectionSetsEnvironmentDefaults(t *testing.T) {
	unsetEnv(t, "ALEX_IN_CHINA")
	unsetEnv(t, "ALEX_NETWORK_REGION")
	t.Setenv("ALEX_SANDBOX_IMAGE", "")

	dialer := newStubDialer()
	dialer.set("ghcr.io:443", dialResponse{err: errors.New("network unreachable")})
	dialer.set("api.openai.com:443", dialResponse{err: errors.New("network unreachable")})
	dialer.set(sandboxChinaRegistryEndpoint, dialResponse{conn: stubNetConn{}})
	dialer.set("aliyun.com:443", dialResponse{conn: stubNetConn{}})
	dialer.set("baidu.com:443", dialResponse{conn: stubNetConn{}})

	cli := &stubDockerCLI{results: []commandResult{{err: errors.New("not found")}}}
	controller := &execSandboxDockerController{cli: cli, dialFn: dialer.DialContext}
	image := controller.resolveSandboxImage(context.Background())
	if image != sandboxDockerImageChina {
		t.Fatalf("expected china sandbox image, got %q", image)
	}

	if value, ok := os.LookupEnv("ALEX_NETWORK_REGION"); !ok || value != "china" {
		t.Fatalf("expected ALEX_NETWORK_REGION to be set to china, got %q (set=%v)", value, ok)
	}
	if value, ok := os.LookupEnv("ALEX_IN_CHINA"); !ok || strings.ToLower(value) != "true" {
		t.Fatalf("expected ALEX_IN_CHINA to default to true, got %q (set=%v)", value, ok)
	}
}

func TestExecSandboxDockerControllerDockerMissing(t *testing.T) {
	t.Parallel()

	dialer := newStubDialer()
	dialer.set("127.0.0.1:8090", dialResponse{err: errors.New("connection refused")})
	dialer.set("ghcr.io:443", dialResponse{conn: stubNetConn{}})
	dialer.set("api.openai.com:443", dialResponse{conn: stubNetConn{}})
	dialer.set(sandboxChinaRegistryEndpoint, dialResponse{err: errors.New("network unreachable")})
	dialer.set("aliyun.com:443", dialResponse{err: errors.New("network unreachable")})
	dialer.set("baidu.com:443", dialResponse{err: errors.New("network unreachable")})
	cli := &stubDockerCLI{lookPathErr: errors.New("not found")}

	controller := &execSandboxDockerController{
		cli:    cli,
		dialFn: dialer.DialContext,
	}

	ctx := context.Background()
	_, err := controller.EnsureRunning(ctx, "http://localhost:8090")
	if err == nil || !strings.Contains(err.Error(), "docker CLI not found") {
		t.Fatalf("expected docker missing error, got %v", err)
	}
	if len(cli.calls) != 0 {
		t.Fatalf("expected no docker commands when CLI missing")
	}
}

func TestExecSandboxDockerControllerRemoteBaseURL(t *testing.T) {
	t.Parallel()

	dialer := newStubDialer()
	dialer.set("127.0.0.1:8090", dialResponse{err: errors.New("connection refused")})
	dialer.set("ghcr.io:443", dialResponse{conn: stubNetConn{}})
	dialer.set("api.openai.com:443", dialResponse{conn: stubNetConn{}})
	dialer.set(sandboxChinaRegistryEndpoint, dialResponse{err: errors.New("network unreachable")})
	dialer.set("aliyun.com:443", dialResponse{err: errors.New("network unreachable")})
	dialer.set("baidu.com:443", dialResponse{err: errors.New("network unreachable")})
	cli := &stubDockerCLI{}

	controller := &execSandboxDockerController{
		cli:    cli,
		dialFn: dialer.DialContext,
	}

	ctx := context.Background()
	result, err := controller.EnsureRunning(ctx, "https://example.com:9999")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Started || result.Reused {
		t.Fatalf("expected no action for remote base URL, got %+v", result)
	}
	if len(dialer.calls) != 0 {
		t.Fatalf("expected no dial attempts for remote base URL")
	}
	if len(cli.calls) != 0 {
		t.Fatalf("expected no docker commands for remote base URL")
	}
}

func containsMount(args []string, dir string) bool {
	target := fmt.Sprintf("type=bind,source=%s,destination=/workspace/skills,readonly", dir)
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--mount" && args[i+1] == target {
			return true
		}
	}
	return false
}

func unsetEnv(t *testing.T, key string) {
	t.Helper()
	original, ok := os.LookupEnv(key)
	if ok {
		t.Cleanup(func() {
			_ = os.Setenv(key, original)
		})
	} else {
		t.Cleanup(func() {
			_ = os.Unsetenv(key)
		})
	}
	_ = os.Unsetenv(key)
}

type stubDockerCLI struct {
	lookPathErr error
	results     []commandResult
	calls       [][]string
}

type commandResult struct {
	output string
	err    error
}

func (s *stubDockerCLI) LookPath(string) (string, error) {
	if s.lookPathErr != nil {
		return "", s.lookPathErr
	}
	return "/usr/bin/docker", nil
}

func (s *stubDockerCLI) Run(ctx context.Context, args ...string) (string, error) {
	s.calls = append(s.calls, append([]string(nil), args...))
	if len(s.results) == 0 {
		return "", nil
	}
	res := s.results[0]
	s.results = s.results[1:]
	return res.output, res.err
}

type dialResponse struct {
	conn net.Conn
	err  error
}

type stubDialer struct {
	responses map[string]dialResponse
	calls     []string
}

func newStubDialer() *stubDialer {
	return &stubDialer{responses: make(map[string]dialResponse)}
}

func (s *stubDialer) set(address string, res dialResponse) {
	if s.responses == nil {
		s.responses = make(map[string]dialResponse)
	}
	s.responses[address] = res
}

func (s *stubDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	s.calls = append(s.calls, address)
	if res, ok := s.responses[address]; ok {
		if res.err != nil {
			return nil, res.err
		}
		if res.conn != nil {
			return res.conn, nil
		}
		return stubNetConn{}, nil
	}
	return nil, fmt.Errorf("unexpected dial to %s", address)
}

type stubNetConn struct{}

func (stubNetConn) Read(b []byte) (int, error)       { return 0, io.EOF }
func (stubNetConn) Write(b []byte) (int, error)      { return len(b), nil }
func (stubNetConn) Close() error                     { return nil }
func (stubNetConn) LocalAddr() net.Addr              { return &net.TCPAddr{} }
func (stubNetConn) RemoteAddr() net.Addr             { return &net.TCPAddr{} }
func (stubNetConn) SetDeadline(time.Time) error      { return nil }
func (stubNetConn) SetReadDeadline(time.Time) error  { return nil }
func (stubNetConn) SetWriteDeadline(time.Time) error { return nil }
