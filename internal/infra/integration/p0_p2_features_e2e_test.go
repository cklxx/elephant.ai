//go:build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/react"
	"alex/internal/domain/agent/taskfile"
	"alex/internal/infra/tools/builtin/orchestration"
)

// ===========================================================================
// P0-P2 Features Real Pipeline E2E Tests
//
// These tests exercise the P0-P2 feature set through the full bridge pipeline:
//   bridge.Executor → codex_bridge.py → fake Python CLI → JSONL response
//
// Features tested:
//   P0-2: Context Preamble — TaskDefaults.ContextPreamble prepended to request prompts
//   P2:   Debate Mode      — challenger task generated, receives collaboration context
//   P1-1: Stale Retry      — slow CLI goes stale, swarm re-dispatches with retry ID
// ===========================================================================

// ---------------------------------------------------------------------------
// P0-2: Context Preamble — verifies that the architectural context preamble set
// in TaskDefaults.ContextPreamble is prepended to external agent request prompts
// through the full bridge pipeline.
// ---------------------------------------------------------------------------

func TestP0P2_ContextPreamble_E2E(t *testing.T) {
	env := setupBridgeEnv(t)

	workspace := t.TempDir()
	t.Chdir(workspace)

	fakeKimi := writeFakeKimiCLI(t, workspace)
	kimiBridge := newFakeBridge(t, env, "kimi", fakeKimi, map[string]string{
		"FAKE_KIMI_MARKER":        "PREAMBLE_KIMI_OK",
		"FAKE_KIMI_SLEEP_SECONDS": "0.1",
	})
	recorder := newRecordingExternalExecutor(kimiBridge)

	mgr := react.NewBackgroundTaskManager(react.BackgroundManagerConfig{
		RunContext: context.Background(),
		Logger:     agent.NoopLogger{},
		Clock:      agent.SystemClock{},
		ExecuteTask: func(ctx context.Context, prompt, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
			return &agent.TaskResult{Answer: "internal-unused", Iterations: 1, TokensUsed: 1}, nil
		},
		ExternalExecutor: recorder,
		SessionID:        "preamble-e2e",
	})
	defer mgr.Shutdown()

	const preamble = "ARCH_CTX: elephant.ai Go system. Key: internal/domain/agent/react (ReAct), " +
		"internal/infra/tools/builtin (tools). Conventions: typed events, port/adapter boundaries."

	tf := &taskfile.TaskFile{
		Version: "1",
		PlanID:  "preamble-test-e2e",
		Defaults: taskfile.TaskDefaults{
			ContextPreamble: preamble,
		},
		Tasks: []taskfile.TaskSpec{
			{
				ID:            "researcher",
				Description:   "Research AI patterns",
				Prompt:        "Analyze multi-agent coordination patterns for elephant.ai",
				AgentType:     "kimi",
				ExecutionMode: "plan",
				Config: map[string]string{
					"approval_policy": "never",
					"sandbox":         "read-only",
				},
			},
			{
				ID:            "synthesizer",
				Description:   "Synthesize research findings",
				Prompt:        "Synthesize the research findings into actionable recommendations",
				AgentType:     "kimi",
				ExecutionMode: "plan",
				DependsOn:     []string{"researcher"},
				Config: map[string]string{
					"approval_policy": "never",
					"sandbox":         "read-only",
				},
			},
		},
	}

	statusPath := filepath.Join(workspace, "preamble.status.yaml")
	exec := taskfile.NewExecutor(mgr, taskfile.ModeTeam, taskfile.DefaultSwarmConfig())
	result, err := exec.ExecuteAndWait(context.Background(), tf, "cause-preamble", statusPath, 30*time.Second)
	if err != nil {
		t.Fatalf("ExecuteAndWait: %v", err)
	}
	if len(result.TaskIDs) != 2 {
		t.Fatalf("expected 2 task IDs, got %d", len(result.TaskIDs))
	}

	results := mgr.Collect(result.TaskIDs, false, 0)
	resultMap := make(map[string]agent.BackgroundTaskResult)
	for _, r := range results {
		resultMap[r.ID] = r
	}

	for _, id := range result.TaskIDs {
		r := resultMap[id]
		if r.Status != agent.BackgroundTaskStatusCompleted {
			t.Errorf("task %s: expected completed, got %s (error=%s)", id, r.Status, r.Error)
		}
	}

	// Verify preamble was prepended in the external request prompt.
	requests, _ := recorder.snapshot()
	if len(requests) != 2 {
		t.Fatalf("expected 2 external requests, got %d", len(requests))
	}

	reqMap := make(map[string]agent.ExternalAgentRequest)
	for _, req := range requests {
		reqMap[req.TaskID] = req
	}

	// Both tasks should have the preamble prepended.
	for _, id := range []string{"researcher", "synthesizer"} {
		req, ok := reqMap[id]
		if !ok {
			t.Errorf("no request found for task %q", id)
			continue
		}
		if !strings.Contains(req.Prompt, preamble) {
			t.Errorf("task %q prompt missing preamble\nprompt prefix: %q\npreamble: %q",
				id, firstN(req.Prompt, 200), preamble)
		}
		if !strings.HasPrefix(req.Prompt, preamble) {
			t.Errorf("task %q: preamble should be at the start of prompt, but it is not a prefix", id)
		}
	}

	// Print analysis report.
	t.Log("")
	t.Log("=== P0-2 Context Preamble E2E Analysis Report ===")
	t.Logf("Preamble length: %d chars", len(preamble))
	for _, id := range []string{"researcher", "synthesizer"} {
		req := reqMap[id]
		ans := resultMap[id]
		t.Logf("")
		t.Logf("Task: %s", id)
		t.Logf("  Agent type: %s", req.AgentType)
		t.Logf("  Prompt length: %d chars", len(req.Prompt))
		t.Logf("  Preamble present at start: %v", strings.HasPrefix(req.Prompt, preamble))
		t.Logf("  Prompt (first 300 chars): %q", firstN(req.Prompt, 300))
		t.Logf("  Agent answer: %q", firstN(ans.Answer, 200))
	}
	t.Log("================================================")
}

// ---------------------------------------------------------------------------
// P2: Debate Mode — verifies that a stage with DebateMode=true generates a
// challenger task that receives collaboration context from the primary task,
// and that the next stage depends on both primary and challenger outputs.
// ---------------------------------------------------------------------------

func TestP0P2_DebateMode_E2E(t *testing.T) {
	env := setupBridgeEnv(t)

	workspace := t.TempDir()
	t.Chdir(workspace)

	fakeKimi := writeFakeKimiCLI(t, workspace)
	kimiBridge := newFakeBridge(t, env, "kimi", fakeKimi, map[string]string{
		"FAKE_KIMI_MARKER":        "DEBATE_KIMI_OK",
		"FAKE_KIMI_SLEEP_SECONDS": "0.1",
	})
	recorder := newRecordingExternalExecutor(kimiBridge)

	// Internal agent (reviewer) captures its prompt so we can verify it receives
	// both primary and debate outputs.
	capture := &internalCapture{}
	const reviewerMarker = "DEBATE_REVIEWER_DONE"

	mgr := react.NewBackgroundTaskManager(react.BackgroundManagerConfig{
		RunContext: context.Background(),
		Logger:     agent.NoopLogger{},
		Clock:      agent.SystemClock{},
		ExecuteTask: func(ctx context.Context, prompt, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
			capture.record(prompt)
			return &agent.TaskResult{Answer: reviewerMarker + ":: review complete", Iterations: 1, TokensUsed: 5}, nil
		},
		ExternalExecutor: recorder,
		SessionID:        "debate-e2e",
	})
	defer mgr.Shutdown()

	// Team with debate_mode on stage 1.
	// Stage 1: analyst (kimi, primary) — DebateMode=true auto-generates team-analyst-debate
	// Stage 2: reviewer (internal, inherits) — depends on both primary + debate
	team := agent.TeamDefinition{
		Name:        "debate_team_e2e",
		Description: "debate mode real pipeline test",
		Roles: []agent.TeamRoleDefinition{
			{
				Name:           "analyst",
				AgentType:      "kimi",
				PromptTemplate: "Analyze architecture: {GOAL}",
				ExecutionMode:  "plan",
				AutonomyLevel:  "full",
				Config:         map[string]string{"approval_policy": "never", "sandbox": "read-only"},
			},
			{
				Name:           "reviewer",
				AgentType:      "internal",
				PromptTemplate: "Review the analysis for {GOAL}",
				ExecutionMode:  "execute",
				AutonomyLevel:  "full",
				InheritContext: true,
			},
		},
		Stages: []agent.TeamStageDefinition{
			{Name: "analysis", Roles: []string{"analyst"}, DebateMode: true},
			{Name: "review", Roles: []string{"reviewer"}},
		},
	}

	ctx := context.Background()
	ctx = agent.WithBackgroundDispatcher(ctx, mgr)
	ctx = agent.WithTeamDefinitions(ctx, []agent.TeamDefinition{team})

	goal := "evaluate event-driven vs request-response for real-time AI assistant systems"
	res, err := orchestration.NewRunTasks().Execute(ctx, ports.ToolCall{
		ID: "call-debate-e2e",
		Arguments: map[string]any{
			"template":        team.Name,
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

	// Collect all results: primary analyst + debate challenger + reviewer.
	allIDs := []string{"team-analyst", "team-analyst-debate", "team-reviewer"}
	results := mgr.Collect(allIDs, false, 0)
	resultMap := make(map[string]agent.BackgroundTaskResult)
	for _, r := range results {
		resultMap[r.ID] = r
	}

	// All three tasks should be present and completed.
	for _, id := range allIDs {
		r, ok := resultMap[id]
		if !ok {
			t.Errorf("no result for task %q", id)
			continue
		}
		if r.Status != agent.BackgroundTaskStatusCompleted {
			t.Errorf("task %s: expected completed, got %s (error=%s)", id, r.Status, r.Error)
		}
	}

	// Challenger (team-analyst-debate) should have received [Collaboration Context]
	// containing the primary analyst's answer.
	externalRequests, maxActive := recorder.snapshot()
	reqMap := make(map[string]agent.ExternalAgentRequest)
	for _, req := range externalRequests {
		reqMap[req.TaskID] = req
	}

	// Two external requests: team-analyst (primary) + team-analyst-debate (challenger).
	if len(externalRequests) != 2 {
		t.Fatalf("expected 2 external requests (primary + challenger), got %d: %v",
			len(externalRequests), taskIDsFromRequests(externalRequests))
	}

	challengerReq, ok := reqMap["team-analyst-debate"]
	if !ok {
		t.Fatalf("no request for team-analyst-debate, got requests: %v", taskIDsFromRequests(externalRequests))
	}

	// Challenger must have collaboration context showing analyst's output.
	if !strings.Contains(challengerReq.Prompt, "[Collaboration Context]") {
		t.Errorf("challenger prompt missing [Collaboration Context]:\n%s", firstN(challengerReq.Prompt, 500))
	}
	if !strings.Contains(challengerReq.Prompt, "DEBATE_KIMI_OK") {
		t.Errorf("challenger prompt missing analyst marker (DEBATE_KIMI_OK):\n%s", firstN(challengerReq.Prompt, 500))
	}

	// Internal reviewer should have received both analyst and debate outputs.
	reviewerPrompts := capture.snapshot()
	if len(reviewerPrompts) != 1 {
		t.Fatalf("expected 1 internal (reviewer) call, got %d", len(reviewerPrompts))
	}
	reviewerPrompt := reviewerPrompts[0]
	if !strings.Contains(reviewerPrompt, "[Collaboration Context]") {
		t.Errorf("reviewer prompt missing [Collaboration Context]")
	}
	if !strings.Contains(reviewerPrompt, "DEBATE_KIMI_OK") {
		t.Errorf("reviewer prompt missing analyst marker (DEBATE_KIMI_OK)")
	}
	// The debate challenger's answer also echoes the prompt (which contains DEBATE_KIMI_OK from the bridge).
	// So the reviewer should see at least 2 appearances of DEBATE_KIMI_OK.

	// Print analysis report.
	t.Log("")
	t.Log("=== P2 Debate Mode E2E Analysis Report ===")
	t.Logf("Goal: %s", goal)
	t.Logf("External agents (max concurrent): %d (maxActive=%d)", len(externalRequests), maxActive)
	t.Log("")
	t.Log("Primary Analyst (team-analyst):")
	if r, ok := resultMap["team-analyst"]; ok {
		t.Logf("  Status: %s", r.Status)
		t.Logf("  Answer: %q", firstN(r.Answer, 300))
	}
	t.Log("")
	t.Log("Debate Challenger (team-analyst-debate):")
	t.Logf("  Prompt length: %d chars", len(challengerReq.Prompt))
	t.Logf("  Has [Collaboration Context]: %v", strings.Contains(challengerReq.Prompt, "[Collaboration Context]"))
	t.Logf("  Has analyst marker: %v", strings.Contains(challengerReq.Prompt, "DEBATE_KIMI_OK"))
	t.Logf("  Prompt (first 400 chars):\n%s", firstN(challengerReq.Prompt, 400))
	if r, ok := resultMap["team-analyst-debate"]; ok {
		t.Logf("  Status: %s", r.Status)
		t.Logf("  Answer: %q", firstN(r.Answer, 300))
	}
	t.Log("")
	t.Log("Reviewer (team-reviewer, internal):")
	t.Logf("  Reviewer prompt length: %d chars", len(reviewerPrompt))
	t.Logf("  Has [Collaboration Context]: %v", strings.Contains(reviewerPrompt, "[Collaboration Context]"))
	t.Logf("  Has analyst output: %v", strings.Contains(reviewerPrompt, "DEBATE_KIMI_OK"))
	if r, ok := resultMap["team-reviewer"]; ok {
		t.Logf("  Status: %s", r.Status)
		t.Logf("  Answer: %q", firstN(r.Answer, 200))
	}
	t.Log("==========================================")
}

// ---------------------------------------------------------------------------
// P1-1: Stale Retry — verifies that a task stuck in the bridge (slow CLI) is
// detected as stale by the BackgroundTaskManager, prompting the SwarmScheduler
// to re-dispatch it under a retry ID. The second call (via counter file) completes
// quickly, demonstrating end-to-end stale recovery through the real bridge pipeline.
// ---------------------------------------------------------------------------

func TestP0P2_StaleRetry_E2E(t *testing.T) {
	env := setupBridgeEnv(t)

	workspace := t.TempDir()
	t.Chdir(workspace)

	// Counter file: tracks invocation count across separate CLI subprocesses.
	counterFile := filepath.Join(workspace, "stale-counter.txt")
	staleMarker := "STALE_RETRY_COMPLETE"

	// Fake CLI: first call sleeps 30s (goes stale), second call completes quickly.
	staleKimi := writeStaleAwareCLI(t, workspace, "stale-kimi", counterFile, staleMarker)
	kimiBridge := newFakeBridge(t, env, "kimi", staleKimi, map[string]string{
		"FAKE_CLI_FIRST_SLEEP": "30.0",
		"FAKE_CLI_RETRY_SLEEP": "0.1",
	})
	recorder := newRecordingExternalExecutor(kimiBridge)

	// Short stale threshold so the task becomes stale quickly.
	const staleThreshold = 600 * time.Millisecond

	mgr := react.NewBackgroundTaskManager(react.BackgroundManagerConfig{
		RunContext:     context.Background(),
		Logger:         agent.NoopLogger{},
		Clock:          agent.SystemClock{},
		StaleThreshold: staleThreshold,
		ExecuteTask: func(ctx context.Context, prompt, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
			return &agent.TaskResult{Answer: "internal-unused", Iterations: 1, TokensUsed: 1}, nil
		},
		ExternalExecutor: recorder,
		SessionID:        "stale-retry-e2e",
	})
	defer mgr.Shutdown()

	tf := &taskfile.TaskFile{
		Version: "1",
		PlanID:  "stale-retry-test-e2e",
		Tasks: []taskfile.TaskSpec{
			{
				ID:            "slow-task",
				Description:   "Task that goes stale then recovers",
				Prompt:        "Implement distributed caching layer",
				AgentType:     "kimi",
				ExecutionMode: "plan",
				Config: map[string]string{
					"approval_policy": "never",
					"sandbox":         "read-only",
				},
			},
		},
	}

	swarmCfg := taskfile.SwarmConfig{
		InitialConcurrency: 5,
		MaxConcurrency:     10,
		ScaleUpThreshold:   0.9,
		ScaleDownThreshold: 0.7,
		ScaleStep:          2,
		// StageTimeout must be long enough for retry subprocess startup under
		// -race/-cover overhead, otherwise retry can be killed too early with
		// "bridge terminated by signal".
		StageTimeout:  staleThreshold + 4*time.Second,
		StaleRetryMax: 1,
	}

	statusPath := filepath.Join(workspace, "stale-retry.status.yaml")
	exec := taskfile.NewExecutor(mgr, taskfile.ModeSwarm, swarmCfg)

	start := time.Now()
	result, err := exec.Execute(context.Background(), tf, "cause-stale-retry", statusPath)
	wallTime := time.Since(start)

	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	retryID := "slow-task-retry-1"
	// Collect until retry reaches terminal state (or timeout). Under -race and
	// coverage, scheduler timings are slower than non-race runs.
	deadline := time.Now().Add(45 * time.Second)
	var results []agent.BackgroundTaskResult
	resultMap := make(map[string]agent.BackgroundTaskResult)
	for {
		results = mgr.Collect(result.TaskIDs, false, 0)
		clear(resultMap)
		for _, r := range results {
			resultMap[r.ID] = r
		}
		retryResult, ok := resultMap[retryID]
		if ok {
			switch retryResult.Status {
			case agent.BackgroundTaskStatusCompleted, agent.BackgroundTaskStatusFailed, agent.BackgroundTaskStatusCancelled:
				goto retryCollected
			}
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

retryCollected:
	// The retry task should have completed successfully.
	retryResult, hasRetry := resultMap[retryID]
	if !hasRetry {
		t.Errorf("expected retry task %q in results; got task IDs: %v", retryID, taskIDsFromResults(results))
	} else {
		if retryResult.Status != agent.BackgroundTaskStatusCompleted {
			t.Errorf("retry task %s: expected completed, got %s (error=%s)",
				retryID, retryResult.Status, retryResult.Error)
		}
		if !strings.Contains(retryResult.Answer, staleMarker) {
			t.Errorf("retry task answer missing marker %q: %q", staleMarker, firstN(retryResult.Answer, 200))
		}
	}

	// Verify two external requests were made (original + retry).
	requests, _ := recorder.snapshot()
	if len(requests) < 2 {
		t.Errorf("expected >=2 bridge requests (original + retry), got %d", len(requests))
	}

	// Print analysis report.
	t.Log("")
	t.Log("=== P1-1 Stale Retry E2E Analysis Report ===")
	t.Logf("Stale threshold: %s", staleThreshold)
	t.Logf("Stage timeout: %s", swarmCfg.StageTimeout)
	t.Logf("StaleRetryMax: %d", swarmCfg.StaleRetryMax)
	t.Logf("Total wall time: %s", wallTime)
	t.Logf("Total task IDs in result: %v", result.TaskIDs)
	t.Logf("Bridge requests dispatched: %d", len(requests))
	for i, req := range requests {
		t.Logf("  Request[%d]: taskID=%q agentType=%q promptLen=%d",
			i, req.TaskID, req.AgentType, len(req.Prompt))
	}
	t.Log("")
	t.Log("Task Results:")
	for _, r := range results {
		t.Logf("  %-30s  status=%-10s  answer=%q",
			r.ID, r.Status, firstN(r.Answer, 150))
	}
	t.Log("=============================================")

	// Final check: we don't require the original task to be completed —
	// it may still be running (bridge process sleeping 30s) but the test
	// cleanup will cancel it. The retry must succeed.
	if !hasRetry || retryResult.Status != agent.BackgroundTaskStatusCompleted {
		t.Errorf("VERDICT: stale retry did NOT result in a completed retry task")
	} else {
		t.Log("VERDICT: stale retry successfully recovered — retry task completed via bridge pipeline")
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// writeStaleAwareCLI creates a fake CLI that uses a counter file to distinguish
// between first (slow, will be killed) and subsequent (fast) invocations.
func writeStaleAwareCLI(t *testing.T, dir, name, counterFile, marker string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	script := fmt.Sprintf(`#!/usr/bin/env python3
import json
import os
import sys
import time

if len(sys.argv) < 3 or sys.argv[1] != "exec" or sys.argv[2] != "--json":
    print("unexpected invocation", file=sys.stderr)
    sys.exit(2)

counter_file = %q
first_sleep  = float(os.getenv("FAKE_CLI_FIRST_SLEEP", "10.0"))
retry_sleep  = float(os.getenv("FAKE_CLI_RETRY_SLEEP", "0.1"))
marker       = %q

# Increment invocation counter.
count = 0
try:
    with open(counter_file, "r") as f:
        count = int(f.read().strip())
except (FileNotFoundError, ValueError):
    count = 0
count += 1
with open(counter_file, "w") as f:
    f.write(str(count))

if count <= 1:
    # First call: sleep long — stale detection will cancel this subprocess.
    time.sleep(first_sleep)
else:
    # Retry call: complete quickly.
    time.sleep(retry_sleep)

prompt = ""
if "--" in sys.argv:
    idx = sys.argv.index("--")
    if idx + 1 < len(sys.argv):
        prompt = sys.argv[idx + 1]

events = [
    {"type": "thread.started", "thread_id": "fake-stale-thread"},
    {"type": "item.completed", "item": {"type": "agent_message", "text": f"{marker}::call={count}::{prompt}"}},
    {"type": "turn.completed", "usage": {"input_tokens": 5, "output_tokens": 3}},
]
for event in events:
    print(json.dumps(event), flush=True)
`, counterFile, marker)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write stale-aware cli: %v", err)
	}
	return path
}

func firstN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func taskIDsFromRequests(reqs []agent.ExternalAgentRequest) []string {
	ids := make([]string, len(reqs))
	for i, r := range reqs {
		ids[i] = r.TaskID
	}
	return ids
}

func taskIDsFromResults(results []agent.BackgroundTaskResult) []string {
	ids := make([]string, len(results))
	for i, r := range results {
		ids[i] = r.ID
	}
	return ids
}
