package integration

import (
	"context"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/react"
	"alex/internal/infra/external/bridge"
	"alex/internal/infra/tools/builtin/orchestration"
)

func TestAgentTeamsKimiInjectE2E_ParallelTemplate(t *testing.T) {
	pythonBin, err := osexec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not found in PATH")
	}

	repoRoot := findRepoRoot(t)
	bridgeScript := filepath.Join(repoRoot, "scripts", "codex_bridge", "codex_bridge.py")
	if _, err := os.Stat(bridgeScript); err != nil {
		t.Skipf("codex bridge script not found at %s", bridgeScript)
	}

	workspace := t.TempDir()
	t.Chdir(workspace)

	fakeKimi := writeFakeKimiCLI(t, workspace)
	recorder := newRecordingExternalExecutor(bridge.New(bridge.BridgeConfig{
		AgentType:          "kimi",
		Binary:             fakeKimi,
		PythonBinary:       pythonBin,
		BridgeScript:       bridgeScript,
		ApprovalPolicy:     "never",
		Sandbox:            "read-only",
		PlanApprovalPolicy: "never",
		PlanSandbox:        "read-only",
		Timeout:            30 * time.Second,
		Env: map[string]string{
			"FAKE_KIMI_MARKER":        "FAKE_KIMI_OK",
			"FAKE_KIMI_SLEEP_SECONDS": "0.35",
		},
	}))

	mgr := react.NewBackgroundTaskManager(react.BackgroundManagerConfig{
		RunContext: context.Background(),
		Logger:     agent.NoopLogger{},
		Clock:      agent.SystemClock{},
		ExecuteTask: func(ctx context.Context, prompt, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
			return &agent.TaskResult{Answer: "internal-not-used", Iterations: 1, TokensUsed: 1}, nil
		},
		ExternalExecutor: recorder,
		SessionID:        "teams-kimi-e2e",
	})
	defer mgr.Shutdown()

	teamName := "kimi_parallel_plan"
	team := agent.TeamDefinition{
		Name:        teamName,
		Description: "parallel kimi role execution",
		Roles: []agent.TeamRoleDefinition{
			{
				Name:           "worker_a",
				AgentType:      "kimi",
				PromptTemplate: "A role executes: {GOAL}",
				ExecutionMode:  "plan",
				AutonomyLevel:  "full",
				Config: map[string]string{
					"approval_policy": "never",
					"sandbox":         "read-only",
				},
			},
			{
				Name:           "worker_b",
				AgentType:      "kimi",
				PromptTemplate: "B role executes: {GOAL}",
				ExecutionMode:  "plan",
				AutonomyLevel:  "full",
				Config: map[string]string{
					"approval_policy": "never",
					"sandbox":         "read-only",
				},
			},
			{
				Name:           "worker_c",
				AgentType:      "kimi",
				PromptTemplate: "C role executes: {GOAL}",
				ExecutionMode:  "plan",
				AutonomyLevel:  "full",
				Config: map[string]string{
					"approval_policy": "never",
					"sandbox":         "read-only",
				},
			},
		},
		Stages: []agent.TeamStageDefinition{
			{Name: "parallel", Roles: []string{"worker_a", "worker_b", "worker_c"}},
		},
	}

	ctx := context.Background()
	ctx = agent.WithBackgroundDispatcher(ctx, mgr)
	ctx = agent.WithTeamDefinitions(ctx, []agent.TeamDefinition{team})

	tool := orchestration.NewRunTasks()
	goal := "validate kimi cli parallel team execution"
	res, err := tool.Execute(ctx, ports.ToolCall{
		ID: "call-kimi-team-inject-e2e",
		Arguments: map[string]any{
			"template":        teamName,
			"goal":            goal,
			"wait":            true,
			"timeout_seconds": 60,
		},
	})
	if err != nil {
		t.Fatalf("run_tasks execute: %v", err)
	}
	if res.Error != nil {
		t.Fatalf("run_tasks tool error: %v, content=%s", res.Error, res.Content)
	}
	if !strings.Contains(res.Content, "All 3 tasks completed.") {
		t.Fatalf("unexpected run_tasks content: %q", res.Content)
	}
	if !strings.Contains(res.Content, "team-"+teamName) {
		t.Fatalf("run_tasks output missing plan id: %q", res.Content)
	}

	statusPath := filepath.Join(workspace, ".elephant", "tasks", "team-"+teamName+".status.yaml")
	statusData, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatalf("read status file: %v", err)
	}
	statusText := string(statusData)
	if !strings.Contains(statusText, "team-"+teamName) {
		t.Fatalf("status file missing plan id: %s", statusText)
	}

	expectedIDs := []string{"team-worker_a", "team-worker_b", "team-worker_c"}
	results := mgr.Collect(expectedIDs, false, 0)
	if len(results) != len(expectedIDs) {
		t.Fatalf("expected %d results, got %d", len(expectedIDs), len(results))
	}
	for _, r := range results {
		if r.Status != agent.BackgroundTaskStatusCompleted {
			t.Fatalf("task %s status=%s error=%s", r.ID, r.Status, r.Error)
		}
		if !strings.Contains(r.Answer, "FAKE_KIMI_OK") {
			t.Fatalf("task %s answer missing marker: %q", r.ID, r.Answer)
		}
	}

	requests, maxActive := recorder.snapshot()
	if len(requests) != 3 {
		t.Fatalf("expected 3 kimi requests, got %d", len(requests))
	}
	if maxActive < 2 {
		t.Fatalf("expected kimi cli parallelism >=2, got %d", maxActive)
	}
	for _, req := range requests {
		if req.AgentType != "kimi" {
			t.Fatalf("expected request agent_type kimi, got %q", req.AgentType)
		}
		if req.ExecutionMode != "plan" {
			t.Fatalf("expected execution_mode=plan, got %q", req.ExecutionMode)
		}
		if strings.TrimSpace(req.Config["approval_policy"]) != "never" {
			t.Fatalf("expected approval_policy=never, got %q", req.Config["approval_policy"])
		}
		if strings.TrimSpace(req.Config["sandbox"]) != "read-only" {
			t.Fatalf("expected sandbox=read-only, got %q", req.Config["sandbox"])
		}
	}
}

type recordingExternalExecutor struct {
	inner agent.ExternalAgentExecutor

	active    atomic.Int64
	maxActive atomic.Int64

	mu       sync.Mutex
	requests []agent.ExternalAgentRequest
}

func newRecordingExternalExecutor(inner agent.ExternalAgentExecutor) *recordingExternalExecutor {
	return &recordingExternalExecutor{inner: inner}
}

func (r *recordingExternalExecutor) Execute(ctx context.Context, req agent.ExternalAgentRequest) (*agent.ExternalAgentResult, error) {
	active := r.active.Add(1)
	for {
		currentMax := r.maxActive.Load()
		if active <= currentMax {
			break
		}
		if r.maxActive.CompareAndSwap(currentMax, active) {
			break
		}
	}

	r.mu.Lock()
	r.requests = append(r.requests, cloneExternalRequest(req))
	r.mu.Unlock()

	defer r.active.Add(-1)
	return r.inner.Execute(ctx, req)
}

func (r *recordingExternalExecutor) SupportedTypes() []string {
	return r.inner.SupportedTypes()
}

func (r *recordingExternalExecutor) snapshot() ([]agent.ExternalAgentRequest, int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]agent.ExternalAgentRequest, len(r.requests))
	copy(out, r.requests)
	return out, r.maxActive.Load()
}

func cloneExternalRequest(req agent.ExternalAgentRequest) agent.ExternalAgentRequest {
	cloned := req
	if req.Config != nil {
		cfg := make(map[string]string, len(req.Config))
		for k, v := range req.Config {
			cfg[k] = v
		}
		cloned.Config = cfg
	}
	return cloned
}

func writeFakeKimiCLI(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "kimi")
	script := `#!/usr/bin/env python3
import json
import os
import sys
import time

if len(sys.argv) < 3 or sys.argv[1] != "exec" or sys.argv[2] != "--json":
    print("unexpected invocation", file=sys.stderr)
    sys.exit(2)

sleep_seconds = float(os.getenv("FAKE_KIMI_SLEEP_SECONDS", "0.35"))
marker = os.getenv("FAKE_KIMI_MARKER", "FAKE_KIMI")

time.sleep(sleep_seconds)

prompt = ""
if "--" in sys.argv:
    idx = sys.argv.index("--")
    if idx + 1 < len(sys.argv):
        prompt = sys.argv[idx + 1]

events = [
    {"type": "thread.started", "thread_id": "fake-kimi-thread"},
    {"type": "item.started", "item": {"type": "command_execution", "command": "echo fake kimi"}},
    {"type": "item.completed", "item": {"type": "agent_message", "text": f"{marker}::{prompt}"}},
    {"type": "turn.completed", "usage": {"input_tokens": 11, "output_tokens": 7}},
]
for event in events:
    print(json.dumps(event), flush=True)
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake kimi cli: %v", err)
	}
	return path
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root")
		}
		dir = parent
	}
}
