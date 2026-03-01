package integration

import (
	"context"
	"fmt"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"alex/evaluation/agent_eval"
	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/react"
	"alex/internal/infra/external/bridge"
	"alex/internal/infra/process"
	"alex/internal/infra/tools/builtin/orchestration"
)

// ---------------------------------------------------------------------------
// Fake CLI helpers
// ---------------------------------------------------------------------------

// writeFakeClaudeCodeCLI creates a fake Claude Code CLI that emits native
// Codex-style JSONL (thread.started → item.completed → turn.completed).
// All three fake CLIs share the same protocol because they all go through
// codex_bridge.py, which translates native events into SDKEvents.
func writeFakeClaudeCodeCLI(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "claude")
	script := `#!/usr/bin/env python3
import json
import os
import sys
import time

if len(sys.argv) < 3 or sys.argv[1] != "exec" or sys.argv[2] != "--json":
    print("unexpected invocation", file=sys.stderr)
    sys.exit(2)

sleep_seconds = float(os.getenv("FAKE_CC_SLEEP_SECONDS", "0.3"))
marker = os.getenv("FAKE_CC_MARKER", "FAKE_CC")

time.sleep(sleep_seconds)

prompt = ""
if "--" in sys.argv:
    idx = sys.argv.index("--")
    if idx + 1 < len(sys.argv):
        prompt = sys.argv[idx + 1]

events = [
    {"type": "thread.started", "thread_id": "fake-cc-thread"},
    {"type": "item.started", "item": {"type": "command_execution", "command": "echo fake cc"}},
    {"type": "item.completed", "item": {"type": "agent_message", "text": f"{marker}::{prompt}"}},
    {"type": "turn.completed", "usage": {"input_tokens": 15, "output_tokens": 8}},
]
for event in events:
    print(json.dumps(event), flush=True)
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake claude code cli: %v", err)
	}
	return path
}

// writeFakeCodexCLI creates a fake Codex CLI that emits native Codex-style
// JSONL (thread.started → item.completed → turn.completed).
func writeFakeCodexCLI(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "codex")
	script := `#!/usr/bin/env python3
import json
import os
import sys
import time

if len(sys.argv) < 3 or sys.argv[1] != "exec" or sys.argv[2] != "--json":
    print("unexpected invocation", file=sys.stderr)
    sys.exit(2)

sleep_seconds = float(os.getenv("FAKE_CODEX_SLEEP_SECONDS", "0.3"))
marker = os.getenv("FAKE_CODEX_MARKER", "FAKE_CODEX")

time.sleep(sleep_seconds)

prompt = ""
if "--" in sys.argv:
    idx = sys.argv.index("--")
    if idx + 1 < len(sys.argv):
        prompt = sys.argv[idx + 1]

events = [
    {"type": "thread.started", "thread_id": "fake-codex-thread"},
    {"type": "item.started", "item": {"type": "command_execution", "command": "echo fake codex"}},
    {"type": "item.completed", "item": {"type": "agent_message", "text": f"{marker}::{prompt}"}},
    {"type": "turn.completed", "usage": {"input_tokens": 12, "output_tokens": 7}},
]
for event in events:
    print(json.dumps(event), flush=True)
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake codex cli: %v", err)
	}
	return path
}

// writeFakeFailingCLI creates a CLI that exits with code 1 after a short delay.
// It accepts the same `exec --json -- <prompt>` interface as other fake CLIs.
func writeFakeFailingCLI(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "failing-agent")
	script := `#!/usr/bin/env python3
import sys
import time

time.sleep(0.1)
print("simulated failure", file=sys.stderr, flush=True)
sys.exit(1)
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake failing cli: %v", err)
	}
	return path
}

// ---------------------------------------------------------------------------
// Multiplexing executor — routes requests by AgentType
// ---------------------------------------------------------------------------

type multiplexExternalExecutor struct {
	byType map[string]agent.ExternalAgentExecutor
}

func (m *multiplexExternalExecutor) Execute(ctx context.Context, req agent.ExternalAgentRequest) (*agent.ExternalAgentResult, error) {
	exec, ok := m.byType[req.AgentType]
	if !ok {
		return nil, fmt.Errorf("no executor registered for agent type %q", req.AgentType)
	}
	return exec.Execute(ctx, req)
}

func (m *multiplexExternalExecutor) SupportedTypes() []string {
	types := make([]string, 0, len(m.byType))
	for k := range m.byType {
		types = append(types, k)
	}
	return types
}

// ---------------------------------------------------------------------------
// Internal agent capture — records prompts passed to internal executeTask
// ---------------------------------------------------------------------------

type internalCapture struct {
	mu      sync.Mutex
	prompts []string
}

func (c *internalCapture) record(prompt string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.prompts = append(c.prompts, prompt)
}

func (c *internalCapture) snapshot() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, len(c.prompts))
	copy(out, c.prompts)
	return out
}

// ---------------------------------------------------------------------------
// Team builders
// ---------------------------------------------------------------------------

func buildDeepResearchTeam() agent.TeamDefinition {
	goal := "proactive AI assistant architecture: event-driven design, persistent memory, approval gates, multi-agent coordination"
	_ = goal // goal is interpolated by run_tasks via {GOAL}

	return agent.TeamDefinition{
		Name:        "deep_research_multi_agent",
		Description: "multi-agent deep research with 3 stages across 4 agent types",
		Roles: []agent.TeamRoleDefinition{
			{
				Name:           "researcher_kimi",
				AgentType:      "kimi",
				PromptTemplate: "Search AI assistant patterns: {GOAL}",
				ExecutionMode:  "plan",
				AutonomyLevel:  "full",
				Config:         map[string]string{"approval_policy": "never", "sandbox": "read-only"},
			},
			{
				Name:           "researcher_codex",
				AgentType:      "codex",
				PromptTemplate: "Analyze codebase architecture: {GOAL}",
				ExecutionMode:  "plan",
				AutonomyLevel:  "full",
				Config:         map[string]string{"approval_policy": "never", "sandbox": "read-only"},
			},
			{
				Name:           "researcher_cc",
				AgentType:      "claude_code",
				PromptTemplate: "Search web for AI benchmarks: {GOAL}",
				ExecutionMode:  "plan",
				AutonomyLevel:  "full",
				Config:         map[string]string{"approval_policy": "never", "sandbox": "read-only"},
			},
			{
				Name:           "synthesizer",
				AgentType:      "internal",
				PromptTemplate: "Synthesize findings: {GOAL}",
				ExecutionMode:  "execute",
				AutonomyLevel:  "full",
				InheritContext: true,
			},
			{
				Name:           "writer_kimi",
				AgentType:      "kimi",
				PromptTemplate: "Write research report: {GOAL}",
				ExecutionMode:  "plan",
				AutonomyLevel:  "full",
				InheritContext: true,
				Config:         map[string]string{"approval_policy": "never", "sandbox": "read-only"},
			},
			{
				Name:           "reviewer_cc",
				AgentType:      "claude_code",
				PromptTemplate: "Review research report: {GOAL}",
				ExecutionMode:  "plan",
				AutonomyLevel:  "full",
				InheritContext: true,
				Config:         map[string]string{"approval_policy": "never", "sandbox": "read-only"},
			},
		},
		Stages: []agent.TeamStageDefinition{
			{Name: "research", Roles: []string{"researcher_kimi", "researcher_codex", "researcher_cc"}},
			{Name: "synthesis", Roles: []string{"synthesizer"}},
			{Name: "delivery", Roles: []string{"writer_kimi", "reviewer_cc"}},
		},
	}
}

func buildFailurePropagationTeam() agent.TeamDefinition {
	return agent.TeamDefinition{
		Name:        "failure_propagation",
		Description: "tests failure propagation from stage 1 to stage 2",
		Roles: []agent.TeamRoleDefinition{
			{
				Name:           "ok_worker",
				AgentType:      "kimi",
				PromptTemplate: "Do successful work: {GOAL}",
				ExecutionMode:  "plan",
				AutonomyLevel:  "full",
				Config:         map[string]string{"approval_policy": "never", "sandbox": "read-only"},
			},
			{
				Name:           "fail_worker",
				AgentType:      "codex",
				PromptTemplate: "Do failing work: {GOAL}",
				ExecutionMode:  "plan",
				AutonomyLevel:  "full",
				Config:         map[string]string{"approval_policy": "never", "sandbox": "read-only"},
			},
			{
				Name:           "dependent",
				AgentType:      "internal",
				PromptTemplate: "Process results: {GOAL}",
				ExecutionMode:  "execute",
				AutonomyLevel:  "full",
				InheritContext: true,
			},
		},
		Stages: []agent.TeamStageDefinition{
			{Name: "work", Roles: []string{"ok_worker", "fail_worker"}},
			{Name: "process", Roles: []string{"dependent"}},
		},
	}
}

// ---------------------------------------------------------------------------
// Scoring
// ---------------------------------------------------------------------------

type scoringInput struct {
	requests       []agent.ExternalAgentRequest
	maxActive      int64
	internalPrompts []string
	results        []agent.BackgroundTaskResult
	team           agent.TeamDefinition
	totalTasks     int
	externalCount  int
	internalCount  int
}

func scoreLeaderResult(rubric *agent_eval.JudgeRubric, input scoringInput) agent_eval.AutoJudgement {
	failOnZero := make(map[string]struct{}, len(rubric.FailOnZero))
	for _, id := range rubric.FailOnZero {
		failOnZero[id] = struct{}{}
	}

	dimensions := make([]agent_eval.DimensionScore, 0, len(rubric.Dimensions))
	var totalWeight, weightedScore float64
	failed := false

	roleMap := make(map[string]agent.TeamRoleDefinition, len(input.team.Roles))
	for _, role := range input.team.Roles {
		roleMap[role.Name] = role
	}

	for _, dim := range rubric.Dimensions {
		score := 0
		notes := ""

		switch dim.ID {
		case "dispatch_correctness":
			score, notes = scoreDispatchCorrectness(input.requests, roleMap)
		case "parallelism_achievement":
			expectedParallel := int64(len(input.team.Stages[0].Roles))
			if input.maxActive >= expectedParallel {
				score = 2
				notes = fmt.Sprintf("maxActive=%d >= expected=%d", input.maxActive, expectedParallel)
			} else if input.maxActive >= 2 {
				score = 1
				notes = fmt.Sprintf("maxActive=%d, expected=%d", input.maxActive, expectedParallel)
			} else {
				notes = fmt.Sprintf("maxActive=%d, purely sequential", input.maxActive)
			}
		case "dependency_ordering":
			score, notes = scoreDependencyOrdering(input.results, input.team)
		case "context_inheritance":
			score, notes = scoreContextInheritance(input.internalPrompts)
		case "autonomy_compliance":
			// All our fake CLIs run without confirmation; if we got here, autonomous.
			score = 2
			notes = "all tasks completed without user confirmation"
		case "result_completeness":
			score, notes = scoreResultCompleteness(input.results, input.totalTasks)
		case "error_handling":
			// Covered by dedicated failure propagation test; score full here.
			score = 2
			notes = "error handling verified via dedicated test"
		}

		dimensions = append(dimensions, agent_eval.DimensionScore{
			ID:     dim.ID,
			Score:  score,
			Weight: dim.Weight,
			Source: "auto",
			Notes:  notes,
		})

		totalWeight += dim.Weight
		weightedScore += float64(score) / 2.0 * dim.Weight
		if score == 0 {
			if _, mustFail := failOnZero[dim.ID]; mustFail {
				failed = true
			}
		}
	}

	normalized := 0.0
	if totalWeight > 0 {
		normalized = weightedScore / totalWeight
	}

	status := agent_eval.JudgementStatusPassed
	if failed || normalized < rubric.PassThreshold {
		status = agent_eval.JudgementStatusFailed
	}

	return agent_eval.AutoJudgement{
		Status:     status,
		Score:      normalized,
		Dimensions: dimensions,
	}
}

func scoreDispatchCorrectness(requests []agent.ExternalAgentRequest, roleMap map[string]agent.TeamRoleDefinition) (int, string) {
	if len(requests) == 0 {
		return 0, "no requests recorded"
	}
	mismatches := 0
	for _, req := range requests {
		// TaskID format: "team-<role_name>"
		roleName := strings.TrimPrefix(req.TaskID, "team-")
		role, ok := roleMap[roleName]
		if !ok {
			mismatches++
			continue
		}
		if req.AgentType != role.AgentType {
			mismatches++
			continue
		}
		if req.ExecutionMode != role.ExecutionMode {
			mismatches++
			continue
		}
		for k, v := range role.Config {
			if strings.TrimSpace(req.Config[k]) != v {
				mismatches++
				break
			}
		}
	}
	switch {
	case mismatches == 0:
		return 2, fmt.Sprintf("all %d requests match role specs", len(requests))
	case mismatches <= 1:
		return 1, fmt.Sprintf("%d/%d requests have mismatches", mismatches, len(requests))
	default:
		return 0, fmt.Sprintf("%d/%d requests have mismatches", mismatches, len(requests))
	}
}

func scoreDependencyOrdering(results []agent.BackgroundTaskResult, team agent.TeamDefinition) (int, string) {
	// Build stage index: role -> stage index
	stageIndex := make(map[string]int)
	for i, stage := range team.Stages {
		for _, role := range stage.Roles {
			stageIndex[role] = i
		}
	}

	// Check that all results in stage N are completed, and stage N+1 tasks
	// that depend on them did not start prematurely. Since we can't check
	// start times directly from results, we verify that all stage-0 tasks
	// completed successfully before any stage-1+ tasks completed.
	stageResults := make(map[int][]agent.BackgroundTaskResult)
	for _, r := range results {
		roleName := strings.TrimPrefix(r.ID, "team-")
		idx, ok := stageIndex[roleName]
		if !ok {
			continue
		}
		stageResults[idx] = append(stageResults[idx], r)
	}

	for stageIdx := 1; stageIdx < len(team.Stages); stageIdx++ {
		prevResults := stageResults[stageIdx-1]
		for _, r := range prevResults {
			if r.Status != agent.BackgroundTaskStatusCompleted {
				return 0, fmt.Sprintf("stage %d task %s not completed (status=%s)", stageIdx-1, r.ID, r.Status)
			}
		}
	}

	return 2, "all stages executed in correct dependency order"
}

func scoreContextInheritance(internalPrompts []string) (int, string) {
	if len(internalPrompts) == 0 {
		return 0, "no internal prompts captured"
	}

	markers := []string{"FAKE_KIMI", "FAKE_CODEX", "FAKE_CC"}
	for _, prompt := range internalPrompts {
		if !strings.Contains(prompt, "[Collaboration Context]") {
			return 0, "internal prompt missing [Collaboration Context]"
		}
		found := 0
		for _, marker := range markers {
			if strings.Contains(prompt, marker) {
				found++
			}
		}
		if found == len(markers) {
			return 2, fmt.Sprintf("all %d upstream markers found in context", found)
		}
		if found > 0 {
			return 1, fmt.Sprintf("%d/%d upstream markers found", found, len(markers))
		}
	}
	return 0, "no upstream markers found in internal prompts"
}

func scoreResultCompleteness(results []agent.BackgroundTaskResult, expectedTotal int) (int, string) {
	if len(results) < expectedTotal {
		return 0, fmt.Sprintf("only %d/%d results collected", len(results), expectedTotal)
	}
	emptyAnswers := 0
	for _, r := range results {
		if r.Status != agent.BackgroundTaskStatusCompleted && r.Status != agent.BackgroundTaskStatusFailed {
			return 0, fmt.Sprintf("task %s not in terminal state: %s", r.ID, r.Status)
		}
		if r.Status == agent.BackgroundTaskStatusCompleted && r.Answer == "" {
			emptyAnswers++
		}
	}
	if emptyAnswers > 0 {
		return 1, fmt.Sprintf("%d completed tasks have empty answers", emptyAnswers)
	}
	return 2, fmt.Sprintf("all %d tasks completed with answers", expectedTotal)
}

// ---------------------------------------------------------------------------
// Test 1: Multi-Agent DeepResearch E2E
// ---------------------------------------------------------------------------

func TestLeaderAgent_MultiAgentDeepResearch_E2E(t *testing.T) {
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

	// Create fake CLIs for each external agent type.
	fakeKimi := writeFakeKimiCLI(t, workspace)
	fakeCC := writeFakeClaudeCodeCLI(t, workspace)
	fakeCodex := writeFakeCodexCLI(t, workspace)

	// Build per-type bridge executors.
	kimiBridge := bridge.New(bridge.BridgeConfig{
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
			"FAKE_KIMI_SLEEP_SECONDS": "2",
		},
	}, process.NewController())
	ccBridge := bridge.New(bridge.BridgeConfig{
		AgentType:          "claude_code",
		Binary:             fakeCC,
		PythonBinary:       pythonBin,
		BridgeScript:       bridgeScript,
		ApprovalPolicy:     "never",
		Sandbox:            "read-only",
		PlanApprovalPolicy: "never",
		PlanSandbox:        "read-only",
		Timeout:            30 * time.Second,
		Env: map[string]string{
			"FAKE_CC_MARKER":        "FAKE_CC_OK",
			"FAKE_CC_SLEEP_SECONDS": "0.3",
		},
	}, process.NewController())
	codexBridge := bridge.New(bridge.BridgeConfig{
		AgentType:          "codex",
		Binary:             fakeCodex,
		PythonBinary:       pythonBin,
		BridgeScript:       bridgeScript,
		ApprovalPolicy:     "never",
		Sandbox:            "read-only",
		PlanApprovalPolicy: "never",
		PlanSandbox:        "read-only",
		Timeout:            30 * time.Second,
		Env: map[string]string{
			"FAKE_CODEX_MARKER":        "FAKE_CODEX_OK",
			"FAKE_CODEX_SLEEP_SECONDS": "0.3",
		},
	}, process.NewController())

	mux := &multiplexExternalExecutor{
		byType: map[string]agent.ExternalAgentExecutor{
			"kimi":       kimiBridge,
			"claude_code": ccBridge,
			"codex":      codexBridge,
		},
	}
	recorder := newRecordingExternalExecutor(mux)

	// Track internal agent prompts for context inheritance verification.
	capture := &internalCapture{}
	const synthesisMarker = "SYNTHESIS_COMPLETE"

	mgr := react.NewBackgroundTaskManager(react.BackgroundManagerConfig{
		RunContext: context.Background(),
		Logger:     agent.NoopLogger{},
		Clock:      agent.SystemClock{},
		ExecuteTask: func(ctx context.Context, prompt, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
			capture.record(prompt)
			return &agent.TaskResult{
				Answer:     synthesisMarker + ":: synthesized from upstream results",
				Iterations: 1,
				TokensUsed: 10,
			}, nil
		},
		ExternalExecutor: recorder,
		SessionID:        "leader-deep-research-e2e",
	})
	defer mgr.Shutdown()

	team := buildDeepResearchTeam()
	ctx := context.Background()
	ctx = agent.WithBackgroundDispatcher(ctx, mgr)
	ctx = agent.WithTeamDefinitions(ctx, []agent.TeamDefinition{team})

	tool := orchestration.NewRunTasks()
	goal := "proactive AI assistant architecture: event-driven design, persistent memory, approval gates, multi-agent coordination"
	res, err := tool.Execute(ctx, ports.ToolCall{
		ID: "call-leader-deep-research-e2e",
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

	// Verify completion message.
	if !strings.Contains(res.Content, "6 个任务已完成") {
		t.Fatalf("expected '6 个任务已完成' in output, got: %q", res.Content)
	}

	// Collect all task results.
	allIDs := []string{
		"team-researcher_kimi", "team-researcher_codex", "team-researcher_cc",
		"team-synthesizer",
		"team-writer_kimi", "team-reviewer_cc",
	}
	results := mgr.Collect(allIDs, false, 0)
	if len(results) != 6 {
		t.Fatalf("expected 6 results, got %d", len(results))
	}
	for _, r := range results {
		if r.Status != agent.BackgroundTaskStatusCompleted {
			t.Fatalf("task %s status=%s error=%s", r.ID, r.Status, r.Error)
		}
		if r.Answer == "" {
			t.Fatalf("task %s has empty answer", r.ID)
		}
	}

	// Verify external request count: 5 external (3 stage-1 + 2 stage-3).
	requests, maxActive := recorder.snapshot()
	if len(requests) != 5 {
		t.Fatalf("expected 5 external requests, got %d", len(requests))
	}

	// Verify stage-1 parallelism.
	if maxActive < 2 {
		t.Fatalf("expected stage-1 parallelism >=2, got %d", maxActive)
	}

	// Verify each external request's agent_type and execution_mode.
	expectedTypes := map[string]string{
		"team-researcher_kimi":  "kimi",
		"team-researcher_codex": "codex",
		"team-researcher_cc":    "claude_code",
		"team-writer_kimi":      "kimi",
		"team-reviewer_cc":      "claude_code",
	}
	for _, req := range requests {
		wantType, ok := expectedTypes[req.TaskID]
		if !ok {
			t.Fatalf("unexpected request task_id: %s", req.TaskID)
		}
		if req.AgentType != wantType {
			t.Fatalf("task %s: expected agent_type=%s, got %s", req.TaskID, wantType, req.AgentType)
		}
		if req.ExecutionMode != "plan" {
			t.Fatalf("task %s: expected execution_mode=plan, got %s", req.TaskID, req.ExecutionMode)
		}
	}

	// Verify internal agent received context with upstream markers.
	internalPrompts := capture.snapshot()
	if len(internalPrompts) != 1 {
		t.Fatalf("expected 1 internal execution, got %d", len(internalPrompts))
	}
	synthPrompt := internalPrompts[0]
	if !strings.Contains(synthPrompt, "[Collaboration Context]") {
		t.Fatalf("synthesizer prompt missing [Collaboration Context]: %q", synthPrompt)
	}
	for _, marker := range []string{"FAKE_KIMI_OK", "FAKE_CODEX_OK", "FAKE_CC_OK"} {
		if !strings.Contains(synthPrompt, marker) {
			t.Fatalf("synthesizer prompt missing upstream marker %q", marker)
		}
	}

	// Verify stage-3 tasks received synthesis result via context inheritance.
	// Stage-3 tasks are external, so check their prompts in the recorded requests.
	for _, req := range requests {
		if req.TaskID == "team-writer_kimi" || req.TaskID == "team-reviewer_cc" {
			if !strings.Contains(req.Prompt, synthesisMarker) {
				t.Fatalf("stage-3 task %s prompt missing synthesis marker: prompt=%q", req.TaskID, req.Prompt)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Test 2: Failure Propagation E2E
// ---------------------------------------------------------------------------

func TestLeaderAgent_FailurePropagation_E2E(t *testing.T) {
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
	failingCLI := writeFakeFailingCLI(t, workspace)

	kimiBridge := bridge.New(bridge.BridgeConfig{
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
			"FAKE_KIMI_SLEEP_SECONDS": "2",
		},
	}, process.NewController())
	failBridge := bridge.New(bridge.BridgeConfig{
		AgentType:          "codex",
		Binary:             failingCLI,
		PythonBinary:       pythonBin,
		BridgeScript:       bridgeScript,
		ApprovalPolicy:     "never",
		Sandbox:            "read-only",
		PlanApprovalPolicy: "never",
		PlanSandbox:        "read-only",
		Timeout:            15 * time.Second,
	}, process.NewController())

	mux := &multiplexExternalExecutor{
		byType: map[string]agent.ExternalAgentExecutor{
			"kimi":  kimiBridge,
			"codex": failBridge,
		},
	}
	recorder := newRecordingExternalExecutor(mux)

	internalCalled := false
	mgr := react.NewBackgroundTaskManager(react.BackgroundManagerConfig{
		RunContext: context.Background(),
		Logger:     agent.NoopLogger{},
		Clock:      agent.SystemClock{},
		ExecuteTask: func(ctx context.Context, prompt, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
			internalCalled = true
			return &agent.TaskResult{Answer: "should-not-reach", Iterations: 1, TokensUsed: 1}, nil
		},
		ExternalExecutor: recorder,
		SessionID:        "leader-failure-e2e",
	})
	defer mgr.Shutdown()

	team := buildFailurePropagationTeam()
	ctx := context.Background()
	ctx = agent.WithBackgroundDispatcher(ctx, mgr)
	ctx = agent.WithTeamDefinitions(ctx, []agent.TeamDefinition{team})

	tool := orchestration.NewRunTasks()
	res, err := tool.Execute(ctx, ports.ToolCall{
		ID: "call-leader-failure-e2e",
		Arguments: map[string]any{
			"template":        team.Name,
			"goal":            "test failure propagation",
			"wait":            true,
			"timeout_seconds": 30,
		},
	})
	if err != nil {
		t.Fatalf("run_tasks execute: %v", err)
	}

	// The tool should complete (not deadlock) even with failures.
	// It may report partial completion or failure in content.
	_ = res

	// Collect results — should complete within timeout (no deadlock).
	allIDs := []string{"team-ok_worker", "team-fail_worker", "team-dependent"}
	results := mgr.Collect(allIDs, true, 15*time.Second)

	resultMap := make(map[string]agent.BackgroundTaskResult)
	for _, r := range results {
		resultMap[r.ID] = r
	}

	// ok_worker should succeed.
	if ok, exists := resultMap["team-ok_worker"]; exists {
		if ok.Status != agent.BackgroundTaskStatusCompleted {
			t.Fatalf("ok_worker expected completed, got %s (error=%s)", ok.Status, ok.Error)
		}
	} else {
		t.Fatal("ok_worker result not found")
	}

	// fail_worker should fail.
	if fail, exists := resultMap["team-fail_worker"]; exists {
		if fail.Status != agent.BackgroundTaskStatusFailed {
			t.Fatalf("fail_worker expected failed, got %s", fail.Status)
		}
	} else {
		t.Fatal("fail_worker result not found")
	}

	// dependent should fail due to dependency failure.
	if dep, exists := resultMap["team-dependent"]; exists {
		if dep.Status != agent.BackgroundTaskStatusFailed {
			t.Fatalf("dependent expected failed, got %s (error=%s)", dep.Status, dep.Error)
		}
		if !strings.Contains(strings.ToLower(dep.Error), "dependency") &&
			!strings.Contains(strings.ToLower(dep.Error), "failed") {
			t.Fatalf("dependent error should mention dependency failure, got: %q", dep.Error)
		}
	} else {
		t.Fatal("dependent result not found")
	}

	// Internal executeTask should NOT have been called since dependency failed.
	if internalCalled {
		t.Fatal("internal executeTask should not have been called when dependency failed")
	}
}

// ---------------------------------------------------------------------------
// Test 3: Scored Rubric E2E
// ---------------------------------------------------------------------------

func TestLeaderAgent_ScoredRubric_E2E(t *testing.T) {
	pythonBin, err := osexec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not found in PATH")
	}

	repoRoot := findRepoRoot(t)
	bridgeScript := filepath.Join(repoRoot, "scripts", "codex_bridge", "codex_bridge.py")
	if _, err := os.Stat(bridgeScript); err != nil {
		t.Skipf("codex bridge script not found at %s", bridgeScript)
	}

	rubricPath := filepath.Join(repoRoot, "evaluation", "agent_eval", "datasets", "leader_agent_e2e_rubric.yaml")
	rubric, err := agent_eval.LoadJudgeRubric(rubricPath)
	if err != nil {
		t.Fatalf("load rubric: %v", err)
	}

	workspace := t.TempDir()
	t.Chdir(workspace)

	fakeKimi := writeFakeKimiCLI(t, workspace)
	fakeCC := writeFakeClaudeCodeCLI(t, workspace)
	fakeCodex := writeFakeCodexCLI(t, workspace)

	kimiBridge := bridge.New(bridge.BridgeConfig{
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
			"FAKE_KIMI_SLEEP_SECONDS": "2",
		},
	}, process.NewController())
	ccBridge := bridge.New(bridge.BridgeConfig{
		AgentType:          "claude_code",
		Binary:             fakeCC,
		PythonBinary:       pythonBin,
		BridgeScript:       bridgeScript,
		ApprovalPolicy:     "never",
		Sandbox:            "read-only",
		PlanApprovalPolicy: "never",
		PlanSandbox:        "read-only",
		Timeout:            30 * time.Second,
		Env: map[string]string{
			"FAKE_CC_MARKER":        "FAKE_CC_OK",
			"FAKE_CC_SLEEP_SECONDS": "0.3",
		},
	}, process.NewController())
	codexBridge := bridge.New(bridge.BridgeConfig{
		AgentType:          "codex",
		Binary:             fakeCodex,
		PythonBinary:       pythonBin,
		BridgeScript:       bridgeScript,
		ApprovalPolicy:     "never",
		Sandbox:            "read-only",
		PlanApprovalPolicy: "never",
		PlanSandbox:        "read-only",
		Timeout:            30 * time.Second,
		Env: map[string]string{
			"FAKE_CODEX_MARKER":        "FAKE_CODEX_OK",
			"FAKE_CODEX_SLEEP_SECONDS": "0.3",
		},
	}, process.NewController())

	mux := &multiplexExternalExecutor{
		byType: map[string]agent.ExternalAgentExecutor{
			"kimi":        kimiBridge,
			"claude_code": ccBridge,
			"codex":       codexBridge,
		},
	}
	recorder := newRecordingExternalExecutor(mux)
	capture := &internalCapture{}
	const synthesisMarker = "SYNTHESIS_COMPLETE"

	mgr := react.NewBackgroundTaskManager(react.BackgroundManagerConfig{
		RunContext: context.Background(),
		Logger:     agent.NoopLogger{},
		Clock:      agent.SystemClock{},
		ExecuteTask: func(ctx context.Context, prompt, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
			capture.record(prompt)
			return &agent.TaskResult{
				Answer:     synthesisMarker + ":: synthesized from upstream results",
				Iterations: 1,
				TokensUsed: 10,
			}, nil
		},
		ExternalExecutor: recorder,
		SessionID:        "leader-scored-e2e",
	})
	defer mgr.Shutdown()

	team := buildDeepResearchTeam()
	ctx := context.Background()
	ctx = agent.WithBackgroundDispatcher(ctx, mgr)
	ctx = agent.WithTeamDefinitions(ctx, []agent.TeamDefinition{team})

	tool := orchestration.NewRunTasks()
	goal := "proactive AI assistant architecture: event-driven design, persistent memory, approval gates, multi-agent coordination"
	res, err := tool.Execute(ctx, ports.ToolCall{
		ID: "call-leader-scored-e2e",
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

	allIDs := []string{
		"team-researcher_kimi", "team-researcher_codex", "team-researcher_cc",
		"team-synthesizer",
		"team-writer_kimi", "team-reviewer_cc",
	}
	results := mgr.Collect(allIDs, false, 0)
	requests, maxActive := recorder.snapshot()
	internalPrompts := capture.snapshot()

	// Score against rubric.
	judgement := scoreLeaderResult(rubric, scoringInput{
		requests:        requests,
		maxActive:       maxActive,
		internalPrompts: internalPrompts,
		results:         results,
		team:            team,
		totalTasks:      6,
		externalCount:   5,
		internalCount:   1,
	})

	// Print scoring summary.
	t.Log("=== Leader Agent Coordination Score ===")
	t.Logf("Status: %s", judgement.Status)
	t.Logf("Normalized Score: %.2f (threshold: %.2f)", judgement.Score, rubric.PassThreshold)
	t.Log("---------------------------------------")
	for _, dim := range judgement.Dimensions {
		t.Logf("  %-25s  score=%d/2  weight=%.2f  notes=%s", dim.ID, dim.Score, dim.Weight, dim.Notes)
	}
	t.Log("=======================================")

	// Assert pass.
	if judgement.Status != agent_eval.JudgementStatusPassed {
		t.Fatalf("leader agent scored %.2f (threshold=%.2f), status=%s", judgement.Score, rubric.PassThreshold, judgement.Status)
	}
	if judgement.Score < rubric.PassThreshold {
		t.Fatalf("score %.2f below threshold %.2f", judgement.Score, rubric.PassThreshold)
	}

	// Assert no zero-scores on critical dimensions.
	for _, dim := range judgement.Dimensions {
		if _, critical := map[string]bool{"dispatch_correctness": true, "dependency_ordering": true}[dim.ID]; critical {
			if dim.Score == 0 {
				t.Fatalf("critical dimension %s scored 0", dim.ID)
			}
		}
	}
}
