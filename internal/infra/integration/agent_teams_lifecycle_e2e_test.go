package integration

import (
	"context"
	"fmt"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/react"
	"alex/internal/infra/external/bridge"
	"alex/internal/infra/process"
	"alex/internal/infra/tools/builtin/orchestration"
)

// ===========================================================================
// Teams Agent Lifecycle E2E — Injection Tests
//
// These tests exercise the full lifecycle of multi-agent teams using injected
// fake CLIs (no real API calls). They cover:
//
//   1. Deep dependency chains   (4 stages)
//   2. Context inheritance       (cross-stage result propagation)
//   3. Mixed-type routing        (kimi + codex + claude_code + internal)
//   4. Process creation/destroy  (bridge → subprocess → done)
//   5. Shutdown cleanup          (mgr.Shutdown cancels in-flight tasks)
//   6. Timing & parallelism      (within-stage concurrency, cross-stage serialization)
//
// Test naming: TestTeamsLifecycle_<Aspect>
// ===========================================================================

// ---------------------------------------------------------------------------
// Test 1: 4-Stage Deep Chain with Full Context Propagation
//
// Stages:
//   0 [parallel]   → scout_kimi (kimi) + scout_codex (codex)
//   1 [sequential] → analyst (internal, inherits stage-0)
//   2 [parallel]   → writer_cc (claude_code, inherits) + auditor_kimi (kimi, inherits)
//   3 [sequential] → finalizer (internal, inherits stage-2)
//
// Verifies:
//   - 4-stage dependency chain executes in order
//   - InheritContext injects upstream results at each stage
//   - Internal agents see [Collaboration Context] with correct markers
//   - External agents see synthesis markers in prompts
//   - Status file written with correct plan ID
// ---------------------------------------------------------------------------

func TestTeamsLifecycle_DeepChain4Stage(t *testing.T) {
	env := setupBridgeEnv(t)

	workspace := t.TempDir()
	t.Chdir(workspace)

	fakeKimi := writeFakeKimiCLI(t, workspace)
	fakeCodex := writeFakeCodexCLI(t, workspace)
	fakeCC := writeFakeClaudeCodeCLI(t, workspace)

	kimiBridge := newFakeBridge(t, env, "kimi", fakeKimi, map[string]string{
		"FAKE_KIMI_MARKER":        "SCOUT_KIMI_OK",
		"FAKE_KIMI_SLEEP_SECONDS": "2",
	})
	codexBridge := newFakeBridge(t, env, "codex", fakeCodex, map[string]string{
		"FAKE_CODEX_MARKER":        "SCOUT_CODEX_OK",
		"FAKE_CODEX_SLEEP_SECONDS": "0.2",
	})
	ccBridge := newFakeBridge(t, env, "claude_code", fakeCC, map[string]string{
		"FAKE_CC_MARKER":        "WRITER_CC_OK",
		"FAKE_CC_SLEEP_SECONDS": "0.2",
	})

	mux := &multiplexExternalExecutor{
		byType: map[string]agent.ExternalAgentExecutor{
			"kimi":        kimiBridge,
			"codex":       codexBridge,
			"claude_code": ccBridge,
		},
	}
	recorder := newRecordingExternalExecutor(mux)
	capture := &internalCapture{}

	const analystMarker = "ANALYST_SYNTH_OK"
	const finalizerMarker = "FINALIZER_OK"
	internalCallCount := atomic.Int32{}

	mgr := react.NewBackgroundTaskManager(react.BackgroundManagerConfig{
		RunContext: context.Background(),
		Logger:     agent.NoopLogger{},
		Clock:      agent.SystemClock{},
		ExecuteTask: func(ctx context.Context, prompt, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
			capture.record(prompt)
			callNum := internalCallCount.Add(1)
			if callNum == 1 {
				return &agent.TaskResult{Answer: analystMarker + ":: analysis from upstream", Iterations: 1, TokensUsed: 5}, nil
			}
			return &agent.TaskResult{Answer: finalizerMarker + ":: final delivery", Iterations: 1, TokensUsed: 5}, nil
		},
		ExternalExecutor: recorder,
		SessionID:        "lifecycle-deep-chain",
	})
	defer mgr.Shutdown()

	team := agent.TeamDefinition{
		Name:        "deep_chain_4stage",
		Description: "4-stage deep dependency chain lifecycle test",
		Roles: []agent.TeamRoleDefinition{
			{Name: "scout_kimi", AgentType: "kimi", PromptTemplate: "Scout trends: {GOAL}", ExecutionMode: "plan", AutonomyLevel: "full", Config: map[string]string{"approval_policy": "never", "sandbox": "read-only"}},
			{Name: "scout_codex", AgentType: "codex", PromptTemplate: "Scan codebase: {GOAL}", ExecutionMode: "plan", AutonomyLevel: "full", Config: map[string]string{"approval_policy": "never", "sandbox": "read-only"}},
			{Name: "analyst", AgentType: "internal", PromptTemplate: "Analyze findings: {GOAL}", ExecutionMode: "execute", AutonomyLevel: "full", InheritContext: true},
			{Name: "writer_cc", AgentType: "claude_code", PromptTemplate: "Write report: {GOAL}", ExecutionMode: "plan", AutonomyLevel: "full", InheritContext: true, Config: map[string]string{"approval_policy": "never", "sandbox": "read-only"}},
			{Name: "auditor_kimi", AgentType: "kimi", PromptTemplate: "Audit report: {GOAL}", ExecutionMode: "plan", AutonomyLevel: "full", InheritContext: true, Config: map[string]string{"approval_policy": "never", "sandbox": "read-only"}},
			{Name: "finalizer", AgentType: "internal", PromptTemplate: "Finalize delivery: {GOAL}", ExecutionMode: "execute", AutonomyLevel: "full", InheritContext: true},
		},
		Stages: []agent.TeamStageDefinition{
			{Name: "scouting", Roles: []string{"scout_kimi", "scout_codex"}},
			{Name: "analysis", Roles: []string{"analyst"}},
			{Name: "reporting", Roles: []string{"writer_cc", "auditor_kimi"}},
			{Name: "finalization", Roles: []string{"finalizer"}},
		},
	}

	ctx := context.Background()
	ctx = agent.WithBackgroundDispatcher(ctx, mgr)
	ctx = agent.WithTeamDefinitions(ctx, []agent.TeamDefinition{team})

	goal := "evaluate observability patterns for distributed AI agent systems"
	res, err := orchestration.NewRunTasks().Execute(ctx, ports.ToolCall{
		ID: "call-deep-chain-4stage",
		Arguments: map[string]any{
			"template":        team.Name,
			"goal":            goal,
			"wait":            true,
			"timeout_seconds": 90,
		},
	})
	if err != nil {
		t.Fatalf("run_tasks execute: %v", err)
	}
	if res.Error != nil {
		t.Fatalf("run_tasks tool error: %v, content=%s", res.Error, res.Content)
	}

	// --- Collect all results ---
	allIDs := []string{
		"team-scout_kimi", "team-scout_codex",
		"team-analyst",
		"team-writer_cc", "team-auditor_kimi",
		"team-finalizer",
	}
	results := mgr.Collect(allIDs, false, 0)
	if len(results) != 6 {
		t.Fatalf("expected 6 results, got %d", len(results))
	}
	resultMap := make(map[string]agent.BackgroundTaskResult)
	for _, r := range results {
		resultMap[r.ID] = r
	}

	// --- Assert: all tasks completed ---
	for _, r := range results {
		if r.Status != agent.BackgroundTaskStatusCompleted {
			t.Errorf("task %s: expected completed, got %s (error=%s)", r.ID, r.Status, r.Error)
		}
	}

	// --- Assert: completion message ---
	if !strings.Contains(res.Content, "6 个任务已完成") {
		t.Errorf("unexpected completion: %q", res.Content)
	}

	// --- Assert: external request count (4 external: 2 scout + 2 reporting) ---
	requests, maxActive := recorder.snapshot()
	if len(requests) != 4 {
		t.Fatalf("expected 4 external requests, got %d", len(requests))
	}

	// --- Assert: stage-0 parallelism ---
	if maxActive < 2 {
		t.Errorf("stage-0 parallelism: expected >=2, got %d", maxActive)
	}

	// --- Assert: internal prompts ---
	prompts := capture.snapshot()
	if len(prompts) != 2 {
		t.Fatalf("expected 2 internal calls, got %d", len(prompts))
	}

	// Stage-1 analyst should see stage-0 scout results
	analystPrompt := prompts[0]
	if !strings.Contains(analystPrompt, "[Collaboration Context]") {
		t.Error("analyst prompt missing [Collaboration Context]")
	}
	if !strings.Contains(analystPrompt, "SCOUT_KIMI_OK") {
		t.Error("analyst prompt missing SCOUT_KIMI_OK marker")
	}
	if !strings.Contains(analystPrompt, "SCOUT_CODEX_OK") {
		t.Error("analyst prompt missing SCOUT_CODEX_OK marker")
	}

	// Stage-3 finalizer should see stage-2 results
	finalizerPrompt := prompts[1]
	if !strings.Contains(finalizerPrompt, "[Collaboration Context]") {
		t.Error("finalizer prompt missing [Collaboration Context]")
	}

	// --- Assert: stage-2 external agents received analyst marker ---
	for _, req := range requests {
		if req.TaskID == "team-writer_cc" || req.TaskID == "team-auditor_kimi" {
			if !strings.Contains(req.Prompt, analystMarker) {
				t.Errorf("stage-2 task %s prompt missing analyst marker", req.TaskID)
			}
		}
	}

	// --- Assert: status file ---
	statusPath := filepath.Join(workspace, ".elephant", "tasks", "team-"+team.Name+".status.yaml")
	statusData, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatalf("read status file: %v", err)
	}
	if !strings.Contains(string(statusData), "team-"+team.Name) {
		t.Errorf("status file missing plan id")
	}

	t.Logf("4-stage deep chain: 6/6 completed, maxActive=%d, 2 internal calls, context flows verified", maxActive)
}

// ---------------------------------------------------------------------------
// Test 2: Process Lifecycle Observation
//
// Verifies the subprocess creation → stdin config → stdout JSONL → wait → cleanup
// lifecycle by sharing a single process.Controller across all bridges and checking
// that all handles are deregistered after completion.
// ---------------------------------------------------------------------------

func TestTeamsLifecycle_ProcessCreationAndCleanup(t *testing.T) {
	env := setupBridgeEnv(t)

	workspace := t.TempDir()
	t.Chdir(workspace)

	fakeKimi := writeFakeKimiCLI(t, workspace)

	ctrl := process.NewController()
	defer func() { _ = ctrl.Shutdown(5 * time.Second) }()

	kimiBridge := bridge.New(bridge.BridgeConfig{
		AgentType:          "kimi",
		Binary:             fakeKimi,
		PythonBinary:       env.pythonBin,
		BridgeScript:       env.bridgeScript,
		ApprovalPolicy:     "never",
		Sandbox:            "read-only",
		PlanApprovalPolicy: "never",
		PlanSandbox:        "read-only",
		Timeout:            30 * time.Second,
		Env: map[string]string{
			"FAKE_KIMI_MARKER":        "PROC_LIFECYCLE_OK",
			"FAKE_KIMI_SLEEP_SECONDS": "2",
		},
	}, ctrl)

	recorder := newRecordingExternalExecutor(kimiBridge)

	mgr := react.NewBackgroundTaskManager(react.BackgroundManagerConfig{
		RunContext: context.Background(),
		Logger:     agent.NoopLogger{},
		Clock:      agent.SystemClock{},
		ExecuteTask: func(ctx context.Context, prompt, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
			return &agent.TaskResult{Answer: "internal-unused", Iterations: 1, TokensUsed: 1}, nil
		},
		ExternalExecutor: recorder,
		SessionID:        "lifecycle-process-cleanup",
	})
	defer mgr.Shutdown()

	teamName := "proc_lifecycle"
	team := agent.TeamDefinition{
		Name:        teamName,
		Description: "process lifecycle observation",
		Roles: []agent.TeamRoleDefinition{
			{Name: "w1", AgentType: "kimi", PromptTemplate: "Worker 1: {GOAL}", ExecutionMode: "plan", AutonomyLevel: "full", Config: map[string]string{"approval_policy": "never", "sandbox": "read-only"}},
			{Name: "w2", AgentType: "kimi", PromptTemplate: "Worker 2: {GOAL}", ExecutionMode: "plan", AutonomyLevel: "full", Config: map[string]string{"approval_policy": "never", "sandbox": "read-only"}},
			{Name: "w3", AgentType: "kimi", PromptTemplate: "Worker 3: {GOAL}", ExecutionMode: "plan", AutonomyLevel: "full", Config: map[string]string{"approval_policy": "never", "sandbox": "read-only"}},
		},
		Stages: []agent.TeamStageDefinition{
			{Name: "work", Roles: []string{"w1", "w2", "w3"}},
		},
	}

	ctx := context.Background()
	ctx = agent.WithBackgroundDispatcher(ctx, mgr)
	ctx = agent.WithTeamDefinitions(ctx, []agent.TeamDefinition{team})

	res, err := orchestration.NewRunTasks().Execute(ctx, ports.ToolCall{
		ID: "call-proc-lifecycle",
		Arguments: map[string]any{
			"template":        teamName,
			"goal":            "process lifecycle test",
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

	// Verify all completed
	allIDs := []string{"team-w1", "team-w2", "team-w3"}
	results := mgr.Collect(allIDs, false, 0)
	for _, r := range results {
		if r.Status != agent.BackgroundTaskStatusCompleted {
			t.Errorf("task %s: expected completed, got %s (error=%s)", r.ID, r.Status, r.Error)
		}
		if !strings.Contains(r.Answer, "PROC_LIFECYCLE_OK") {
			t.Errorf("task %s: answer missing marker: %q", r.ID, r.Answer)
		}
	}

	// Verify process creation/cleanup
	requests, maxActive := recorder.snapshot()
	if len(requests) != 3 {
		t.Fatalf("expected 3 bridge processes, got %d", len(requests))
	}
	if maxActive < 2 {
		t.Errorf("expected parallelism >=2, got %d", maxActive)
	}

	// After completion, controller should have no active bridge processes
	activeProcs := ctrl.List()
	activeBridge := 0
	for _, p := range activeProcs {
		if strings.Contains(p.Name, "bridge") {
			activeBridge++
		}
	}
	if activeBridge > 0 {
		t.Errorf("expected 0 active bridge processes after completion, got %d", activeBridge)
	}

	t.Logf("Process lifecycle: %d spawned, maxActive=%d, %d active after cleanup", len(requests), maxActive, activeBridge)
}

// ---------------------------------------------------------------------------
// Test 3: Shutdown Cancellation
//
// Starts a team with slow workers, calls mgr.Shutdown() mid-flight.
// Verifies shutdown is fast (cancels in-flight) and tasks are not completed.
// ---------------------------------------------------------------------------

func TestTeamsLifecycle_ShutdownCancellation(t *testing.T) {
	env := setupBridgeEnv(t)

	workspace := t.TempDir()
	t.Chdir(workspace)

	slowKimi := writeFakeSlowCLI(t, workspace, "slow-kimi", 10.0)

	kimiBridge := bridge.New(bridge.BridgeConfig{
		AgentType:          "kimi",
		Binary:             slowKimi,
		PythonBinary:       env.pythonBin,
		BridgeScript:       env.bridgeScript,
		ApprovalPolicy:     "never",
		Sandbox:            "read-only",
		PlanApprovalPolicy: "never",
		PlanSandbox:        "read-only",
		Timeout:            30 * time.Second,
		Env: map[string]string{
			"FAKE_KIMI_MARKER":        "SHOULD_NOT_COMPLETE",
			"FAKE_KIMI_SLEEP_SECONDS": "10",
		},
	}, process.NewController())

	mgr := react.NewBackgroundTaskManager(react.BackgroundManagerConfig{
		RunContext: context.Background(),
		Logger:     agent.NoopLogger{},
		Clock:      agent.SystemClock{},
		ExecuteTask: func(ctx context.Context, prompt, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
			return &agent.TaskResult{Answer: "unused", Iterations: 1, TokensUsed: 1}, nil
		},
		ExternalExecutor: kimiBridge,
		SessionID:        "lifecycle-shutdown",
	})

	teamName := "shutdown_test"
	team := agent.TeamDefinition{
		Name:        teamName,
		Description: "shutdown cancellation test",
		Roles: []agent.TeamRoleDefinition{
			{Name: "slow_a", AgentType: "kimi", PromptTemplate: "Slow A: {GOAL}", ExecutionMode: "plan", AutonomyLevel: "full", Config: map[string]string{"approval_policy": "never", "sandbox": "read-only"}},
			{Name: "slow_b", AgentType: "kimi", PromptTemplate: "Slow B: {GOAL}", ExecutionMode: "plan", AutonomyLevel: "full", Config: map[string]string{"approval_policy": "never", "sandbox": "read-only"}},
		},
		Stages: []agent.TeamStageDefinition{
			{Name: "slow_work", Roles: []string{"slow_a", "slow_b"}},
		},
	}

	ctx := context.Background()
	ctx = agent.WithBackgroundDispatcher(ctx, mgr)
	ctx = agent.WithTeamDefinitions(ctx, []agent.TeamDefinition{team})

	// Dispatch without waiting
	tool := orchestration.NewRunTasks()
	go func() {
		_, _ = tool.Execute(ctx, ports.ToolCall{
			ID: "call-shutdown-test",
			Arguments: map[string]any{
				"template":        teamName,
				"goal":            "shutdown cancellation test",
				"wait":            false,
				"timeout_seconds": 60,
			},
		})
	}()

	// Wait for tasks to start
	time.Sleep(1 * time.Second)

	// Shutdown mid-flight
	shutdownStart := time.Now()
	mgr.Shutdown()
	shutdownDuration := time.Since(shutdownStart)

	// Shutdown should be fast — not waiting for 10s sleep
	if shutdownDuration > 5*time.Second {
		t.Errorf("shutdown took %s, expected <5s", shutdownDuration)
	}

	// Tasks should be cancelled or failed, not completed
	allIDs := []string{"team-slow_a", "team-slow_b"}
	results := mgr.Collect(allIDs, false, 0)

	completedCount := 0
	for _, r := range results {
		if r.Status == agent.BackgroundTaskStatusCompleted {
			completedCount++
		}
	}

	if completedCount == 2 {
		t.Error("both tasks completed despite shutdown — cancellation did not work")
	}

	t.Logf("Shutdown: %s, %d tasks collected, %d completed (expected <2)", shutdownDuration, len(results), completedCount)
}

// ---------------------------------------------------------------------------
// Test 4: Stage Timing — Parallelism & Serialization
//
// Stage-0: 2 parallel workers (0.5s each via fake CLI)
// Stage-1: 1 internal dependent (0.3s)
//
// Verifies concurrent execution within stage and serial ordering across stages.
// ---------------------------------------------------------------------------

func TestTeamsLifecycle_StageTiming(t *testing.T) {
	env := setupBridgeEnv(t)

	workspace := t.TempDir()
	t.Chdir(workspace)

	fakeKimi := writeFakeKimiCLI(t, workspace)

	kimiBridge := newFakeBridge(t, env, "kimi", fakeKimi, map[string]string{
		"FAKE_KIMI_MARKER":        "TIMING_OK",
		"FAKE_KIMI_SLEEP_SECONDS": "2",
	})
	recorder := newRecordingExternalExecutor(kimiBridge)

	capture := &internalCapture{}
	mgr := react.NewBackgroundTaskManager(react.BackgroundManagerConfig{
		RunContext: context.Background(),
		Logger:     agent.NoopLogger{},
		Clock:      agent.SystemClock{},
		ExecuteTask: func(ctx context.Context, prompt, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
			capture.record(prompt)
			time.Sleep(300 * time.Millisecond)
			return &agent.TaskResult{Answer: "TIMING_INTERNAL_OK:: processed", Iterations: 1, TokensUsed: 1}, nil
		},
		ExternalExecutor: recorder,
		SessionID:        "lifecycle-timing",
	})
	defer mgr.Shutdown()

	team := agent.TeamDefinition{
		Name:        "timing_test",
		Description: "stage timing verification",
		Roles: []agent.TeamRoleDefinition{
			{Name: "fast_a", AgentType: "kimi", PromptTemplate: "Fast A: {GOAL}", ExecutionMode: "plan", AutonomyLevel: "full", Config: map[string]string{"approval_policy": "never", "sandbox": "read-only"}},
			{Name: "fast_b", AgentType: "kimi", PromptTemplate: "Fast B: {GOAL}", ExecutionMode: "plan", AutonomyLevel: "full", Config: map[string]string{"approval_policy": "never", "sandbox": "read-only"}},
			{Name: "dependent", AgentType: "internal", PromptTemplate: "Process: {GOAL}", ExecutionMode: "execute", AutonomyLevel: "full", InheritContext: true},
		},
		Stages: []agent.TeamStageDefinition{
			{Name: "parallel_work", Roles: []string{"fast_a", "fast_b"}},
			{Name: "serial_process", Roles: []string{"dependent"}},
		},
	}

	ctx := context.Background()
	ctx = agent.WithBackgroundDispatcher(ctx, mgr)
	ctx = agent.WithTeamDefinitions(ctx, []agent.TeamDefinition{team})

	start := time.Now()
	res, err := orchestration.NewRunTasks().Execute(ctx, ports.ToolCall{
		ID: "call-timing-test",
		Arguments: map[string]any{
			"template":        team.Name,
			"goal":            "timing test",
			"wait":            true,
			"timeout_seconds": 30,
		},
	})
	wallClock := time.Since(start)

	if err != nil {
		t.Fatalf("run_tasks execute: %v", err)
	}
	if res.Error != nil {
		t.Fatalf("run_tasks tool error: %v, content=%s", res.Error, res.Content)
	}

	allIDs := []string{"team-fast_a", "team-fast_b", "team-dependent"}
	results := mgr.Collect(allIDs, false, 0)
	for _, r := range results {
		if r.Status != agent.BackgroundTaskStatusCompleted {
			t.Errorf("task %s: expected completed, got %s", r.ID, r.Status)
		}
	}

	_, maxActive := recorder.snapshot()

	if maxActive < 2 {
		t.Errorf("stage-0 parallelism: expected >=2, got %d", maxActive)
	}

	// Wall clock upper bound: parallel should be well under 8s
	if wallClock > 8*time.Second {
		t.Errorf("wall clock %s too slow", wallClock)
	}

	// Internal agent should have received context from stage-0
	prompts := capture.snapshot()
	if len(prompts) != 1 {
		t.Fatalf("expected 1 internal call, got %d", len(prompts))
	}
	if !strings.Contains(prompts[0], "TIMING_OK") {
		t.Error("dependent prompt missing TIMING_OK marker from stage-0")
	}

	t.Logf("Timing: wall=%s, maxActive=%d, internal_calls=%d", wallClock, maxActive, len(prompts))
}

// ---------------------------------------------------------------------------
// Test 5: Partial Failure — One Fails, Dependent Blocked
//
// Stage-0: ok_worker (success) + fail_worker (fails)
// Stage-1: dependent (internal, inherits)
//
// Verifies failure propagation without deadlock.
// ---------------------------------------------------------------------------

func TestTeamsLifecycle_PartialFailure(t *testing.T) {
	env := setupBridgeEnv(t)

	workspace := t.TempDir()
	t.Chdir(workspace)

	fakeKimi := writeFakeKimiCLI(t, workspace)
	failingCLI := writeFakeFailingCLI(t, workspace)

	kimiBridge := newFakeBridge(t, env, "kimi", fakeKimi, map[string]string{
		"FAKE_KIMI_MARKER":        "PARTIAL_OK",
		"FAKE_KIMI_SLEEP_SECONDS": "2",
	})
	failBridge := newFakeBridge(t, env, "codex", failingCLI, nil)

	mux := &multiplexExternalExecutor{
		byType: map[string]agent.ExternalAgentExecutor{
			"kimi":  kimiBridge,
			"codex": failBridge,
		},
	}

	internalCalled := atomic.Bool{}
	mgr := react.NewBackgroundTaskManager(react.BackgroundManagerConfig{
		RunContext: context.Background(),
		Logger:     agent.NoopLogger{},
		Clock:      agent.SystemClock{},
		ExecuteTask: func(ctx context.Context, prompt, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
			internalCalled.Store(true)
			return &agent.TaskResult{Answer: "should-not-be-called", Iterations: 1, TokensUsed: 1}, nil
		},
		ExternalExecutor: mux,
		SessionID:        "lifecycle-partial-failure",
	})
	defer mgr.Shutdown()

	team := agent.TeamDefinition{
		Name:        "partial_failure",
		Description: "mixed success/failure lifecycle",
		Roles: []agent.TeamRoleDefinition{
			{Name: "ok_worker", AgentType: "kimi", PromptTemplate: "Success: {GOAL}", ExecutionMode: "plan", AutonomyLevel: "full", Config: map[string]string{"approval_policy": "never", "sandbox": "read-only"}},
			{Name: "fail_worker", AgentType: "codex", PromptTemplate: "Fail: {GOAL}", ExecutionMode: "plan", AutonomyLevel: "full", Config: map[string]string{"approval_policy": "never", "sandbox": "read-only"}},
			{Name: "dependent_inheritor", AgentType: "internal", PromptTemplate: "Process: {GOAL}", ExecutionMode: "execute", AutonomyLevel: "full", InheritContext: true},
		},
		Stages: []agent.TeamStageDefinition{
			{Name: "work", Roles: []string{"ok_worker", "fail_worker"}},
			{Name: "process", Roles: []string{"dependent_inheritor"}},
		},
	}

	ctx := context.Background()
	ctx = agent.WithBackgroundDispatcher(ctx, mgr)
	ctx = agent.WithTeamDefinitions(ctx, []agent.TeamDefinition{team})

	res, err := orchestration.NewRunTasks().Execute(ctx, ports.ToolCall{
		ID: "call-partial-failure",
		Arguments: map[string]any{
			"template":        team.Name,
			"goal":            "test partial failure",
			"wait":            true,
			"timeout_seconds": 30,
		},
	})
	if err != nil {
		t.Fatalf("run_tasks execute: %v", err)
	}
	_ = res

	allIDs := []string{"team-ok_worker", "team-fail_worker", "team-dependent_inheritor"}
	results := mgr.Collect(allIDs, true, 15*time.Second)

	resultMap := make(map[string]agent.BackgroundTaskResult)
	for _, r := range results {
		resultMap[r.ID] = r
	}

	if ok := resultMap["team-ok_worker"]; ok.Status != agent.BackgroundTaskStatusCompleted {
		t.Errorf("ok_worker: expected completed, got %s (error=%s)", ok.Status, ok.Error)
	} else if !strings.Contains(ok.Answer, "PARTIAL_OK") {
		t.Errorf("ok_worker: answer missing marker: %q", ok.Answer)
	}

	if fail := resultMap["team-fail_worker"]; fail.Status != agent.BackgroundTaskStatusFailed {
		t.Errorf("fail_worker: expected failed, got %s", fail.Status)
	}

	if dep := resultMap["team-dependent_inheritor"]; dep.Status != agent.BackgroundTaskStatusFailed {
		t.Errorf("dependent: expected failed, got %s (error=%s)", dep.Status, dep.Error)
	} else if !strings.Contains(strings.ToLower(dep.Error), "dependency") && !strings.Contains(strings.ToLower(dep.Error), "failed") {
		t.Errorf("dependent: error should mention dependency: %q", dep.Error)
	}

	if internalCalled.Load() {
		t.Error("internal executeTask called despite dependency failure")
	}

	t.Logf("Partial failure: ok=completed, fail=failed, dependent=failed(propagated), internal_called=%v", internalCalled.Load())
}

// ---------------------------------------------------------------------------
// Test 6: Config Dispatch Correctness
//
// Two kimi workers with different configs. Verifies per-role Config fields
// are faithfully delivered to the bridge executor.
// ---------------------------------------------------------------------------

func TestTeamsLifecycle_ConfigDispatch(t *testing.T) {
	env := setupBridgeEnv(t)

	workspace := t.TempDir()
	t.Chdir(workspace)

	fakeKimi := writeFakeKimiCLI(t, workspace)

	kimiBridge := newFakeBridge(t, env, "kimi", fakeKimi, map[string]string{
		"FAKE_KIMI_MARKER":        "CONFIG_OK",
		"FAKE_KIMI_SLEEP_SECONDS": "2",
	})
	recorder := newRecordingExternalExecutor(kimiBridge)

	mgr := react.NewBackgroundTaskManager(react.BackgroundManagerConfig{
		RunContext: context.Background(),
		Logger:     agent.NoopLogger{},
		Clock:      agent.SystemClock{},
		ExecuteTask: func(ctx context.Context, prompt, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
			return &agent.TaskResult{Answer: "internal", Iterations: 1, TokensUsed: 1}, nil
		},
		ExternalExecutor: recorder,
		SessionID:        "lifecycle-config-dispatch",
	})
	defer mgr.Shutdown()

	team := agent.TeamDefinition{
		Name:        "config_dispatch",
		Description: "config faithfulness test",
		Roles: []agent.TeamRoleDefinition{
			{
				Name: "strict_worker", AgentType: "kimi", PromptTemplate: "Strict: {GOAL}",
				ExecutionMode: "plan", AutonomyLevel: "full",
				Config: map[string]string{"approval_policy": "never", "sandbox": "read-only"},
			},
			{
				Name: "loose_worker", AgentType: "kimi", PromptTemplate: "Loose: {GOAL}",
				ExecutionMode: "plan", AutonomyLevel: "full",
				Config: map[string]string{"approval_policy": "on-failure", "sandbox": "network-only"},
			},
		},
		Stages: []agent.TeamStageDefinition{
			{Name: "dispatch", Roles: []string{"strict_worker", "loose_worker"}},
		},
	}

	ctx := context.Background()
	ctx = agent.WithBackgroundDispatcher(ctx, mgr)
	ctx = agent.WithTeamDefinitions(ctx, []agent.TeamDefinition{team})

	res, err := orchestration.NewRunTasks().Execute(ctx, ports.ToolCall{
		ID: "call-config-dispatch",
		Arguments: map[string]any{
			"template":        team.Name,
			"goal":            "config dispatch test",
			"wait":            true,
			"timeout_seconds": 30,
		},
	})
	if err != nil {
		t.Fatalf("run_tasks execute: %v", err)
	}
	if res.Error != nil {
		t.Fatalf("run_tasks tool error: %v, content=%s", res.Error, res.Content)
	}

	requests, _ := recorder.snapshot()
	if len(requests) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(requests))
	}

	reqMap := make(map[string]agent.ExternalAgentRequest)
	for _, req := range requests {
		reqMap[req.TaskID] = req
	}

	// strict_worker
	strict := reqMap["team-strict_worker"]
	if strings.TrimSpace(strict.Config["approval_policy"]) != "never" {
		t.Errorf("strict approval_policy: got %q, want never", strict.Config["approval_policy"])
	}
	if strings.TrimSpace(strict.Config["sandbox"]) != "read-only" {
		t.Errorf("strict sandbox: got %q, want read-only", strict.Config["sandbox"])
	}
	if strict.ExecutionMode != "plan" {
		t.Errorf("strict execution_mode: got %q, want plan", strict.ExecutionMode)
	}

	// loose_worker
	loose := reqMap["team-loose_worker"]
	if strings.TrimSpace(loose.Config["approval_policy"]) != "on-failure" {
		t.Errorf("loose approval_policy: got %q, want on-failure", loose.Config["approval_policy"])
	}
	if strings.TrimSpace(loose.Config["sandbox"]) != "network-only" {
		t.Errorf("loose sandbox: got %q, want network-only", loose.Config["sandbox"])
	}

	t.Log("Config dispatch: per-role configs correctly routed")
}

// ===========================================================================
// Shared helpers
// ===========================================================================

type bridgeEnv struct {
	pythonBin    string
	bridgeScript string
}

func setupBridgeEnv(t *testing.T) bridgeEnv {
	t.Helper()
	pythonBin, err := osexec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not found in PATH")
	}
	repoRoot := findRepoRoot(t)
	bridgeScript := filepath.Join(repoRoot, "scripts", "codex_bridge", "codex_bridge.py")
	if _, err := os.Stat(bridgeScript); err != nil {
		t.Skipf("codex bridge script not found at %s", bridgeScript)
	}
	return bridgeEnv{pythonBin: pythonBin, bridgeScript: bridgeScript}
}

func newFakeBridge(t *testing.T, env bridgeEnv, agentType, binary string, extraEnv map[string]string) *bridge.Executor {
	t.Helper()
	return bridge.New(bridge.BridgeConfig{
		AgentType:          agentType,
		Binary:             binary,
		PythonBinary:       env.pythonBin,
		BridgeScript:       env.bridgeScript,
		ApprovalPolicy:     "never",
		Sandbox:            "read-only",
		PlanApprovalPolicy: "never",
		PlanSandbox:        "read-only",
		Timeout:            30 * time.Second,
		Env:                extraEnv,
	}, process.NewController())
}

func writeFakeSlowCLI(t *testing.T, dir string, name string, sleepSec float64) string {
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

sleep_seconds = float(os.getenv("FAKE_KIMI_SLEEP_SECONDS", "%.1f"))
marker = os.getenv("FAKE_KIMI_MARKER", "SLOW_CLI")

time.sleep(sleep_seconds)

prompt = ""
if "--" in sys.argv:
    idx = sys.argv.index("--")
    if idx + 1 < len(sys.argv):
        prompt = sys.argv[idx + 1]

events = [
    {"type": "thread.started", "thread_id": "fake-slow-thread"},
    {"type": "item.completed", "item": {"type": "agent_message", "text": f"{marker}::{prompt}"}},
    {"type": "turn.completed", "usage": {"input_tokens": 5, "output_tokens": 3}},
]
for event in events:
    print(json.dumps(event), flush=True)
`, sleepSec)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake slow cli: %v", err)
	}
	return path
}
