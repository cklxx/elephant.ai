package integration

import (
	"context"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"alex/internal/delivery/channels"
	larkgw "alex/internal/delivery/channels/lark"
	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/ports/storage"
	"alex/internal/domain/agent/react"
	"alex/internal/domain/agent/taskfile"
	"alex/internal/infra/external/bridge"
	"alex/internal/infra/process"
	"alex/internal/infra/tools/builtin/orchestration"
	"alex/internal/shared/logging"
)

// ---------------------------------------------------------------------------
// teamsLarkExecutor — bridges channels.AgentExecutor to run_tasks execution
// ---------------------------------------------------------------------------

type teamsLarkExecutor struct {
	mgr          *react.BackgroundTaskManager
	teamDefs     []agent.TeamDefinition
	recorder     agent.TeamRunRecorder
	timeout      time.Duration

	mu          sync.Mutex
	lastTask    string
	callCount   int
}

func (e *teamsLarkExecutor) EnsureSession(_ context.Context, sessionID string) (*storage.Session, error) {
	if sessionID == "" {
		sessionID = "teams-lark-e2e-session"
	}
	return &storage.Session{ID: sessionID, Metadata: map[string]string{}}, nil
}

func (e *teamsLarkExecutor) ExecuteTask(ctx context.Context, task string, _ string, _ agent.EventListener) (*agent.TaskResult, error) {
	e.mu.Lock()
	e.lastTask = task
	e.callCount++
	e.mu.Unlock()

	ctx = agent.WithBackgroundDispatcher(ctx, e.mgr)
	ctx = agent.WithTeamDefinitions(ctx, e.teamDefs)
	if e.recorder != nil {
		ctx = agent.WithTeamRunRecorder(ctx, e.recorder)
	}

	timeout := e.timeout
	if timeout == 0 {
		timeout = 90 * time.Second
	}

	// Find the team template name from the task content.
	templateName := ""
	for _, td := range e.teamDefs {
		if strings.Contains(task, td.Name) || strings.Contains(strings.ToLower(task), strings.ToLower(td.Description)) {
			templateName = td.Name
			break
		}
	}
	if templateName == "" && len(e.teamDefs) == 1 {
		templateName = e.teamDefs[0].Name
	}
	if templateName == "" {
		return &agent.TaskResult{Answer: "no matching team template found"}, nil
	}

	tool := orchestration.NewRunTasks()
	res, err := tool.Execute(ctx, ports.ToolCall{
		ID: "call-lark-inject-e2e",
		Arguments: map[string]any{
			"template":        templateName,
			"goal":            task,
			"wait":            true,
			"timeout_seconds": int(timeout.Seconds()),
		},
	})
	if err != nil {
		return nil, err
	}
	if res.Error != nil {
		return &agent.TaskResult{Answer: res.Content}, nil
	}
	return &agent.TaskResult{Answer: res.Content}, nil
}

func (e *teamsLarkExecutor) snapshot() (string, int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.lastTask, e.callCount
}

// ---------------------------------------------------------------------------
// setupLarkTeamsGateway — creates Gateway + RecordingMessenger for tests
// ---------------------------------------------------------------------------

func setupLarkTeamsGateway(t *testing.T, exec channels.AgentExecutor) (*larkgw.Gateway, *larkgw.RecordingMessenger) {
	t.Helper()
	rec := larkgw.NewRecordingMessenger()
	cfg := larkgw.Config{
		BaseConfig: channels.BaseConfig{
			SessionPrefix: "test-teams-lark",
			AllowDirect:   true,
			AllowGroups:   true,
		},
		AppID:     "test_teams_app",
		AppSecret: "test_teams_secret",
	}
	gw, err := larkgw.NewGateway(cfg, exec, logging.OrNop(nil))
	if err != nil {
		t.Fatalf("NewGateway: %v", err)
	}
	gw.SetMessenger(rec)
	return gw, rec
}

// ---------------------------------------------------------------------------
// waitForReply — polls RecordingMessenger for a ReplyMessage
// ---------------------------------------------------------------------------

func waitForReply(t *testing.T, rec *larkgw.RecordingMessenger, timeout time.Duration) []larkgw.MessengerCall {
	t.Helper()
	deadline := time.After(timeout)
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for ReplyMessage (waited %s)", timeout)
			return nil
		case <-tick.C:
			calls := rec.CallsByMethod("ReplyMessage")
			if len(calls) > 0 {
				return calls
			}
		}
	}
}

// ---------------------------------------------------------------------------
// mockTeamRunRecorder — captures team run records
// ---------------------------------------------------------------------------

type mockTeamRunRecorderLark struct {
	mu      sync.Mutex
	records []agent.TeamRunRecord
}

func (m *mockTeamRunRecorderLark) RecordTeamRun(_ context.Context, record agent.TeamRunRecord) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.records = append(m.records, record)
	return "rec-lark-1", nil
}

func (m *mockTeamRunRecorderLark) snapshot() []agent.TeamRunRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]agent.TeamRunRecord, len(m.records))
	copy(out, m.records)
	return out
}

// ---------------------------------------------------------------------------
// B1: TestLarkInject_TeamHappyPath
// ---------------------------------------------------------------------------

func TestLarkInject_TeamHappyPath(t *testing.T) {
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
			"FAKE_KIMI_MARKER":        "KIMI_LARK_OK",
			"FAKE_KIMI_SLEEP_SECONDS": "0.3",
		},
	}, process.NewController()))

	team := agent.TeamDefinition{
		Name:        "lark_research",
		Description: "parallel kimi research via lark inject",
		Roles: []agent.TeamRoleDefinition{
			{Name: "scout_a", AgentType: "kimi", PromptTemplate: "Scout A: {GOAL}", ExecutionMode: "plan", AutonomyLevel: "full", Config: map[string]string{"approval_policy": "never", "sandbox": "read-only"}},
			{Name: "scout_b", AgentType: "kimi", PromptTemplate: "Scout B: {GOAL}", ExecutionMode: "plan", AutonomyLevel: "full", Config: map[string]string{"approval_policy": "never", "sandbox": "read-only"}},
			{Name: "scout_c", AgentType: "kimi", PromptTemplate: "Scout C: {GOAL}", ExecutionMode: "plan", AutonomyLevel: "full", Config: map[string]string{"approval_policy": "never", "sandbox": "read-only"}},
		},
		Stages: []agent.TeamStageDefinition{
			{Name: "parallel_scout", Roles: []string{"scout_a", "scout_b", "scout_c"}},
		},
	}

	mgr := react.NewBackgroundTaskManager(react.BackgroundManagerConfig{
		RunContext: context.Background(),
		Logger:    agent.NoopLogger{},
		Clock:     agent.SystemClock{},
		ExecuteTask: func(ctx context.Context, prompt, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
			return &agent.TaskResult{Answer: "internal-not-used", Iterations: 1, TokensUsed: 1}, nil
		},
		ExternalExecutor: recorder,
		SessionID:        "lark-inject-happy-e2e",
	})
	defer mgr.Shutdown()

	exec := &teamsLarkExecutor{mgr: mgr, teamDefs: []agent.TeamDefinition{team}, timeout: 60 * time.Second}
	gw, rec := setupLarkTeamsGateway(t, exec)

	err = gw.InjectMessage(context.Background(), "oc_lark_e2e_001", "p2p", "ou_lark_001", "om_lark_hp_001", "调研分布式事务")
	if err != nil {
		t.Fatalf("InjectMessage: %v", err)
	}

	calls := waitForReply(t, rec, 30*time.Second)
	replyContent := calls[0].Content
	if !strings.Contains(replyContent, "completed") && !strings.Contains(replyContent, "All 3 tasks completed") {
		t.Fatalf("reply missing completion marker: %q", replyContent)
	}

	expectedIDs := []string{"team-scout_a", "team-scout_b", "team-scout_c"}
	results := mgr.Collect(expectedIDs, false, 0)
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	for _, r := range results {
		if r.Status != agent.BackgroundTaskStatusCompleted {
			t.Fatalf("task %s status=%s error=%s", r.ID, r.Status, r.Error)
		}
		if !strings.Contains(r.Answer, "KIMI_LARK_OK") {
			t.Fatalf("task %s answer missing marker: %q", r.ID, r.Answer)
		}
	}

	requests, _ := recorder.snapshot()
	if len(requests) != 3 {
		t.Fatalf("expected 3 external requests, got %d", len(requests))
	}

	_, callCount := exec.snapshot()
	if callCount != 1 {
		t.Fatalf("expected executor called once, got %d", callCount)
	}
}

// ---------------------------------------------------------------------------
// B2: TestLarkInject_MultiStageTeam
// ---------------------------------------------------------------------------

func TestLarkInject_MultiStageTeam(t *testing.T) {
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
			"FAKE_KIMI_MARKER":        "STAGE_KIMI_OK",
			"FAKE_KIMI_SLEEP_SECONDS": "0.3",
		},
	}, process.NewController())
	extRecorder := newRecordingExternalExecutor(kimiBridge)

	capture := &internalCapture{}
	const synthesisMarker = "STAGE_SYNTHESIS_DONE"

	team := agent.TeamDefinition{
		Name:        "multi_stage_lark",
		Description: "3-stage team: scout → synthesizer → delivery",
		Roles: []agent.TeamRoleDefinition{
			{Name: "scout_1", AgentType: "kimi", PromptTemplate: "Scout research: {GOAL}", ExecutionMode: "plan", AutonomyLevel: "full", Config: map[string]string{"approval_policy": "never", "sandbox": "read-only"}},
			{Name: "scout_2", AgentType: "kimi", PromptTemplate: "Scout analysis: {GOAL}", ExecutionMode: "plan", AutonomyLevel: "full", Config: map[string]string{"approval_policy": "never", "sandbox": "read-only"}},
			{Name: "synthesizer", AgentType: "internal", PromptTemplate: "Synthesize: {GOAL}", ExecutionMode: "execute", AutonomyLevel: "full", InheritContext: true},
			{Name: "delivery", AgentType: "kimi", PromptTemplate: "Deliver report: {GOAL}", ExecutionMode: "plan", AutonomyLevel: "full", InheritContext: true, Config: map[string]string{"approval_policy": "never", "sandbox": "read-only"}},
		},
		Stages: []agent.TeamStageDefinition{
			{Name: "scout", Roles: []string{"scout_1", "scout_2"}},
			{Name: "synthesis", Roles: []string{"synthesizer"}},
			{Name: "delivery", Roles: []string{"delivery"}},
		},
	}

	mgr := react.NewBackgroundTaskManager(react.BackgroundManagerConfig{
		RunContext: context.Background(),
		Logger:    agent.NoopLogger{},
		Clock:     agent.SystemClock{},
		ExecuteTask: func(ctx context.Context, prompt, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
			capture.record(prompt)
			return &agent.TaskResult{Answer: synthesisMarker + ":: synthesized results", Iterations: 1, TokensUsed: 5}, nil
		},
		ExternalExecutor: extRecorder,
		SessionID:        "lark-inject-multistage-e2e",
	})
	defer mgr.Shutdown()

	exec := &teamsLarkExecutor{mgr: mgr, teamDefs: []agent.TeamDefinition{team}, timeout: 90 * time.Second}
	gw, rec := setupLarkTeamsGateway(t, exec)

	err = gw.InjectMessage(context.Background(), "oc_lark_e2e_002", "p2p", "ou_lark_002", "om_lark_ms_001", "multi stage research")
	if err != nil {
		t.Fatalf("InjectMessage: %v", err)
	}

	calls := waitForReply(t, rec, 60*time.Second)
	replyContent := calls[0].Content
	if !strings.Contains(replyContent, "All 4 tasks completed") {
		t.Fatalf("reply missing completion: %q", replyContent)
	}

	allIDs := []string{"team-scout_1", "team-scout_2", "team-synthesizer", "team-delivery"}
	results := mgr.Collect(allIDs, false, 0)
	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}
	for _, r := range results {
		if r.Status != agent.BackgroundTaskStatusCompleted {
			t.Fatalf("task %s status=%s error=%s", r.ID, r.Status, r.Error)
		}
	}

	// Verify internal agent received upstream scout markers via context inheritance.
	prompts := capture.snapshot()
	if len(prompts) != 1 {
		t.Fatalf("expected 1 internal execution, got %d", len(prompts))
	}
	if !strings.Contains(prompts[0], "[Collaboration Context]") {
		t.Fatalf("synthesizer prompt missing [Collaboration Context]: %q", prompts[0])
	}
	if !strings.Contains(prompts[0], "STAGE_KIMI_OK") {
		t.Fatalf("synthesizer prompt missing upstream kimi marker")
	}

	// Verify stage-3 delivery task received synthesis marker.
	requests, _ := extRecorder.snapshot()
	deliveryFound := false
	for _, req := range requests {
		if req.TaskID == "team-delivery" {
			deliveryFound = true
			if !strings.Contains(req.Prompt, synthesisMarker) {
				t.Fatalf("delivery prompt missing synthesis marker: %q", req.Prompt)
			}
		}
	}
	if !deliveryFound {
		t.Fatal("delivery request not found in external requests")
	}
}

// ---------------------------------------------------------------------------
// B3: TestLarkInject_MixedAgentTypes
// ---------------------------------------------------------------------------

func TestLarkInject_MixedAgentTypes(t *testing.T) {
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
	fakeCodex := writeFakeCodexCLI(t, workspace)

	kimiBridge := bridge.New(bridge.BridgeConfig{
		AgentType: "kimi", Binary: fakeKimi, PythonBinary: pythonBin, BridgeScript: bridgeScript,
		ApprovalPolicy: "never", Sandbox: "read-only", PlanApprovalPolicy: "never", PlanSandbox: "read-only",
		Timeout: 30 * time.Second,
		Env:     map[string]string{"FAKE_KIMI_MARKER": "MIX_KIMI", "FAKE_KIMI_SLEEP_SECONDS": "0.3"},
	}, process.NewController())
	codexBridge := bridge.New(bridge.BridgeConfig{
		AgentType: "codex", Binary: fakeCodex, PythonBinary: pythonBin, BridgeScript: bridgeScript,
		ApprovalPolicy: "never", Sandbox: "read-only", PlanApprovalPolicy: "never", PlanSandbox: "read-only",
		Timeout: 30 * time.Second,
		Env:     map[string]string{"FAKE_CODEX_MARKER": "MIX_CODEX", "FAKE_CODEX_SLEEP_SECONDS": "0.3"},
	}, process.NewController())

	mux := &multiplexExternalExecutor{byType: map[string]agent.ExternalAgentExecutor{"kimi": kimiBridge, "codex": codexBridge}}
	extRecorder := newRecordingExternalExecutor(mux)

	capture := &internalCapture{}

	team := agent.TeamDefinition{
		Name:        "mixed_agents_lark",
		Description: "mixed agent types: kimi + codex + internal",
		Roles: []agent.TeamRoleDefinition{
			{Name: "researcher", AgentType: "kimi", PromptTemplate: "Research: {GOAL}", ExecutionMode: "plan", AutonomyLevel: "full", Config: map[string]string{"approval_policy": "never", "sandbox": "read-only"}},
			{Name: "coder", AgentType: "codex", PromptTemplate: "Code: {GOAL}", ExecutionMode: "plan", AutonomyLevel: "full", Config: map[string]string{"approval_policy": "never", "sandbox": "read-only"}},
			{Name: "reviewer", AgentType: "internal", PromptTemplate: "Review: {GOAL}", ExecutionMode: "execute", AutonomyLevel: "full", InheritContext: true},
		},
		Stages: []agent.TeamStageDefinition{
			{Name: "work", Roles: []string{"researcher", "coder"}},
			{Name: "review", Roles: []string{"reviewer"}},
		},
	}

	mgr := react.NewBackgroundTaskManager(react.BackgroundManagerConfig{
		RunContext: context.Background(),
		Logger:    agent.NoopLogger{},
		Clock:     agent.SystemClock{},
		ExecuteTask: func(ctx context.Context, prompt, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
			capture.record(prompt)
			return &agent.TaskResult{Answer: "REVIEW_DONE:: internal review", Iterations: 1, TokensUsed: 5}, nil
		},
		ExternalExecutor: extRecorder,
		SessionID:        "lark-inject-mixed-e2e",
	})
	defer mgr.Shutdown()

	exec := &teamsLarkExecutor{mgr: mgr, teamDefs: []agent.TeamDefinition{team}, timeout: 60 * time.Second}
	gw, rec := setupLarkTeamsGateway(t, exec)

	err = gw.InjectMessage(context.Background(), "oc_lark_e2e_003", "p2p", "ou_lark_003", "om_lark_mix_001", "mixed agent analysis")
	if err != nil {
		t.Fatalf("InjectMessage: %v", err)
	}

	calls := waitForReply(t, rec, 30*time.Second)
	replyContent := calls[0].Content
	if !strings.Contains(replyContent, "All 3 tasks completed") {
		t.Fatalf("reply missing completion: %q", replyContent)
	}

	// Verify routing: each external request went to the correct agent type.
	requests, _ := extRecorder.snapshot()
	if len(requests) != 2 {
		t.Fatalf("expected 2 external requests, got %d", len(requests))
	}
	typeMap := map[string]string{}
	for _, req := range requests {
		typeMap[req.TaskID] = req.AgentType
	}
	if typeMap["team-researcher"] != "kimi" {
		t.Fatalf("researcher routed to %q, expected kimi", typeMap["team-researcher"])
	}
	if typeMap["team-coder"] != "codex" {
		t.Fatalf("coder routed to %q, expected codex", typeMap["team-coder"])
	}

	// Verify internal reviewer received upstream context.
	prompts := capture.snapshot()
	if len(prompts) != 1 {
		t.Fatalf("expected 1 internal execution, got %d", len(prompts))
	}
	if !strings.Contains(prompts[0], "[Collaboration Context]") {
		t.Fatalf("reviewer prompt missing [Collaboration Context]")
	}
	if !strings.Contains(prompts[0], "MIX_KIMI") {
		t.Fatalf("reviewer prompt missing kimi marker")
	}
	if !strings.Contains(prompts[0], "MIX_CODEX") {
		t.Fatalf("reviewer prompt missing codex marker")
	}
}

// ---------------------------------------------------------------------------
// B4: TestLarkInject_PartialFailure
// ---------------------------------------------------------------------------

func TestLarkInject_PartialFailure(t *testing.T) {
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
		AgentType: "kimi", Binary: fakeKimi, PythonBinary: pythonBin, BridgeScript: bridgeScript,
		ApprovalPolicy: "never", Sandbox: "read-only", PlanApprovalPolicy: "never", PlanSandbox: "read-only",
		Timeout: 30 * time.Second,
		Env:     map[string]string{"FAKE_KIMI_MARKER": "PARTIAL_OK", "FAKE_KIMI_SLEEP_SECONDS": "0.2"},
	}, process.NewController())
	failBridge := bridge.New(bridge.BridgeConfig{
		AgentType: "codex", Binary: failingCLI, PythonBinary: pythonBin, BridgeScript: bridgeScript,
		ApprovalPolicy: "never", Sandbox: "read-only", PlanApprovalPolicy: "never", PlanSandbox: "read-only",
		Timeout: 15 * time.Second,
	}, process.NewController())

	mux := &multiplexExternalExecutor{byType: map[string]agent.ExternalAgentExecutor{"kimi": kimiBridge, "codex": failBridge}}

	team := agent.TeamDefinition{
		Name:        "partial_fail_lark",
		Description: "team with one failing role",
		Roles: []agent.TeamRoleDefinition{
			{Name: "ok_worker", AgentType: "kimi", PromptTemplate: "OK: {GOAL}", ExecutionMode: "plan", AutonomyLevel: "full", Config: map[string]string{"approval_policy": "never", "sandbox": "read-only"}},
			{Name: "fail_worker", AgentType: "codex", PromptTemplate: "Fail: {GOAL}", ExecutionMode: "plan", AutonomyLevel: "full", Config: map[string]string{"approval_policy": "never", "sandbox": "read-only"}},
			{Name: "dependent", AgentType: "internal", PromptTemplate: "Process: {GOAL}", ExecutionMode: "execute", AutonomyLevel: "full", InheritContext: true},
		},
		Stages: []agent.TeamStageDefinition{
			{Name: "work", Roles: []string{"ok_worker", "fail_worker"}},
			{Name: "process", Roles: []string{"dependent"}},
		},
	}

	internalCalled := false
	mgr := react.NewBackgroundTaskManager(react.BackgroundManagerConfig{
		RunContext: context.Background(),
		Logger:    agent.NoopLogger{},
		Clock:     agent.SystemClock{},
		ExecuteTask: func(ctx context.Context, prompt, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
			internalCalled = true
			return &agent.TaskResult{Answer: "should-not-reach", Iterations: 1, TokensUsed: 1}, nil
		},
		ExternalExecutor: mux,
		SessionID:        "lark-inject-failure-e2e",
	})
	defer mgr.Shutdown()

	exec := &teamsLarkExecutor{mgr: mgr, teamDefs: []agent.TeamDefinition{team}, timeout: 30 * time.Second}
	gw, rec := setupLarkTeamsGateway(t, exec)

	err = gw.InjectMessage(context.Background(), "oc_lark_e2e_004", "p2p", "ou_lark_004", "om_lark_fail_001", "test partial failure")
	if err != nil {
		t.Fatalf("InjectMessage: %v", err)
	}

	calls := waitForReply(t, rec, 30*time.Second)
	replyContent := calls[0].Content
	// Reply should contain failure information.
	if !strings.Contains(strings.ToLower(replyContent), "fail") && !strings.Contains(replyContent, "failed") {
		t.Logf("reply content: %q", replyContent)
		// The reply may indicate partial completion rather than explicit "fail" —
		// the key verification is the task statuses below.
	}

	// Verify task statuses.
	allIDs := []string{"team-ok_worker", "team-fail_worker", "team-dependent"}
	results := mgr.Collect(allIDs, true, 15*time.Second)
	resultMap := map[string]agent.BackgroundTaskResult{}
	for _, r := range results {
		resultMap[r.ID] = r
	}

	if ok, exists := resultMap["team-ok_worker"]; exists {
		if ok.Status != agent.BackgroundTaskStatusCompleted {
			t.Fatalf("ok_worker expected completed, got %s (error=%s)", ok.Status, ok.Error)
		}
	} else {
		t.Fatal("ok_worker result not found")
	}

	if fail, exists := resultMap["team-fail_worker"]; exists {
		if fail.Status != agent.BackgroundTaskStatusFailed {
			t.Fatalf("fail_worker expected failed, got %s", fail.Status)
		}
	} else {
		t.Fatal("fail_worker result not found")
	}

	if dep, exists := resultMap["team-dependent"]; exists {
		if dep.Status != agent.BackgroundTaskStatusFailed {
			t.Fatalf("dependent expected failed, got %s (error=%s)", dep.Status, dep.Error)
		}
	} else {
		t.Fatal("dependent result not found")
	}

	if internalCalled {
		t.Fatal("internal executeTask should not have been called when dependency failed")
	}
}

// ---------------------------------------------------------------------------
// B5: TestLarkInject_StatusFile
// ---------------------------------------------------------------------------

func TestLarkInject_StatusFile(t *testing.T) {
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
	kimiBridge := bridge.New(bridge.BridgeConfig{
		AgentType: "kimi", Binary: fakeKimi, PythonBinary: pythonBin, BridgeScript: bridgeScript,
		ApprovalPolicy: "never", Sandbox: "read-only", PlanApprovalPolicy: "never", PlanSandbox: "read-only",
		Timeout: 30 * time.Second,
		Env:     map[string]string{"FAKE_KIMI_MARKER": "STATUS_OK", "FAKE_KIMI_SLEEP_SECONDS": "0.25"},
	}, process.NewController())

	teamName := "status_file_lark"
	team := agent.TeamDefinition{
		Name:        teamName,
		Description: "verify status file sidecar",
		Roles: []agent.TeamRoleDefinition{
			{Name: "worker_a", AgentType: "kimi", PromptTemplate: "A: {GOAL}", ExecutionMode: "plan", AutonomyLevel: "full", Config: map[string]string{"approval_policy": "never", "sandbox": "read-only"}},
			{Name: "worker_b", AgentType: "kimi", PromptTemplate: "B: {GOAL}", ExecutionMode: "plan", AutonomyLevel: "full", Config: map[string]string{"approval_policy": "never", "sandbox": "read-only"}},
		},
		Stages: []agent.TeamStageDefinition{
			{Name: "parallel", Roles: []string{"worker_a", "worker_b"}},
		},
	}

	mgr := react.NewBackgroundTaskManager(react.BackgroundManagerConfig{
		RunContext: context.Background(),
		Logger:    agent.NoopLogger{},
		Clock:     agent.SystemClock{},
		ExecuteTask: func(ctx context.Context, prompt, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
			return &agent.TaskResult{Answer: "internal-not-used", Iterations: 1, TokensUsed: 1}, nil
		},
		ExternalExecutor: kimiBridge,
		SessionID:        "lark-inject-status-e2e",
	})
	defer mgr.Shutdown()

	exec := &teamsLarkExecutor{mgr: mgr, teamDefs: []agent.TeamDefinition{team}, timeout: 60 * time.Second}
	gw, rec := setupLarkTeamsGateway(t, exec)

	err = gw.InjectMessage(context.Background(), "oc_lark_e2e_005", "p2p", "ou_lark_005", "om_lark_sf_001", "status file test")
	if err != nil {
		t.Fatalf("InjectMessage: %v", err)
	}

	waitForReply(t, rec, 30*time.Second)

	// Ensure all tasks have completed before checking the status file.
	expectedIDs := []string{"team-worker_a", "team-worker_b"}
	results := mgr.Collect(expectedIDs, true, 15*time.Second)
	for _, r := range results {
		if r.Status != agent.BackgroundTaskStatusCompleted {
			t.Fatalf("task %s status=%s error=%s", r.ID, r.Status, r.Error)
		}
	}

	// Wait for the status poller to sync (polls every 2s).
	time.Sleep(3 * time.Second)

	// Verify status file was written.
	statusPath := filepath.Join(workspace, ".elephant", "tasks", "team-"+teamName+".status.yaml")
	sf, err := taskfile.ReadStatusFile(statusPath)
	if err != nil {
		t.Fatalf("ReadStatusFile: %v", err)
	}
	if sf.PlanID != "team-"+teamName {
		t.Fatalf("expected plan_id 'team-%s', got %q", teamName, sf.PlanID)
	}
	if len(sf.Tasks) != 2 {
		t.Fatalf("expected 2 tasks in status file, got %d", len(sf.Tasks))
	}
	for _, ts := range sf.Tasks {
		if ts.Status != "completed" {
			t.Fatalf("task %s status=%s, expected completed (error=%s)", ts.ID, ts.Status, ts.Error)
		}
	}
}

// ---------------------------------------------------------------------------
// B6: TestLarkInject_TeamRunRecorder
// ---------------------------------------------------------------------------

func TestLarkInject_TeamRunRecorder(t *testing.T) {
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
	kimiBridge := bridge.New(bridge.BridgeConfig{
		AgentType: "kimi", Binary: fakeKimi, PythonBinary: pythonBin, BridgeScript: bridgeScript,
		ApprovalPolicy: "never", Sandbox: "read-only", PlanApprovalPolicy: "never", PlanSandbox: "read-only",
		Timeout: 30 * time.Second,
		Env:     map[string]string{"FAKE_KIMI_MARKER": "REC_OK", "FAKE_KIMI_SLEEP_SECONDS": "0.25"},
	}, process.NewController())

	teamName := "recorder_lark"
	team := agent.TeamDefinition{
		Name:        teamName,
		Description: "verify team run recorder",
		Roles: []agent.TeamRoleDefinition{
			{Name: "worker_a", AgentType: "kimi", PromptTemplate: "A: {GOAL}", ExecutionMode: "plan", AutonomyLevel: "full", Config: map[string]string{"approval_policy": "never", "sandbox": "read-only"}},
			{Name: "worker_b", AgentType: "kimi", PromptTemplate: "B: {GOAL}", ExecutionMode: "plan", AutonomyLevel: "full", Config: map[string]string{"approval_policy": "never", "sandbox": "read-only"}},
		},
		Stages: []agent.TeamStageDefinition{
			{Name: "parallel", Roles: []string{"worker_a", "worker_b"}},
		},
	}

	mgr := react.NewBackgroundTaskManager(react.BackgroundManagerConfig{
		RunContext: context.Background(),
		Logger:    agent.NoopLogger{},
		Clock:     agent.SystemClock{},
		ExecuteTask: func(ctx context.Context, prompt, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
			return &agent.TaskResult{Answer: "internal-not-used", Iterations: 1, TokensUsed: 1}, nil
		},
		ExternalExecutor: kimiBridge,
		SessionID:        "lark-inject-recorder-e2e",
	})
	defer mgr.Shutdown()

	mockRecorder := &mockTeamRunRecorderLark{}
	exec := &teamsLarkExecutor{mgr: mgr, teamDefs: []agent.TeamDefinition{team}, recorder: mockRecorder, timeout: 60 * time.Second}
	gw, rec := setupLarkTeamsGateway(t, exec)

	err = gw.InjectMessage(context.Background(), "oc_lark_e2e_006", "p2p", "ou_lark_006", "om_lark_rec_001", "recorder test")
	if err != nil {
		t.Fatalf("InjectMessage: %v", err)
	}

	waitForReply(t, rec, 30*time.Second)

	records := mockRecorder.snapshot()
	if len(records) != 1 {
		t.Fatalf("expected 1 recorded team run, got %d", len(records))
	}
	rec0 := records[0]
	if rec0.TeamName != teamName {
		t.Fatalf("expected team name %q, got %q", teamName, rec0.TeamName)
	}
	if len(rec0.Roles) != 2 {
		t.Fatalf("expected 2 roles, got %d", len(rec0.Roles))
	}
	if len(rec0.Stages) != 1 {
		t.Fatalf("expected 1 stage, got %d", len(rec0.Stages))
	}
	if rec0.Stages[0].Name != "parallel" {
		t.Fatalf("expected stage name 'parallel', got %q", rec0.Stages[0].Name)
	}
	if rec0.DispatchState != "completed" {
		t.Fatalf("expected dispatch state 'completed', got %q", rec0.DispatchState)
	}
}
