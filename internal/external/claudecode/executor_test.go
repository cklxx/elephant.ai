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
	stdout  io.ReadCloser
	waitErr error
}

func (f *fakeSubprocess) Start(ctx context.Context) error { return nil }
func (f *fakeSubprocess) Stdout() io.ReadCloser           { return f.stdout }
func (f *fakeSubprocess) Wait() error                     { return f.waitErr }
func (f *fakeSubprocess) Stop() error                     { return nil }

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
