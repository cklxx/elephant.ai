package claudecode

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/infra/external/subprocess"
)

// fakeBridgeRunner simulates the Python bridge subprocess.
type fakeBridgeRunner struct {
	stdin      strings.Builder
	stdout     io.Reader
	waitErr    error
	stderrTail string
	started    bool
}

func (f *fakeBridgeRunner) Start(_ context.Context) error {
	f.started = true
	return nil
}

func (f *fakeBridgeRunner) Write(data []byte) error {
	f.stdin.Write(data)
	return nil
}

func (f *fakeBridgeRunner) Stdout() interface{ Read([]byte) (int, error) } {
	return f.stdout
}

func (f *fakeBridgeRunner) StderrTail() string { return f.stderrTail }
func (f *fakeBridgeRunner) Wait() error        { return f.waitErr }
func (f *fakeBridgeRunner) Stop() error         { return nil }

func TestSDKBridgeExecutor_ParsesToolAndResult(t *testing.T) {
	exec := NewSDKBridge(SDKBridgeConfig{
		DefaultMode:  "autonomous",
		Timeout:      2 * time.Second,
		PythonBinary: "/usr/bin/python3",
		BridgeScript: "/fake/cc_bridge.py",
	})

	out := strings.Join([]string{
		`{"type":"tool","tool_name":"Bash","summary":"command=ls -la","files":[],"iter":1}`,
		`{"type":"tool","tool_name":"Write","summary":"file_path=/src/main.go","files":["/src/main.go"],"iter":2}`,
		`{"type":"result","answer":"Done refactoring.","tokens":1500,"cost":0.03,"iters":2,"is_error":false}`,
		"",
	}, "\n")

	fake := &fakeBridgeRunner{stdout: strings.NewReader(out)}
	exec.subprocessFactory = func(_ subprocess.Config) sdkBridgeRunner { return fake }

	var progress []agent.ExternalAgentProgress
	res, err := exec.Execute(context.Background(), agent.ExternalAgentRequest{
		TaskID:    "t1",
		AgentType: "claude_code",
		Prompt:    "refactor auth",
		OnProgress: func(p agent.ExternalAgentProgress) {
			progress = append(progress, p)
		},
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if res == nil {
		t.Fatal("expected result")
	}
	if res.Answer != "Done refactoring." {
		t.Fatalf("unexpected answer: %q", res.Answer)
	}
	if res.TokensUsed != 1500 {
		t.Fatalf("unexpected tokens: %d", res.TokensUsed)
	}
	if res.Iterations != 2 {
		t.Fatalf("unexpected iterations: %d", res.Iterations)
	}
	if len(progress) != 2 {
		t.Fatalf("expected 2 progress events, got %d", len(progress))
	}
	if progress[0].CurrentTool != "Bash" {
		t.Fatalf("first progress tool: %q", progress[0].CurrentTool)
	}
	if progress[0].CurrentArgs != "command=ls -la" {
		t.Fatalf("first progress args: %q", progress[0].CurrentArgs)
	}
	if progress[1].CurrentTool != "Write" {
		t.Fatalf("second progress tool: %q", progress[1].CurrentTool)
	}
	if len(progress[1].FilesTouched) != 1 || progress[1].FilesTouched[0] != "/src/main.go" {
		t.Fatalf("expected file touch: %v", progress[1].FilesTouched)
	}

	// Verify config was written to stdin.
	stdinData := fake.stdin.String()
	if !strings.Contains(stdinData, `"prompt":"refactor auth"`) {
		t.Fatalf("expected prompt in stdin, got %q", stdinData)
	}
}

func TestSDKBridgeExecutor_HandlesErrorEvent(t *testing.T) {
	exec := NewSDKBridge(SDKBridgeConfig{
		DefaultMode:  "autonomous",
		Timeout:      2 * time.Second,
		PythonBinary: "/usr/bin/python3",
		BridgeScript: "/fake/cc_bridge.py",
	})

	out := `{"type":"error","message":"API rate limit exceeded"}` + "\n"
	fake := &fakeBridgeRunner{stdout: strings.NewReader(out)}
	exec.subprocessFactory = func(_ subprocess.Config) sdkBridgeRunner { return fake }

	_, err := exec.Execute(context.Background(), agent.ExternalAgentRequest{
		TaskID:    "t2",
		AgentType: "claude_code",
		Prompt:    "hello",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "API rate limit exceeded") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSDKBridgeExecutor_HandlesResultWithIsError(t *testing.T) {
	exec := NewSDKBridge(SDKBridgeConfig{
		DefaultMode:  "autonomous",
		Timeout:      2 * time.Second,
		PythonBinary: "/usr/bin/python3",
		BridgeScript: "/fake/cc_bridge.py",
	})

	out := `{"type":"result","answer":"context window exceeded","tokens":8000,"cost":0.5,"iters":10,"is_error":true}` + "\n"
	fake := &fakeBridgeRunner{stdout: strings.NewReader(out)}
	exec.subprocessFactory = func(_ subprocess.Config) sdkBridgeRunner { return fake }

	res, err := exec.Execute(context.Background(), agent.ExternalAgentRequest{
		TaskID:    "t3",
		AgentType: "claude_code",
		Prompt:    "hello",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Error != "context window exceeded" {
		t.Fatalf("expected error in result: %q", res.Error)
	}
	if res.TokensUsed != 8000 {
		t.Fatalf("unexpected tokens: %d", res.TokensUsed)
	}
}

func TestSDKBridgeExecutor_EmptyPromptReturnsError(t *testing.T) {
	exec := NewSDKBridge(SDKBridgeConfig{})
	_, err := exec.Execute(context.Background(), agent.ExternalAgentRequest{
		TaskID: "t4",
		Prompt: "",
	})
	if err == nil {
		t.Fatal("expected error for empty prompt")
	}
}

func TestSDKBridgeExecutor_ProcessExitError(t *testing.T) {
	exec := NewSDKBridge(SDKBridgeConfig{
		DefaultMode:  "autonomous",
		Timeout:      2 * time.Second,
		PythonBinary: "/usr/bin/python3",
		BridgeScript: "/fake/cc_bridge.py",
	})

	fake := &fakeBridgeRunner{
		stdout:     strings.NewReader(""),
		waitErr:    fakeExitError{code: 1},
		stderrTail: "not logged in",
	}
	exec.subprocessFactory = func(_ subprocess.Config) sdkBridgeRunner { return fake }

	_, err := exec.Execute(context.Background(), agent.ExternalAgentRequest{
		TaskID:    "t5",
		AgentType: "claude_code",
		Prompt:    "hello",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "exit=1") {
		t.Fatalf("expected exit code: %v", err)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "claude login") {
		t.Fatalf("expected auth hint: %v", err)
	}
}

func TestSDKBridgeExecutor_SkipsInvalidJSON(t *testing.T) {
	exec := NewSDKBridge(SDKBridgeConfig{
		DefaultMode:  "autonomous",
		Timeout:      2 * time.Second,
		PythonBinary: "/usr/bin/python3",
		BridgeScript: "/fake/cc_bridge.py",
	})

	out := strings.Join([]string{
		`not json at all`,
		`{"type":"tool","tool_name":"Edit","summary":"file_path=/a.go","files":["/a.go"],"iter":1}`,
		`{"type":"result","answer":"ok","tokens":100,"cost":0.01,"iters":1,"is_error":false}`,
		"",
	}, "\n")

	fake := &fakeBridgeRunner{stdout: strings.NewReader(out)}
	exec.subprocessFactory = func(_ subprocess.Config) sdkBridgeRunner { return fake }

	var progress []agent.ExternalAgentProgress
	res, err := exec.Execute(context.Background(), agent.ExternalAgentRequest{
		TaskID:    "t6",
		AgentType: "claude_code",
		Prompt:    "hello",
		OnProgress: func(p agent.ExternalAgentProgress) {
			progress = append(progress, p)
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Answer != "ok" {
		t.Fatalf("unexpected answer: %q", res.Answer)
	}
	if len(progress) != 1 {
		t.Fatalf("expected 1 progress event, got %d", len(progress))
	}
}

func TestParseSDKEvent(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		check   func(SDKEvent) error
	}{
		{
			name:  "tool event",
			input: `{"type":"tool","tool_name":"Bash","summary":"command=ls","files":[],"iter":1}`,
			check: func(ev SDKEvent) error {
				if ev.Type != SDKEventTool {
					return fmt.Errorf("type: %q", ev.Type)
				}
				if ev.ToolName != "Bash" {
					return fmt.Errorf("tool_name: %q", ev.ToolName)
				}
				if ev.Summary != "command=ls" {
					return fmt.Errorf("summary: %q", ev.Summary)
				}
				return nil
			},
		},
		{
			name:  "result event",
			input: `{"type":"result","answer":"done","tokens":500,"cost":0.01,"iters":3,"is_error":false}`,
			check: func(ev SDKEvent) error {
				if ev.Type != SDKEventResult {
					return fmt.Errorf("type: %q", ev.Type)
				}
				if ev.Answer != "done" {
					return fmt.Errorf("answer: %q", ev.Answer)
				}
				if ev.Tokens != 500 {
					return fmt.Errorf("tokens: %d", ev.Tokens)
				}
				return nil
			},
		},
		{
			name:  "error event",
			input: `{"type":"error","message":"boom"}`,
			check: func(ev SDKEvent) error {
				if ev.Type != SDKEventError {
					return fmt.Errorf("type: %q", ev.Type)
				}
				if ev.Message != "boom" {
					return fmt.Errorf("message: %q", ev.Message)
				}
				return nil
			},
		},
		{
			name:    "invalid json",
			input:   `not json`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev, err := ParseSDKEvent([]byte(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.check != nil {
				if err := tt.check(ev); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
