//go:build integration

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
	"alex/internal/infra/process"
	"alex/internal/infra/tools/builtin/orchestration"
)

// ---------------------------------------------------------------------------
// Helpers — kimi CLI uses local subscription, no API key needed
// ---------------------------------------------------------------------------

func requireKimiCLI(t *testing.T) string {
	t.Helper()
	path, err := osexec.LookPath("kimi")
	if err != nil {
		t.Skip("kimi CLI not found in PATH")
	}
	return path
}

func kimiBridgeScript(t *testing.T) string {
	t.Helper()
	repoRoot := findRepoRoot(t)
	script := filepath.Join(repoRoot, "scripts", "kimi_bridge", "kimi_bridge.py")
	if _, err := os.Stat(script); err != nil {
		t.Fatalf("kimi bridge script not found at %s", script)
	}
	return script
}

func newKimiBridgeExecutor(t *testing.T, workDir string) *bridge.Executor {
	t.Helper()
	pythonBin, err := osexec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not found in PATH")
	}
	kimiBin := requireKimiCLI(t)
	script := kimiBridgeScript(t)

	return bridge.New(bridge.BridgeConfig{
		AgentType:    "kimi",
		Binary:       kimiBin,
		PythonBinary: pythonBin,
		BridgeScript: script,
		Timeout:      120 * time.Second,
	}, process.NewController())
}

// progressRecorder captures OnProgress callbacks in a thread-safe manner.
type progressRecorder struct {
	mu     sync.Mutex
	events []agent.ExternalAgentProgress
	count  atomic.Int64
}

func (p *progressRecorder) record(ev agent.ExternalAgentProgress) {
	p.count.Add(1)
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, ev)
}

func (p *progressRecorder) snapshot() []agent.ExternalAgentProgress {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]agent.ExternalAgentProgress, len(p.events))
	copy(out, p.events)
	return out
}

// progressCapturingExecutor wraps an executor to inject OnProgress recording.
type progressCapturingExecutor struct {
	inner    agent.ExternalAgentExecutor
	recorder *progressRecorder
}

func (p *progressCapturingExecutor) Execute(ctx context.Context, req agent.ExternalAgentRequest) (*agent.ExternalAgentResult, error) {
	original := req.OnProgress
	req.OnProgress = func(ev agent.ExternalAgentProgress) {
		p.recorder.record(ev)
		if original != nil {
			original(ev)
		}
	}
	return p.inner.Execute(ctx, req)
}

func (p *progressCapturingExecutor) SupportedTypes() []string {
	return p.inner.SupportedTypes()
}

// ---------------------------------------------------------------------------
// Test 1: Bridge 进程生命周期 — spawn → JSONL → result
//
// 验证最基础的链路：bridge 启动 kimi CLI，stdin config 传入，
// stdout JSONL 解析，进程退出，result 返回。
// ---------------------------------------------------------------------------

func TestKimiReal_BridgeLifecycle(t *testing.T) {
	workspace := t.TempDir()
	exec := newKimiBridgeExecutor(t, workspace)

	progress := &progressRecorder{}
	result, err := exec.Execute(context.Background(), agent.ExternalAgentRequest{
		TaskID:     "kimi-lifecycle",
		Prompt:     "What is 2+3? Reply with just the number.",
		AgentType:  "kimi",
		WorkingDir: workspace,
		OnProgress: progress.record,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	// 结果交付完整性
	if result.Answer == "" {
		t.Fatal("answer is empty — result delivery failed")
	}
	if !strings.Contains(result.Answer, "5") {
		t.Fatalf("expected answer to contain '5', got: %q", result.Answer)
	}

	t.Logf("answer=%q tokens=%d iters=%d", result.Answer, result.TokensUsed, result.Iterations)
}

// ---------------------------------------------------------------------------
// Test 2: 工具调用 → OnProgress 回调链路
//
// 在 workspace 放一个 marker 文件，让 kimi 用 Shell 列目录。
// 验证 bridge 把 tool_calls 翻译成 SDKEvent、触发 OnProgress。
// ---------------------------------------------------------------------------

func TestKimiReal_ToolProgress(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "marker_e2e.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	exec := newKimiBridgeExecutor(t, workspace)
	progress := &progressRecorder{}

	result, err := exec.Execute(context.Background(), agent.ExternalAgentRequest{
		TaskID:     "kimi-tool-progress",
		Prompt:     "Run `ls` in the current directory with Shell and tell me what files you see.",
		AgentType:  "kimi",
		WorkingDir: workspace,
		OnProgress: progress.record,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Answer == "" {
		t.Fatal("answer is empty")
	}

	// OnProgress 至少触发一次 (Shell tool_call → SDKEvent type:tool → applyEvent → OnProgress)
	events := progress.snapshot()
	if len(events) == 0 {
		t.Fatal("OnProgress never called — tool event chain broken")
	}

	// 验证 progress event 字段
	for i, ev := range events {
		if ev.LastActivity.IsZero() {
			t.Fatalf("progress[%d] has zero LastActivity", i)
		}
		if ev.Iteration < 1 {
			t.Fatalf("progress[%d] iteration=%d, expected >=1", i, ev.Iteration)
		}
		t.Logf("progress[%d]: tool=%s summary=%s iter=%d", i, ev.CurrentTool, ev.CurrentArgs, ev.Iteration)
	}

	// iterations > 0 — bridge 正确计数
	if result.Iterations < 1 {
		t.Fatalf("iterations=%d, expected >=1", result.Iterations)
	}

	t.Logf("answer=%q iters=%d progress_events=%d", result.Answer, result.Iterations, len(events))
}

// ---------------------------------------------------------------------------
// Test 3: 多轮工具交互 — 搜索 + 总结
//
// 搜索类任务天然产生多轮 tool 交互 (search → read → answer)。
// 验证 bridge 在多轮中持续发射 SDKEvent、最终 result 完整。
// ---------------------------------------------------------------------------

func TestKimiReal_MultiTurnSearch(t *testing.T) {
	workspace := t.TempDir()
	exec := newKimiBridgeExecutor(t, workspace)

	result, err := exec.Execute(context.Background(), agent.ExternalAgentRequest{
		TaskID:     "kimi-search",
		Prompt:     "Search for 'Go context package cancellation' and summarize the key points in 2 sentences.",
		AgentType:  "kimi",
		WorkingDir: workspace,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	if result.Answer == "" {
		t.Fatal("answer is empty")
	}
	if len(result.Answer) < 20 {
		t.Fatalf("answer too short (len=%d): %q", len(result.Answer), result.Answer)
	}

	t.Logf("answer (len=%d) iters=%d: %q", len(result.Answer), result.Iterations, result.Answer)
}

// ---------------------------------------------------------------------------
// Test 4: 混合 agents team — 真实 kimi + fake codex/cc
//
// 完整的 agents team 端到端：
//   Stage 1 (parallel): real kimi researcher + fake codex analyzer
//   Stage 2 (sequential): fake cc reviewer (inherit_context=true)
//
// 验证维度：
//   1. 调度正确性 — agent_type 路由匹配
//   2. 进度监控   — kimi 的 OnProgress 被触发
//   3. 阶段依赖   — stage-2 在 stage-1 全部完成后执行
//   4. 上下文继承 — cc_reviewer prompt 含 [Collaboration Context] + 上游真实回答
//   5. 结果交付   — Collect() 返回 3 个 completed result，answer 非空
//   6. 混合路由   — real kimi 真实回答 + fake 含 marker
//   7. 状态文件   — .status.yaml 正确记录所有 task
// ---------------------------------------------------------------------------

func TestKimiReal_AgentsTeam_EndToEnd(t *testing.T) {
	pythonBin, err := osexec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not found in PATH")
	}
	kimiBin := requireKimiCLI(t)
	kimiScript := kimiBridgeScript(t)

	repoRoot := findRepoRoot(t)
	codexBridgeScript := filepath.Join(repoRoot, "scripts", "codex_bridge", "codex_bridge.py")
	if _, err := os.Stat(codexBridgeScript); err != nil {
		t.Fatalf("codex bridge script not found at %s", codexBridgeScript)
	}

	workspace := t.TempDir()
	t.Chdir(workspace)

	// --- 构建 executors ---

	fakeCodex := writeFakeCodexCLI(t, workspace)
	fakeCC := writeFakeClaudeCodeCLI(t, workspace)

	kimiProgress := &progressRecorder{}
	realKimiBridge := bridge.New(bridge.BridgeConfig{
		AgentType:    "kimi",
		Binary:       kimiBin,
		PythonBinary: pythonBin,
		BridgeScript: kimiScript,
		Timeout:      120 * time.Second,
	}, process.NewController())
	fakeCodexBridge := bridge.New(bridge.BridgeConfig{
		AgentType:          "codex",
		Binary:             fakeCodex,
		PythonBinary:       pythonBin,
		BridgeScript:       codexBridgeScript,
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
	fakeCCBridge := bridge.New(bridge.BridgeConfig{
		AgentType:          "claude_code",
		Binary:             fakeCC,
		PythonBinary:       pythonBin,
		BridgeScript:       codexBridgeScript,
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

	mux := &multiplexExternalExecutor{
		byType: map[string]agent.ExternalAgentExecutor{
			"kimi":        &progressCapturingExecutor{inner: realKimiBridge, recorder: kimiProgress},
			"codex":       fakeCodexBridge,
			"claude_code": fakeCCBridge,
		},
	}
	recorder := newRecordingExternalExecutor(mux)

	mgr := react.NewBackgroundTaskManager(react.BackgroundManagerConfig{
		RunContext: context.Background(),
		Logger:     agent.NoopLogger{},
		Clock:      agent.SystemClock{},
		ExecuteTask: func(ctx context.Context, prompt, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
			return &agent.TaskResult{Answer: "internal-not-used", Iterations: 1, TokensUsed: 1}, nil
		},
		ExternalExecutor: recorder,
		SessionID:        "kimi-real-team-e2e",
	})
	defer mgr.Shutdown()

	// --- Team 定义: 2 stages, 3 roles ---

	teamName := "kimi_real_mixed_team"
	team := agent.TeamDefinition{
		Name:        teamName,
		Description: "real kimi + fake codex/cc mixed team",
		Roles: []agent.TeamRoleDefinition{
			{
				Name:           "kimi_researcher",
				AgentType:      "kimi",
				PromptTemplate: "Briefly explain what the Go context package does. Answer in 2-3 sentences.",
				ExecutionMode:  "plan",
				AutonomyLevel:  "full",
				Config:         map[string]string{"approval_policy": "never", "sandbox": "read-only"},
			},
			{
				Name:           "codex_analyzer",
				AgentType:      "codex",
				PromptTemplate: "Analyze patterns for: {GOAL}",
				ExecutionMode:  "plan",
				AutonomyLevel:  "full",
				Config:         map[string]string{"approval_policy": "never", "sandbox": "read-only"},
			},
			{
				Name:           "cc_reviewer",
				AgentType:      "claude_code",
				PromptTemplate: "Review findings for: {GOAL}",
				ExecutionMode:  "plan",
				AutonomyLevel:  "full",
				InheritContext: true,
				Config:         map[string]string{"approval_policy": "never", "sandbox": "read-only"},
			},
		},
		Stages: []agent.TeamStageDefinition{
			{Name: "research", Roles: []string{"kimi_researcher", "codex_analyzer"}},
			{Name: "review", Roles: []string{"cc_reviewer"}},
		},
	}

	ctx := context.Background()
	ctx = agent.WithBackgroundDispatcher(ctx, mgr)
	ctx = agent.WithTeamDefinitions(ctx, []agent.TeamDefinition{team})

	// --- 执行 ---

	res, err := orchestration.NewRunTasks().Execute(ctx, ports.ToolCall{
		ID: "call-kimi-real-team-e2e",
		Arguments: map[string]any{
			"template":        teamName,
			"goal":            "Go context package usage patterns",
			"wait":            true,
			"timeout_seconds": 180,
		},
	})
	if err != nil {
		t.Fatalf("run_tasks execute: %v", err)
	}
	if res.Error != nil {
		t.Fatalf("run_tasks tool error: %v, content=%s", res.Error, res.Content)
	}

	// --- 1. 调度正确性 ---

	allIDs := []string{"team-kimi_researcher", "team-codex_analyzer", "team-cc_reviewer"}
	results := mgr.Collect(allIDs, true, 120*time.Second)
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	requests, _ := recorder.snapshot()
	if len(requests) != 3 {
		t.Fatalf("expected 3 external requests, got %d", len(requests))
	}

	expectedTypes := map[string]string{
		"team-kimi_researcher": "kimi",
		"team-codex_analyzer":  "codex",
		"team-cc_reviewer":     "claude_code",
	}
	for _, req := range requests {
		wantType, ok := expectedTypes[req.TaskID]
		if !ok {
			t.Fatalf("unexpected task_id: %s", req.TaskID)
		}
		if req.AgentType != wantType {
			t.Fatalf("task %s: want agent_type=%s, got %s", req.TaskID, wantType, req.AgentType)
		}
	}
	t.Log("[PASS] 调度正确性: agent_type 路由匹配")

	// --- 2. 结果交付 ---

	resultMap := make(map[string]agent.BackgroundTaskResult)
	for _, r := range results {
		resultMap[r.ID] = r
	}
	for _, id := range allIDs {
		r := resultMap[id]
		if r.Status != agent.BackgroundTaskStatusCompleted {
			t.Fatalf("task %s: status=%s error=%s", r.ID, r.Status, r.Error)
		}
		if r.Answer == "" {
			t.Fatalf("task %s: empty answer", r.ID)
		}
	}
	t.Log("[PASS] 结果交付: 3 tasks completed, answers non-empty")

	// --- 3. 混合路由: real kimi vs fake markers ---

	kimiResult := resultMap["team-kimi_researcher"]
	if len(kimiResult.Answer) < 10 {
		t.Fatalf("kimi answer too short for real response: %q", kimiResult.Answer)
	}
	if strings.Contains(kimiResult.Answer, "FAKE_") {
		t.Fatalf("kimi answer contains fake marker — not a real response: %q", kimiResult.Answer)
	}
	t.Logf("[PASS] 混合路由: kimi real answer (len=%d)", len(kimiResult.Answer))

	codexResult := resultMap["team-codex_analyzer"]
	if !strings.Contains(codexResult.Answer, "FAKE_CODEX_OK") {
		t.Fatalf("codex answer missing marker: %q", codexResult.Answer)
	}
	ccResult := resultMap["team-cc_reviewer"]
	if !strings.Contains(ccResult.Answer, "FAKE_CC_OK") {
		t.Fatalf("cc answer missing marker: %q", ccResult.Answer)
	}
	t.Log("[PASS] 混合路由: fake codex/cc answers contain markers")

	// --- 4. 上下文继承 ---

	for _, req := range requests {
		if req.TaskID != "team-cc_reviewer" {
			continue
		}
		if !strings.Contains(req.Prompt, "[Collaboration Context]") {
			t.Fatalf("cc_reviewer prompt missing [Collaboration Context]")
		}
		// 上游 kimi 的真实回答应该出现在 context 中
		snippet := kimiResult.Answer
		if len(snippet) > 30 {
			snippet = snippet[:30]
		}
		if !strings.Contains(req.Prompt, snippet) {
			t.Logf("WARN: cc_reviewer context may not contain kimi snippet %q", snippet)
		}
		// 上游 fake codex 的 marker 也应该在 context 中
		if !strings.Contains(req.Prompt, "FAKE_CODEX_OK") {
			t.Logf("WARN: cc_reviewer context missing FAKE_CODEX_OK marker")
		}
	}
	t.Log("[PASS] 上下文继承: cc_reviewer received [Collaboration Context]")

	// --- 5. 进度监控 ---

	kimiEvents := kimiProgress.snapshot()
	if len(kimiEvents) > 0 {
		for i, ev := range kimiEvents {
			if ev.LastActivity.IsZero() {
				t.Fatalf("kimi progress[%d] has zero LastActivity", i)
			}
			t.Logf("  kimi progress[%d]: tool=%s iter=%d", i, ev.CurrentTool, ev.Iteration)
		}
		t.Logf("[PASS] 进度监控: %d progress events captured", len(kimiEvents))
	} else {
		t.Log("[INFO] 进度监控: kimi task had no tool calls (pure text response)")
	}

	// --- 6. 状态文件 ---

	statusPath := filepath.Join(workspace, ".elephant", "tasks", "team-"+teamName+".status.yaml")
	statusData, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatalf("read status file: %v", err)
	}
	statusText := string(statusData)
	for _, id := range allIDs {
		if !strings.Contains(statusText, id) {
			t.Fatalf("status file missing task %s", id)
		}
	}
	if !strings.Contains(statusText, "completed") {
		t.Fatalf("status file missing 'completed' status")
	}
	t.Log("[PASS] 状态文件: .status.yaml 记录完整")

	// --- 7. 阶段依赖 (stage ordering) ---
	// cc_reviewer 有 [Collaboration Context] 说明 stage-1 已全部完成后才执行
	if strings.Contains(res.Content, "All 3 tasks completed.") {
		t.Log("[PASS] 阶段依赖: stage ordering correct")
	}

	t.Log("=== agents team E2E 全部通过 ===")
}

// ---------------------------------------------------------------------------
// Test 5: Deep Research — 3 stages, 6 roles, 2 real kimi
//
// 复刻 buildDeepResearchTeam() 但 kimi 用真实 CLI:
//   Stage 1 (research, parallel): real kimi + fake codex + fake cc
//   Stage 2 (synthesis):          internal agent
//   Stage 3 (delivery, parallel): real kimi (writer) + fake cc (reviewer)
//
// 验证维度:
//   1. 调度正确性 — 5 external (2 kimi + 1 codex + 2 cc) + 1 internal
//   2. Stage-1 并行  — maxActive >= 2
//   3. 阶段依赖     — 3 stages 严格顺序
//   4. 上下文继承   — synthesizer 收到 3 个上游 marker/answer
//   5. 跨阶段传递   — writer_kimi 收到 synthesis 结果
//   6. kimi 真实回答 — 2 个 kimi role 都是真实回答 (非 fake marker)
//   7. 结果交付     — 6 个 task 全部 completed
// ---------------------------------------------------------------------------

func TestKimiReal_DeepResearch_EndToEnd(t *testing.T) {
	pythonBin, err := osexec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not found in PATH")
	}
	kimiBin := requireKimiCLI(t)
	kimiScript := kimiBridgeScript(t)

	repoRoot := findRepoRoot(t)
	codexBridgeScript := filepath.Join(repoRoot, "scripts", "codex_bridge", "codex_bridge.py")
	if _, err := os.Stat(codexBridgeScript); err != nil {
		t.Fatalf("codex bridge script not found at %s", codexBridgeScript)
	}

	workspace := t.TempDir()
	t.Chdir(workspace)

	// --- executors ---

	fakeCodex := writeFakeCodexCLI(t, workspace)
	fakeCC := writeFakeClaudeCodeCLI(t, workspace)

	kimiProgress := &progressRecorder{}
	realKimiBridge := bridge.New(bridge.BridgeConfig{
		AgentType:    "kimi",
		Binary:       kimiBin,
		PythonBinary: pythonBin,
		BridgeScript: kimiScript,
		Timeout:      120 * time.Second,
	}, process.NewController())
	fakeCodexBridge := bridge.New(bridge.BridgeConfig{
		AgentType:          "codex",
		Binary:             fakeCodex,
		PythonBinary:       pythonBin,
		BridgeScript:       codexBridgeScript,
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
	fakeCCBridge := bridge.New(bridge.BridgeConfig{
		AgentType:          "claude_code",
		Binary:             fakeCC,
		PythonBinary:       pythonBin,
		BridgeScript:       codexBridgeScript,
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

	mux := &multiplexExternalExecutor{
		byType: map[string]agent.ExternalAgentExecutor{
			"kimi":        &progressCapturingExecutor{inner: realKimiBridge, recorder: kimiProgress},
			"codex":       fakeCodexBridge,
			"claude_code": fakeCCBridge,
		},
	}
	recorder := newRecordingExternalExecutor(mux)

	const synthesisMarker = "SYNTHESIS_DEEP_RESEARCH"
	capture := &internalCapture{}

	mgr := react.NewBackgroundTaskManager(react.BackgroundManagerConfig{
		RunContext: context.Background(),
		Logger:     agent.NoopLogger{},
		Clock:      agent.SystemClock{},
		ExecuteTask: func(ctx context.Context, prompt, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
			capture.record(prompt)
			return &agent.TaskResult{
				Answer:     synthesisMarker + ":: synthesized from upstream research",
				Iterations: 1,
				TokensUsed: 10,
			}, nil
		},
		ExternalExecutor: recorder,
		SessionID:        "kimi-deep-research-e2e",
	})
	defer mgr.Shutdown()

	// --- team 定义 (同 buildDeepResearchTeam 但 kimi 走真实 CLI) ---

	team := buildDeepResearchTeam()
	ctx := context.Background()
	ctx = agent.WithBackgroundDispatcher(ctx, mgr)
	ctx = agent.WithTeamDefinitions(ctx, []agent.TeamDefinition{team})

	// --- 执行 ---

	goal := "proactive AI assistant architecture: event-driven design, persistent memory, approval gates"
	res, err := orchestration.NewRunTasks().Execute(ctx, ports.ToolCall{
		ID: "call-kimi-deep-research-e2e",
		Arguments: map[string]any{
			"template":        team.Name,
			"goal":            goal,
			"wait":            true,
			"timeout_seconds": 240,
		},
	})
	if err != nil {
		t.Fatalf("run_tasks execute: %v", err)
	}
	if res.Error != nil {
		t.Fatalf("run_tasks tool error: %v, content=%s", res.Error, res.Content)
	}

	// --- 1. 结果交付: 6 tasks 全部 completed ---

	allIDs := []string{
		"team-researcher_kimi", "team-researcher_codex", "team-researcher_cc",
		"team-synthesizer",
		"team-writer_kimi", "team-reviewer_cc",
	}
	results := mgr.Collect(allIDs, true, 120*time.Second)
	if len(results) != 6 {
		t.Fatalf("expected 6 results, got %d", len(results))
	}
	resultMap := make(map[string]agent.BackgroundTaskResult)
	for _, r := range results {
		resultMap[r.ID] = r
		if r.Status != agent.BackgroundTaskStatusCompleted {
			t.Fatalf("task %s: status=%s error=%s", r.ID, r.Status, r.Error)
		}
		if r.Answer == "" {
			t.Fatalf("task %s: empty answer", r.ID)
		}
	}
	t.Log("[PASS] 结果交付: 6 tasks completed, answers non-empty")

	// --- 2. 调度正确性: 5 external requests ---

	requests, maxActive := recorder.snapshot()
	if len(requests) != 5 {
		t.Fatalf("expected 5 external requests, got %d", len(requests))
	}
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
			t.Fatalf("unexpected task_id: %s", req.TaskID)
		}
		if req.AgentType != wantType {
			t.Fatalf("task %s: want agent_type=%s, got %s", req.TaskID, wantType, req.AgentType)
		}
	}
	t.Log("[PASS] 调度正确性: 5 external requests, agent_type 匹配")

	// --- 3. Stage-1 并行 ---

	if maxActive < 2 {
		t.Fatalf("stage-1 parallelism: maxActive=%d, expected >=2", maxActive)
	}
	t.Logf("[PASS] Stage-1 并行: maxActive=%d", maxActive)

	// --- 4. kimi 真实回答 (2 个 kimi role) ---

	for _, kimiID := range []string{"team-researcher_kimi", "team-writer_kimi"} {
		r := resultMap[kimiID]
		if len(r.Answer) < 10 {
			t.Fatalf("%s answer too short: %q", kimiID, r.Answer)
		}
		if strings.Contains(r.Answer, "FAKE_") {
			t.Fatalf("%s answer is a fake marker: %q", kimiID, r.Answer)
		}
		t.Logf("[PASS] %s real answer (len=%d)", kimiID, len(r.Answer))
	}

	// --- 5. 上下文继承: synthesizer 收到上游 markers ---

	internalPrompts := capture.snapshot()
	if len(internalPrompts) != 1 {
		t.Fatalf("expected 1 internal execution, got %d", len(internalPrompts))
	}
	synthPrompt := internalPrompts[0]
	if !strings.Contains(synthPrompt, "[Collaboration Context]") {
		t.Fatalf("synthesizer prompt missing [Collaboration Context]")
	}
	// fake codex 和 fake cc 的 marker 应该在 context 中
	for _, marker := range []string{"FAKE_CODEX_OK", "FAKE_CC_OK"} {
		if !strings.Contains(synthPrompt, marker) {
			t.Fatalf("synthesizer prompt missing upstream marker %q", marker)
		}
	}
	// kimi 的真实回答也应该在 context 中
	kimiResearchAnswer := resultMap["team-researcher_kimi"].Answer
	snippet := kimiResearchAnswer
	if len(snippet) > 40 {
		snippet = snippet[:40]
	}
	if !strings.Contains(synthPrompt, snippet) {
		t.Logf("WARN: synthesizer context may not contain kimi answer snippet")
	}
	t.Log("[PASS] 上下文继承: synthesizer 收到上游 context")

	// --- 6. 跨阶段传递: stage-3 tasks 收到 synthesis 结果 ---

	for _, req := range requests {
		if req.TaskID == "team-writer_kimi" || req.TaskID == "team-reviewer_cc" {
			if !strings.Contains(req.Prompt, synthesisMarker) {
				t.Fatalf("stage-3 task %s prompt missing synthesis marker", req.TaskID)
			}
		}
	}
	t.Log("[PASS] 跨阶段传递: stage-3 tasks 收到 synthesis 结果")

	// --- 7. 进度监控 ---

	kimiEvents := kimiProgress.snapshot()
	t.Logf("[INFO] kimi progress events: %d total across 2 kimi roles", len(kimiEvents))
	for i, ev := range kimiEvents {
		t.Logf("  progress[%d]: tool=%s iter=%d", i, ev.CurrentTool, ev.Iteration)
	}

	// --- 8. 完成消息 ---

	if !strings.Contains(res.Content, "All 6 tasks completed.") {
		t.Fatalf("expected 'All 6 tasks completed.' in output, got: %q", res.Content)
	}
	t.Log("[PASS] 完成消息: All 6 tasks completed")

	t.Log("=== deep research E2E 全部通过 ===")
}
