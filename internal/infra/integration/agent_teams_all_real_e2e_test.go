//go:build integration

package integration

import (
	"context"
	"fmt"
	osexec "os/exec"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"alex/evaluation/agent_eval"
	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/react"
	"alex/internal/infra/external/bridge"
	"alex/internal/infra/tools/builtin/orchestration"
)

// ---------------------------------------------------------------------------
// Helpers — real CLI executors for all three agent types
// ---------------------------------------------------------------------------

func requireCLI(t *testing.T, name string) string {
	t.Helper()
	path, err := osexec.LookPath(name)
	if err != nil {
		t.Skipf("%s CLI not found in PATH", name)
	}
	return path
}

func requireBridgeScript(t *testing.T, subdir, filename string) string {
	t.Helper()
	repoRoot := findRepoRoot(t)
	script := filepath.Join(repoRoot, "scripts", subdir, filename)
	if _, err := os.Stat(script); err != nil {
		t.Fatalf("bridge script not found at %s", script)
	}
	return script
}

func requireCCBridgePython(t *testing.T) string {
	t.Helper()
	repoRoot := findRepoRoot(t)
	venvPython := filepath.Join(repoRoot, "scripts", "cc_bridge", ".venv", "bin", "python3")
	if _, err := os.Stat(venvPython); err != nil {
		t.Fatalf("cc_bridge venv not found at %s — run scripts/cc_bridge/setup.sh first", venvPython)
	}
	return venvPython
}

func newRealBridgeExecutor(t *testing.T, agentType, binary, pythonBin, bridgeScript string, timeout time.Duration) *bridge.Executor {
	t.Helper()
	cfg := bridge.BridgeConfig{
		AgentType:          agentType,
		Binary:             binary,
		PythonBinary:       pythonBin,
		BridgeScript:       bridgeScript,
		ApprovalPolicy:     "never",
		Sandbox:            "read-only",
		PlanApprovalPolicy: "never",
		PlanSandbox:        "read-only",
		Timeout:            timeout,
	}
	// Claude Code bridge uses mode=autonomous + allowed_tools instead of
	// approval_policy/sandbox, which are codex/kimi concepts.
	if agentType == "claude_code" {
		cfg.DefaultMode = "autonomous"
	}
	return bridge.New(cfg)
}

// ---------------------------------------------------------------------------
// Test: All-Real Deep Research E2E
//
// 3 stages, 6 roles, ALL real CLIs (kimi + codex + claude_code):
//   Stage 1 (research, parallel): real kimi + real codex + real claude_code
//   Stage 2 (synthesis):          internal agent (captures context)
//   Stage 3 (delivery, parallel): real kimi (writer) + real claude_code (reviewer)
//
// Scoring rubric:
//   1. dispatch_correctness  — all 5 external requests match role specs
//   2. parallelism           — stage-1 maxActive >= 2
//   3. dependency_ordering   — stages execute in order
//   4. context_inheritance   — synthesizer receives all 3 upstream outputs
//   5. cross_stage_delivery  — stage-3 receives synthesis result
//   6. content_quality       — real answers are substantive (>50 chars)
//   7. result_completeness   — all 6 tasks completed with non-empty answers
// ---------------------------------------------------------------------------

func TestAllReal_DeepResearch_E2E(t *testing.T) {
	pythonBin := requireCLI(t, "python3")
	kimiBin := requireCLI(t, "kimi")
	codexBin := requireCLI(t, "codex")
	claudeBin := requireCLI(t, "claude")

	kimiScript := requireBridgeScript(t, "kimi_bridge", "kimi_bridge.py")
	codexScript := requireBridgeScript(t, "codex_bridge", "codex_bridge.py")
	ccScript := requireBridgeScript(t, "cc_bridge", "cc_bridge.py")
	ccPython := requireCCBridgePython(t)

	repoRoot := findRepoRoot(t)

	workspace := t.TempDir()
	t.Chdir(workspace)

	// --- Build real executors for all 3 types ---

	_ = repoRoot // used later for rubric
	kimiBridge := newRealBridgeExecutor(t, "kimi", kimiBin, pythonBin, kimiScript, 180*time.Second)
	codexBridge := newRealBridgeExecutor(t, "codex", codexBin, pythonBin, codexScript, 180*time.Second)
	ccBridge := newRealBridgeExecutor(t, "claude_code", claudeBin, ccPython, ccScript, 180*time.Second)

	mux := &multiplexExternalExecutor{
		byType: map[string]agent.ExternalAgentExecutor{
			"kimi":        kimiBridge,
			"codex":       codexBridge,
			"claude_code": ccBridge,
		},
	}
	recorder := newRecordingExternalExecutor(mux)

	// Track internal agent prompts for context inheritance verification.
	capture := &internalCapture{}
	const synthesisMarker = "ALL_REAL_SYNTHESIS_COMPLETE"

	mgr := react.NewBackgroundTaskManager(react.BackgroundManagerConfig{
		RunContext: context.Background(),
		Logger:     agent.NoopLogger{},
		Clock:      agent.SystemClock{},
		ExecuteTask: func(ctx context.Context, prompt, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
			capture.record(prompt)
			return &agent.TaskResult{
				Answer:     synthesisMarker + ":: synthesized from all-real upstream research",
				Iterations: 1,
				TokensUsed: 10,
			}, nil
		},
		ExternalExecutor: recorder,
		SessionID:        "all-real-deep-research-e2e",
	})
	defer mgr.Shutdown()

	team := buildDeepResearchTeam()
	ctx := context.Background()
	ctx = agent.WithBackgroundDispatcher(ctx, mgr)
	ctx = agent.WithTeamDefinitions(ctx, []agent.TeamDefinition{team})

	// --- Execute ---

	goal := "Compare Saga vs 2PC vs TCC for distributed transaction in a Go microservice system with 10+ services"
	res, err := orchestration.NewRunTasks().Execute(ctx, ports.ToolCall{
		ID: "call-all-real-deep-research-e2e",
		Arguments: map[string]any{
			"template":        team.Name,
			"goal":            goal,
			"wait":            true,
			"timeout_seconds": 600,
		},
	})
	if err != nil {
		t.Fatalf("run_tasks execute: %v", err)
	}
	if res.Error != nil {
		t.Fatalf("run_tasks tool error: %v, content=%s", res.Error, res.Content)
	}

	// --- Collect results ---

	allIDs := []string{
		"team-researcher_kimi", "team-researcher_codex", "team-researcher_cc",
		"team-synthesizer",
		"team-writer_kimi", "team-reviewer_cc",
	}
	results := mgr.Collect(allIDs, true, 300*time.Second)
	if len(results) != 6 {
		t.Fatalf("expected 6 results, got %d", len(results))
	}

	resultMap := make(map[string]agent.BackgroundTaskResult)
	for _, r := range results {
		resultMap[r.ID] = r
	}

	requests, maxActive := recorder.snapshot()
	internalPrompts := capture.snapshot()

	// --- Score ---

	rubricPath := filepath.Join(repoRoot, "evaluation", "agent_eval", "datasets", "leader_agent_e2e_rubric.yaml")
	rubric, err := agent_eval.LoadJudgeRubric(rubricPath)
	if err != nil {
		t.Fatalf("load rubric: %v", err)
	}

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

	// --- Print detailed results ---

	t.Log("=== All-Real Deep Research Coordination Score ===")
	t.Logf("Status: %s", judgement.Status)
	t.Logf("Normalized Score: %.2f (threshold: %.2f)", judgement.Score, rubric.PassThreshold)
	t.Log("-------------------------------------------------")
	for _, dim := range judgement.Dimensions {
		t.Logf("  %-25s  score=%d/2  weight=%.2f  notes=%s", dim.ID, dim.Score, dim.Weight, dim.Notes)
	}
	t.Log("=================================================")

	// --- Individual task analysis ---

	t.Log("")
	t.Log("=== Individual Task Results ===")
	for _, id := range allIDs {
		r := resultMap[id]
		answerLen := len(r.Answer)
		preview := r.Answer
		if len(preview) > 150 {
			preview = preview[:150] + "..."
		}
		t.Logf("  %-25s  status=%-10s  answer_len=%d  preview=%q", id, r.Status, answerLen, preview)
	}

	// --- Detailed checks beyond rubric ---

	// Check real agent answers are substantive
	realAgentIDs := []string{
		"team-researcher_kimi", "team-researcher_codex", "team-researcher_cc",
		"team-writer_kimi", "team-reviewer_cc",
	}
	for _, id := range realAgentIDs {
		r := resultMap[id]
		if r.Status != agent.BackgroundTaskStatusCompleted {
			t.Errorf("%s: expected completed, got %s (error=%s)", id, r.Status, r.Error)
			continue
		}
		if len(r.Answer) < 50 {
			t.Errorf("%s: answer too short for real agent (%d chars): %q", id, len(r.Answer), r.Answer)
		}
	}

	// Check context inheritance
	if len(internalPrompts) >= 1 {
		synthPrompt := internalPrompts[0]
		if !strings.Contains(synthPrompt, "[Collaboration Context]") {
			t.Error("synthesizer prompt missing [Collaboration Context]")
		}
		t.Logf("")
		t.Logf("=== Synthesizer Context (first 500 chars) ===")
		preview := synthPrompt
		if len(preview) > 500 {
			preview = preview[:500]
		}
		t.Logf("%s", preview)
	}

	// Check cross-stage delivery
	for _, req := range requests {
		if req.TaskID == "team-writer_kimi" || req.TaskID == "team-reviewer_cc" {
			if !strings.Contains(req.Prompt, synthesisMarker) {
				t.Errorf("stage-3 task %s prompt missing synthesis marker", req.TaskID)
			}
		}
	}

	// --- Final verdict ---

	t.Log("")
	t.Log("=== VERDICT ===")
	if judgement.Status != agent_eval.JudgementStatusPassed {
		t.Errorf("FAILED: scored %.2f (threshold=%.2f), status=%s", judgement.Score, rubric.PassThreshold, judgement.Status)
	} else {
		t.Logf("PASSED: scored %.2f (threshold=%.2f)", judgement.Score, rubric.PassThreshold)
	}
}

// ---------------------------------------------------------------------------
// Test: 5 Case Suite — runs all 5 deep research cases sequentially
// ---------------------------------------------------------------------------

func TestAllReal_DeepResearch_5Cases(t *testing.T) {
	pythonBin := requireCLI(t, "python3")
	kimiBin := requireCLI(t, "kimi")
	codexBin := requireCLI(t, "codex")
	claudeBin := requireCLI(t, "claude")

	kimiScript := requireBridgeScript(t, "kimi_bridge", "kimi_bridge.py")
	codexScript := requireBridgeScript(t, "codex_bridge", "codex_bridge.py")
	ccScript := requireBridgeScript(t, "cc_bridge", "cc_bridge.py")
	ccPython := requireCCBridgePython(t)

	cases := []struct {
		name string
		goal string
	}{
		{
			name: "Case1_DistributedTransaction",
			goal: "Compare Saga vs 2PC vs TCC for distributed transaction in a Go microservice system with 10+ services, considering failure recovery, observability, and team adoption cost",
		},
		{
			name: "Case2_MemorySubsystemEvolution",
			goal: "Analyze memory subsystem architecture patterns, compare with industry patterns (MemGPT, LangGraph persistence, Letta), and propose evolution path",
		},
		{
			name: "Case3_MultiAgentFrameworkComparison",
			goal: "Compare CrewAI, AutoGen, LangGraph multi-agent orchestration: orchestration model, context sharing, failure handling, extensibility",
		},
		{
			name: "Case4_EventDrivenVsRequestResponse",
			goal: "Compare event-driven vs request-response architecture for real-time AI assistant systems: latency, scalability, complexity, debugging",
		},
		{
			name: "Case5_DatabaseMultiRegion",
			goal: "Compare PostgreSQL vs CockroachDB vs TiDB for global multi-region deployment with strong consistency requirements",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			workspace := t.TempDir()
			t.Chdir(workspace)

			kimiBridge := newRealBridgeExecutor(t, "kimi", kimiBin, pythonBin, kimiScript, 180*time.Second)
			codexBridge := newRealBridgeExecutor(t, "codex", codexBin, pythonBin, codexScript, 180*time.Second)
			ccBridge := newRealBridgeExecutor(t, "claude_code", claudeBin, ccPython, ccScript, 180*time.Second)

			mux := &multiplexExternalExecutor{
				byType: map[string]agent.ExternalAgentExecutor{
					"kimi":        kimiBridge,
					"codex":       codexBridge,
					"claude_code": ccBridge,
				},
			}
			recorder := newRecordingExternalExecutor(mux)
			capture := &internalCapture{}
			const synthMarker = "SYNTH_5CASE"

			mgr := react.NewBackgroundTaskManager(react.BackgroundManagerConfig{
				RunContext: context.Background(),
				Logger:     agent.NoopLogger{},
				Clock:      agent.SystemClock{},
				ExecuteTask: func(ctx context.Context, prompt, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
					capture.record(prompt)
					return &agent.TaskResult{
						Answer:     synthMarker + ":: synthesized from upstream",
						Iterations: 1,
						TokensUsed: 10,
					}, nil
				},
				ExternalExecutor: recorder,
				SessionID:        fmt.Sprintf("5case-%s", tc.name),
			})
			defer mgr.Shutdown()

			team := buildDeepResearchTeam()
			ctx := context.Background()
			ctx = agent.WithBackgroundDispatcher(ctx, mgr)
			ctx = agent.WithTeamDefinitions(ctx, []agent.TeamDefinition{team})

			res, err := orchestration.NewRunTasks().Execute(ctx, ports.ToolCall{
				ID: fmt.Sprintf("call-5case-%s", tc.name),
				Arguments: map[string]any{
					"template":        team.Name,
					"goal":            tc.goal,
					"wait":            true,
					"timeout_seconds": 600,
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
			results := mgr.Collect(allIDs, true, 300*time.Second)
			requests, maxActive := recorder.snapshot()

			// Summary
			completed := 0
			failed := 0
			for _, r := range results {
				if r.Status == agent.BackgroundTaskStatusCompleted {
					completed++
				} else {
					failed++
					t.Logf("  FAILED: %s status=%s error=%s", r.ID, r.Status, r.Error)
				}
			}

			t.Logf("Results: %d/%d completed, %d failed, maxActive=%d", completed, len(allIDs), failed, maxActive)

			// Per-role detail
			for _, r := range results {
				preview := r.Answer
				if len(preview) > 100 {
					preview = preview[:100] + "..."
				}
				t.Logf("  %-25s  status=%-10s  len=%d  %q", r.ID, r.Status, len(r.Answer), preview)
			}

			// Context check
			prompts := capture.snapshot()
			if len(prompts) > 0 && strings.Contains(prompts[0], "[Collaboration Context]") {
				t.Log("  Context inheritance: OK")
			} else {
				t.Log("  Context inheritance: MISSING")
			}

			// Score
			t.Logf("  External requests: %d, maxActive: %d", len(requests), maxActive)

			if completed < 4 {
				t.Errorf("too many failures: only %d/%d completed", completed, len(allIDs))
			}
		})
	}
}
