package claudecode

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	agent "alex/internal/agent/ports/agent"
	"alex/internal/external/subprocess"
)

type fakeSubprocess struct {
	stdout     io.ReadCloser
	waitErr    error
	stderrTail string
}

func (f *fakeSubprocess) Start(ctx context.Context) error { return nil }
func (f *fakeSubprocess) Stdout() io.ReadCloser           { return f.stdout }
func (f *fakeSubprocess) Wait() error                     { return f.waitErr }
func (f *fakeSubprocess) Stop() error                     { return nil }
func (f *fakeSubprocess) StderrTail() string              { return f.stderrTail }

func TestExecutor_Execute_ParsesProgressAndUsage(t *testing.T) {
	exec := New(Config{DefaultMode: "autonomous", Timeout: 2 * time.Second})

	out := strings.Join([]string{
		`{"type":"assistant","message":{"tool_use":{"name":"Bash","input":{"cmd":"ls"}}}}`,
		`{"type":"result","output":"done","usage":{"input_tokens":10,"output_tokens":5},"cost":0.02}`,
		"",
	}, "\n")

	exec.subprocessFactory = func(cfg subprocess.Config) subprocessRunner {
		return &fakeSubprocess{stdout: io.NopCloser(strings.NewReader(out))}
	}

	var progress []agent.ExternalAgentProgress
	res, err := exec.Execute(context.Background(), agent.ExternalAgentRequest{
		TaskID:    "t1",
		AgentType: "claude_code",
		Prompt:    "hello",
		OnProgress: func(p agent.ExternalAgentProgress) {
			progress = append(progress, p)
		},
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if res == nil {
		t.Fatalf("expected result")
	}
	if res.Answer != "done" {
		t.Fatalf("unexpected answer: %q", res.Answer)
	}
	if res.TokensUsed != 15 {
		t.Fatalf("unexpected tokens: %d", res.TokensUsed)
	}
	if res.Iterations != 1 {
		t.Fatalf("unexpected iterations: %d", res.Iterations)
	}
	if len(progress) != 1 {
		t.Fatalf("expected 1 progress event, got %d", len(progress))
	}
	if progress[0].CurrentTool != "Bash" {
		t.Fatalf("unexpected tool: %q", progress[0].CurrentTool)
	}
	if !strings.Contains(progress[0].CurrentArgs, "ls") {
		t.Fatalf("unexpected tool args: %q", progress[0].CurrentArgs)
	}
	if progress[0].LastActivity.IsZero() {
		t.Fatalf("expected last activity")
	}
}

type fakeExitError struct {
	code int
}

func (f fakeExitError) Error() string { return "process exit" }
func (f fakeExitError) ExitCode() int { return f.code }

func TestExecutor_Execute_IncludesStderrTailOnFailure(t *testing.T) {
	exec := New(Config{DefaultMode: "autonomous", Timeout: 2 * time.Second})
	exec.subprocessFactory = func(cfg subprocess.Config) subprocessRunner {
		return &fakeSubprocess{
			stdout:     io.NopCloser(strings.NewReader("")),
			waitErr:    fakeExitError{code: 2},
			stderrTail: "not logged in",
		}
	}

	_, err := exec.Execute(context.Background(), agent.ExternalAgentRequest{
		TaskID:    "t2",
		AgentType: "claude_code",
		Prompt:    "hello",
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "stderr tail") {
		t.Fatalf("expected stderr tail in error, got %q", msg)
	}
	if !strings.Contains(msg, "exit=2") {
		t.Fatalf("expected exit code in error, got %q", msg)
	}
	if !strings.Contains(strings.ToLower(msg), "claude login") {
		t.Fatalf("expected auth hint in error, got %q", msg)
	}
}
