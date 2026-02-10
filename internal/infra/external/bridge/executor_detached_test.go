package bridge

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/infra/external/subprocess"
)

// fakeDetachedRunner simulates a detached bridge subprocess that writes to a file.
type fakeDetachedRunner struct {
	fakeBridgeRunner
	outputFile string
	doneFile   string
	pid        int
}

func (f *fakeDetachedRunner) Start(_ context.Context) error {
	f.started = true
	return nil
}

func (f *fakeDetachedRunner) PID() int             { return f.pid }
func (f *fakeDetachedRunner) Done() <-chan struct{} { ch := make(chan struct{}); close(ch); return ch }

func TestExecutor_DetachedMode_ParsesToolAndResult(t *testing.T) {
	dir := t.TempDir()
	workDir := dir

	exec := New(BridgeConfig{
		AgentType:    "claude_code",
		DefaultMode:  "autonomous",
		Timeout:      10 * time.Second,
		PythonBinary: "/usr/bin/python3",
		BridgeScript: "/fake/cc_bridge.py",
		Detached:     true,
	})

	// The fake runner writes events to the output file and done sentinel.
	fake := &fakeDetachedRunner{pid: 12345}
	exec.subprocessFactory = func(cfg subprocess.Config) bridgeRunner {
		fake.outputFile = cfg.OutputFile
		fake.doneFile = filepath.Join(filepath.Dir(cfg.OutputFile), ".done")

		// Simulate bridge writing to file after a short delay.
		go func() {
			time.Sleep(200 * time.Millisecond)
			events := strings.Join([]string{
				`{"type":"tool","tool_name":"Bash","summary":"command=ls","files":[],"iter":1}`,
				`{"type":"result","answer":"detached done","tokens":2000,"cost":0.05,"iters":1,"is_error":false}`,
				"",
			}, "\n")
			_ = os.WriteFile(fake.outputFile, []byte(events), 0o644)
			time.Sleep(100 * time.Millisecond)
			_ = os.WriteFile(fake.doneFile, nil, 0o644)
		}()

		return fake
	}

	var bridgeInfo *BridgeStartedInfo
	var progress []agent.ExternalAgentProgress
	res, err := exec.Execute(context.Background(), agent.ExternalAgentRequest{
		TaskID:    "t-detached-1",
		AgentType: "claude_code",
		Prompt:    "test detached",
		WorkingDir: workDir,
		OnProgress: func(p agent.ExternalAgentProgress) {
			progress = append(progress, p)
		},
		OnBridgeStarted: func(info any) {
			if bi, ok := info.(BridgeStartedInfo); ok {
				bridgeInfo = &bi
			}
		},
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if res == nil {
		t.Fatal("expected result")
	}
	if res.Answer != "detached done" {
		t.Errorf("Answer = %q, want 'detached done'", res.Answer)
	}
	if res.TokensUsed != 2000 {
		t.Errorf("TokensUsed = %d, want 2000", res.TokensUsed)
	}
	if len(progress) != 1 {
		t.Errorf("expected 1 progress event, got %d", len(progress))
	}
	if bridgeInfo == nil {
		t.Error("expected OnBridgeStarted to be called")
	} else {
		if bridgeInfo.PID != 12345 {
			t.Errorf("PID = %d, want 12345", bridgeInfo.PID)
		}
		if bridgeInfo.TaskID != "t-detached-1" {
			t.Errorf("TaskID = %q, want t-detached-1", bridgeInfo.TaskID)
		}
	}

	// Verify the output directory was created.
	outDir := filepath.Join(workDir, ".elephant", "bridge", "t-detached-1")
	if _, err := os.Stat(outDir); os.IsNotExist(err) {
		t.Error("expected output dir to be created")
	}
}

func TestExecutor_DetachedMode_HandlesErrorEvent(t *testing.T) {
	dir := t.TempDir()

	exec := New(BridgeConfig{
		AgentType:    "claude_code",
		DefaultMode:  "autonomous",
		Timeout:      10 * time.Second,
		PythonBinary: "/usr/bin/python3",
		BridgeScript: "/fake/cc_bridge.py",
		Detached:     true,
	})

	fake := &fakeDetachedRunner{pid: 12346}
	exec.subprocessFactory = func(cfg subprocess.Config) bridgeRunner {
		fake.outputFile = cfg.OutputFile
		fake.doneFile = filepath.Join(filepath.Dir(cfg.OutputFile), ".done")

		go func() {
			time.Sleep(200 * time.Millisecond)
			_ = os.WriteFile(fake.outputFile, []byte(`{"type":"error","message":"detached error"}`+"\n"), 0o644)
			time.Sleep(100 * time.Millisecond)
			_ = os.WriteFile(fake.doneFile, nil, 0o644)
		}()

		return fake
	}

	_, err := exec.Execute(context.Background(), agent.ExternalAgentRequest{
		TaskID:     "t-detached-err",
		AgentType:  "claude_code",
		Prompt:     "test error",
		WorkingDir: dir,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "detached error") {
		t.Errorf("error = %q, want to contain 'detached error'", err)
	}
}
