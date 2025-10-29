package tools

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
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
		{"http://localhost:8888", true},
		{"http://127.0.0.1:9000", true},
		{"http://[::1]:8080", true},
		{"http://example.com:8888", false},
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
	dialer.set("127.0.0.1:8888", dialResponse{conn: stubNetConn{}})
	cli := &stubDockerCLI{}
	controller := &execSandboxDockerController{
		cli:    cli,
		dialFn: dialer.DialContext,
	}

	ctx := context.Background()
	result, err := controller.EnsureRunning(ctx, "http://localhost:8888")
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
	t.Parallel()

	dialer := newStubDialer()
	dialer.set("127.0.0.1:8888", dialResponse{err: errors.New("connection refused")})
	dialer.set("ghcr.io:443", dialResponse{conn: stubNetConn{}})
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
	result, err := controller.EnsureRunning(ctx, "http://localhost:8888")
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
	t.Parallel()

	dialer := newStubDialer()
	dialer.set("127.0.0.1:8888", dialResponse{err: errors.New("connection refused")})
	dialer.set("ghcr.io:443", dialResponse{err: errors.New("network unreachable")})
	dialer.set(sandboxChinaRegistryEndpoint, dialResponse{conn: stubNetConn{}})
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
	result, err := controller.EnsureRunning(ctx, "http://localhost:8888")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Started || result.Reused {
		t.Fatalf("expected started result, got %+v", result)
	}
	if result.Image != sandboxDockerImageChina {
		t.Fatalf("expected docker image %q, got %q", sandboxDockerImageChina, result.Image)
	}
	if cmd := cli.calls[2]; len(cmd) < 1 || cmd[len(cmd)-1] != sandboxDockerImageChina {
		t.Fatalf("expected sandbox image %q, got %v", sandboxDockerImageChina, cmd)
	}
}

func TestExecSandboxDockerControllerDockerMissing(t *testing.T) {
	t.Parallel()

	dialer := newStubDialer()
	dialer.set("127.0.0.1:8888", dialResponse{err: errors.New("connection refused")})
	cli := &stubDockerCLI{lookPathErr: errors.New("not found")}

	controller := &execSandboxDockerController{
		cli:    cli,
		dialFn: dialer.DialContext,
	}

	ctx := context.Background()
	_, err := controller.EnsureRunning(ctx, "http://localhost:8888")
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
	dialer.set("127.0.0.1:8888", dialResponse{err: errors.New("connection refused")})
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
