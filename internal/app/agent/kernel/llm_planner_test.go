package kernel

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	core "alex/internal/domain/agent/ports"
	kerneldomain "alex/internal/domain/kernel"
	portsllm "alex/internal/domain/agent/ports/llm"
)

// ─────────────────────────────────────────────────────────────────────────────
// Mock LLM Factory + Client
// ─────────────────────────────────────────────────────────────────────────────

type mockPlannerClient struct {
	response *core.CompletionResponse
	err      error
	model    string
}

func (c *mockPlannerClient) Complete(_ context.Context, _ core.CompletionRequest) (*core.CompletionResponse, error) {
	return c.response, c.err
}

func (c *mockPlannerClient) Model() string { return c.model }

type mockPlannerFactory struct {
	client portsllm.LLMClient
	err    error
}

func (f *mockPlannerFactory) GetClient(_, _ string, _ portsllm.LLMConfig) (portsllm.LLMClient, error) {
	return f.client, f.err
}

func (f *mockPlannerFactory) GetIsolatedClient(_, _ string, _ portsllm.LLMConfig) (portsllm.LLMClient, error) {
	return f.client, f.err
}

func (f *mockPlannerFactory) DisableRetry() {}

// ─────────────────────────────────────────────────────────────────────────────
// parsePlanningDecisions tests
// ─────────────────────────────────────────────────────────────────────────────

func TestParsePlanningDecisions_ValidJSON(t *testing.T) {
	input := `[
		{"agent_id": "web-builder", "dispatch": true, "priority": 8, "prompt": "build website", "reason": "GOAL says so"},
		{"agent_id": "researcher", "dispatch": false, "priority": 3, "prompt": "search", "reason": "already done"}
	]`
	decisions, err := parsePlanningDecisions(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(decisions) != 2 {
		t.Fatalf("expected 2 decisions, got %d", len(decisions))
	}
	if decisions[0].AgentID != "web-builder" {
		t.Errorf("agent_id = %q, want %q", decisions[0].AgentID, "web-builder")
	}
	if !decisions[0].Dispatch {
		t.Error("decision[0].Dispatch should be true")
	}
	if decisions[1].Dispatch {
		t.Error("decision[1].Dispatch should be false")
	}
}

func TestParsePlanningDecisions_WithMarkdownFences(t *testing.T) {
	input := "Here's my plan:\n```json\n[{\"agent_id\": \"builder\", \"dispatch\": true, \"priority\": 5, \"prompt\": \"do it\", \"reason\": \"why not\"}]\n```\nDone."
	decisions, err := parsePlanningDecisions(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(decisions))
	}
	if decisions[0].AgentID != "builder" {
		t.Errorf("agent_id = %q, want %q", decisions[0].AgentID, "builder")
	}
}

func TestParsePlanningDecisions_WithPlainFences(t *testing.T) {
	input := "```\n[{\"agent_id\": \"x\", \"dispatch\": true, \"priority\": 1, \"prompt\": \"p\", \"reason\": \"r\"}]\n```"
	decisions, err := parsePlanningDecisions(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(decisions))
	}
}

func TestParsePlanningDecisions_EmptyArray(t *testing.T) {
	decisions, err := parsePlanningDecisions("[]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(decisions) != 0 {
		t.Fatalf("expected 0 decisions, got %d", len(decisions))
	}
}

func TestParsePlanningDecisions_NoJSON(t *testing.T) {
	_, err := parsePlanningDecisions("I don't know what to do")
	if err == nil {
		t.Fatal("expected error for non-JSON input")
	}
}

func TestParsePlanningDecisions_MalformedJSON(t *testing.T) {
	_, err := parsePlanningDecisions("[{invalid json}]")
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// LLMPlanner.toDispatchSpecs tests
// ─────────────────────────────────────────────────────────────────────────────

// testLLMPlanner creates a planner for unit testing with the logger properly initialized.
func testLLMPlanner(config LLMPlannerConfig, staticAgents []AgentConfig) *LLMPlanner {
	return NewLLMPlanner("test-kernel", &mockPlannerFactory{}, config, staticAgents, nil)
}

func TestLLMPlanner_ToDispatchSpecs_FiltersNonDispatch(t *testing.T) {
	p := testLLMPlanner(LLMPlannerConfig{MaxDispatches: 5}, nil)
	decisions := []planningDecision{
		{AgentID: "a", Dispatch: true, Priority: 8, Prompt: "do A", Reason: "needed"},
		{AgentID: "b", Dispatch: false, Priority: 5, Prompt: "skip", Reason: "done"},
		{AgentID: "c", Dispatch: true, Priority: 6, Prompt: "do C", Reason: "also needed"},
	}
	specs := p.toDispatchSpecs(decisions, "state", map[string]kerneldomain.Dispatch{})
	if len(specs) != 2 {
		t.Fatalf("expected 2 specs, got %d", len(specs))
	}
	if specs[0].AgentID != "a" || specs[1].AgentID != "c" {
		t.Errorf("unexpected agent IDs: %v, %v", specs[0].AgentID, specs[1].AgentID)
	}
}

func TestLLMPlanner_ToDispatchSpecs_RespectsMaxDispatches(t *testing.T) {
	p := testLLMPlanner(LLMPlannerConfig{MaxDispatches: 1}, nil)
	decisions := []planningDecision{
		{AgentID: "a", Dispatch: true, Priority: 8, Prompt: "do A", Reason: "r"},
		{AgentID: "b", Dispatch: true, Priority: 5, Prompt: "do B", Reason: "r"},
	}
	specs := p.toDispatchSpecs(decisions, "state", map[string]kerneldomain.Dispatch{})
	if len(specs) != 1 {
		t.Fatalf("expected 1 spec (max_dispatches=1), got %d", len(specs))
	}
	if specs[0].AgentID != "a" {
		t.Errorf("expected agent 'a', got %q", specs[0].AgentID)
	}
}

func TestLLMPlanner_ToDispatchSpecs_SkipsRunning(t *testing.T) {
	p := testLLMPlanner(LLMPlannerConfig{MaxDispatches: 5}, nil)
	decisions := []planningDecision{
		{AgentID: "running-agent", Dispatch: true, Priority: 8, Prompt: "redo", Reason: "r"},
		{AgentID: "idle-agent", Dispatch: true, Priority: 5, Prompt: "go", Reason: "r"},
	}
	recent := map[string]kerneldomain.Dispatch{
		"running-agent": {AgentID: "running-agent", Status: kerneldomain.DispatchRunning},
	}
	specs := p.toDispatchSpecs(decisions, "state", recent)
	if len(specs) != 1 {
		t.Fatalf("expected 1 spec (running skipped), got %d", len(specs))
	}
	if specs[0].AgentID != "idle-agent" {
		t.Errorf("expected 'idle-agent', got %q", specs[0].AgentID)
	}
}

func TestLLMPlanner_ToDispatchSpecs_FallsBackToStaticPrompt(t *testing.T) {
	p := testLLMPlanner(LLMPlannerConfig{MaxDispatches: 5}, []AgentConfig{
		{AgentID: "explorer", Prompt: "explore state: {STATE}", Enabled: true},
	})
	decisions := []planningDecision{
		{AgentID: "explorer", Dispatch: true, Priority: 5, Prompt: "", Reason: "use static prompt"},
	}
	specs := p.toDispatchSpecs(decisions, "my-state", map[string]kerneldomain.Dispatch{})
	if len(specs) != 1 {
		t.Fatalf("expected 1 spec, got %d", len(specs))
	}
	if specs[0].Prompt != "explore state: my-state" {
		t.Errorf("expected static prompt with STATE replaced, got %q", specs[0].Prompt)
	}
}

func TestLLMPlanner_ToDispatchSpecs_AdHocAgentFallbackPrompt(t *testing.T) {
	p := testLLMPlanner(LLMPlannerConfig{MaxDispatches: 5}, nil)
	decisions := []planningDecision{
		{AgentID: "new-agent", Dispatch: true, Priority: 7, Prompt: "", Reason: "build something"},
	}
	specs := p.toDispatchSpecs(decisions, "state-content", map[string]kerneldomain.Dispatch{})
	if len(specs) != 1 {
		t.Fatalf("expected 1 spec, got %d", len(specs))
	}
	if specs[0].Prompt == "" {
		t.Error("expected non-empty fallback prompt for ad-hoc agent")
	}
	// Should contain the reason and state.
	if !containsAll(specs[0].Prompt, "build something", "state-content") {
		t.Errorf("fallback prompt missing expected content: %q", specs[0].Prompt)
	}
}

func TestLLMPlanner_ToDispatchSpecs_DefaultPriority(t *testing.T) {
	p := testLLMPlanner(LLMPlannerConfig{MaxDispatches: 5}, nil)
	decisions := []planningDecision{
		{AgentID: "a", Dispatch: true, Priority: 0, Prompt: "go", Reason: "r"},
	}
	specs := p.toDispatchSpecs(decisions, "", map[string]kerneldomain.Dispatch{})
	if specs[0].Priority != 5 {
		t.Errorf("expected default priority 5, got %d", specs[0].Priority)
	}
}

func TestLLMPlanner_ToDispatchSpecs_MetadataSet(t *testing.T) {
	p := testLLMPlanner(LLMPlannerConfig{MaxDispatches: 5}, nil)
	decisions := []planningDecision{
		{AgentID: "a", Dispatch: true, Priority: 8, Prompt: "go", Reason: "important work"},
	}
	specs := p.toDispatchSpecs(decisions, "", map[string]kerneldomain.Dispatch{})
	if specs[0].Metadata["planner"] != "llm" {
		t.Errorf("expected metadata planner=llm, got %q", specs[0].Metadata["planner"])
	}
	if specs[0].Metadata["reason"] == "" {
		t.Error("expected non-empty reason in metadata")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// LLMPlanner.Plan integration tests (with mock LLM)
// ─────────────────────────────────────────────────────────────────────────────

func TestLLMPlanner_Plan_Success(t *testing.T) {
	client := &mockPlannerClient{
		response: &core.CompletionResponse{
			Content: `[{"agent_id": "website-builder", "dispatch": true, "priority": 9, "prompt": "build the site now", "reason": "GOAL requires it"}]`,
		},
		model: "gpt-4o-mini",
	}
	factory := &mockPlannerFactory{client: client}
	p := NewLLMPlanner("test-kernel", factory, LLMPlannerConfig{
		Provider:      "openai",
		Model:         "gpt-4o-mini",
		MaxDispatches: 3,
		Timeout:       10 * time.Second,
	}, nil, nil)

	specs, err := p.Plan(context.Background(), "state content", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("expected 1 spec, got %d", len(specs))
	}
	if specs[0].AgentID != "website-builder" {
		t.Errorf("agent_id = %q, want %q", specs[0].AgentID, "website-builder")
	}
	if specs[0].Priority != 9 {
		t.Errorf("priority = %d, want %d", specs[0].Priority, 9)
	}
}

func TestLLMPlanner_Plan_LLMError(t *testing.T) {
	factory := &mockPlannerFactory{client: nil, err: errors.New("provider down")}
	p := NewLLMPlanner("test-kernel", factory, LLMPlannerConfig{
		Provider: "openai",
		Model:    "gpt-4o-mini",
		Timeout:  5 * time.Second,
	}, nil, nil)

	_, err := p.Plan(context.Background(), "state", nil)
	if err == nil {
		t.Fatal("expected error when factory fails")
	}
}

func TestLLMPlanner_Plan_ParseError_ReturnsError(t *testing.T) {
	client := &mockPlannerClient{
		response: &core.CompletionResponse{Content: "I don't know what to do"},
		model:    "gpt-4o-mini",
	}
	factory := &mockPlannerFactory{client: client}
	p := NewLLMPlanner("test-kernel", factory, LLMPlannerConfig{
		Provider: "openai",
		Model:    "gpt-4o-mini",
		Timeout:  5 * time.Second,
	}, nil, nil)

	specs, err := p.Plan(context.Background(), "state", nil)
	if err == nil {
		t.Fatal("expected error on parse failure")
	}
	if specs != nil {
		t.Errorf("expected nil specs on parse failure, got %v", specs)
	}
}

func TestLLMPlanner_Plan_EmptyArrayResponse(t *testing.T) {
	client := &mockPlannerClient{
		response: &core.CompletionResponse{Content: "[]"},
		model:    "gpt-4o-mini",
	}
	factory := &mockPlannerFactory{client: client}
	p := NewLLMPlanner("test-kernel", factory, LLMPlannerConfig{
		Provider:      "openai",
		Model:         "gpt-4o-mini",
		MaxDispatches: 3,
		Timeout:       5 * time.Second,
	}, nil, nil)

	specs, err := p.Plan(context.Background(), "state", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(specs) != 0 {
		t.Errorf("expected 0 specs for empty array, got %d", len(specs))
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// HybridPlanner tests
// ─────────────────────────────────────────────────────────────────────────────

func TestHybridPlanner_UsesLLMWhenAvailable(t *testing.T) {
	client := &mockPlannerClient{
		response: &core.CompletionResponse{
			Content: `[{"agent_id": "dynamic-agent", "dispatch": true, "priority": 9, "prompt": "do stuff", "reason": "goal says"}]`,
		},
		model: "gpt-4o-mini",
	}
	factory := &mockPlannerFactory{client: client}

	staticPlanner := NewStaticPlanner("k", []AgentConfig{
		{AgentID: "static-agent", Prompt: "static {STATE}", Enabled: true, Priority: 5},
	})
	llmPlanner := NewLLMPlanner("k", factory, LLMPlannerConfig{
		Provider:      "openai",
		Model:         "gpt-4o-mini",
		MaxDispatches: 3,
		Timeout:       5 * time.Second,
	}, nil, nil)

	hybrid := NewHybridPlanner(staticPlanner, llmPlanner, nil)
	specs, err := hybrid.Plan(context.Background(), "state", map[string]kerneldomain.Dispatch{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("expected 1 LLM spec, got %d", len(specs))
	}
	if specs[0].AgentID != "dynamic-agent" {
		t.Errorf("expected LLM agent, got %q", specs[0].AgentID)
	}
}

func TestHybridPlanner_FallsBackOnLLMError(t *testing.T) {
	factory := &mockPlannerFactory{client: nil, err: errors.New("llm down")}

	staticPlanner := NewStaticPlanner("k", []AgentConfig{
		{AgentID: "static-agent", Prompt: "static {STATE}", Enabled: true, Priority: 5},
	})
	llmPlanner := NewLLMPlanner("k", factory, LLMPlannerConfig{
		Provider: "openai",
		Model:    "gpt-4o-mini",
		Timeout:  5 * time.Second,
	}, nil, nil)

	hybrid := NewHybridPlanner(staticPlanner, llmPlanner, nil)
	specs, err := hybrid.Plan(context.Background(), "my state", map[string]kerneldomain.Dispatch{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("expected 1 static fallback spec, got %d", len(specs))
	}
	if specs[0].AgentID != "static-agent" {
		t.Errorf("expected static agent fallback, got %q", specs[0].AgentID)
	}
}

func TestHybridPlanner_FallsBackOnEmptyLLMPlan(t *testing.T) {
	client := &mockPlannerClient{
		response: &core.CompletionResponse{Content: "[]"},
		model:    "gpt-4o-mini",
	}
	factory := &mockPlannerFactory{client: client}

	staticPlanner := NewStaticPlanner("k", []AgentConfig{
		{AgentID: "static-agent", Prompt: "static {STATE}", Enabled: true, Priority: 5},
	})
	llmPlanner := NewLLMPlanner("k", factory, LLMPlannerConfig{
		Provider:      "openai",
		Model:         "gpt-4o-mini",
		MaxDispatches: 3,
		Timeout:       5 * time.Second,
	}, nil, nil)

	hybrid := NewHybridPlanner(staticPlanner, llmPlanner, nil)
	specs, err := hybrid.Plan(context.Background(), "state", map[string]kerneldomain.Dispatch{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("expected 1 static fallback spec, got %d", len(specs))
	}
	if specs[0].AgentID != "static-agent" {
		t.Errorf("expected static agent, got %q", specs[0].AgentID)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// GOAL.md reading tests
// ─────────────────────────────────────────────────────────────────────────────

func TestLLMPlanner_ReadGoalFile_Exists(t *testing.T) {
	dir := t.TempDir()
	goalPath := filepath.Join(dir, "GOAL.md")
	if err := os.WriteFile(goalPath, []byte("# Build a website\n- MVP in 7 days"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := testLLMPlanner(LLMPlannerConfig{GoalFilePath: goalPath}, nil)
	content := p.readGoalFile()
	if content == "" {
		t.Fatal("expected non-empty goal content")
	}
	if !containsAll(content, "Build a website", "MVP") {
		t.Errorf("goal content missing expected text: %q", content)
	}
}

func TestLLMPlanner_ReadGoalFile_Missing(t *testing.T) {
	p := testLLMPlanner(LLMPlannerConfig{GoalFilePath: "/nonexistent/GOAL.md"}, nil)
	content := p.readGoalFile()
	if content != "" {
		t.Errorf("expected empty string for missing file, got %q", content)
	}
}

func TestLLMPlanner_ReadGoalFile_Empty(t *testing.T) {
	p := testLLMPlanner(LLMPlannerConfig{GoalFilePath: ""}, nil)
	content := p.readGoalFile()
	if content != "" {
		t.Errorf("expected empty string for empty path, got %q", content)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Engine.TriggerNow tests
// ─────────────────────────────────────────────────────────────────────────────

func TestEngine_TriggerNow_AcceptsFirstSignal(t *testing.T) {
	exec := &mockExecutor{}
	engine, _ := newTestEngine(t, exec)
	if !engine.TriggerNow() {
		t.Error("first TriggerNow should return true")
	}
}

func TestEngine_TriggerNow_SecondSignalRejected(t *testing.T) {
	exec := &mockExecutor{}
	engine, _ := newTestEngine(t, exec)
	engine.TriggerNow()
	if engine.TriggerNow() {
		t.Error("second TriggerNow should return false (already pending)")
	}
}

func TestEngine_TriggerNow_AcceptsAfterDrain(t *testing.T) {
	exec := &mockExecutor{}
	engine, _ := newTestEngine(t, exec)
	engine.TriggerNow()
	// Drain the channel manually.
	<-engine.triggerCh
	if !engine.TriggerNow() {
		t.Error("TriggerNow should accept after previous signal was consumed")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// buildPlanningPrompt tests
// ─────────────────────────────────────────────────────────────────────────────

func TestLLMPlanner_BuildPlanningPrompt_ContainsState(t *testing.T) {
	p := testLLMPlanner(LLMPlannerConfig{MaxDispatches: 3}, []AgentConfig{{AgentID: "research", Priority: 5, Enabled: true}})
	prompt := p.buildPlanningPrompt("# My State\nactive", "# My Goal\nbuild site", map[string]kerneldomain.Dispatch{
		"research": {AgentID: "research", Status: kerneldomain.DispatchDone, UpdatedAt: time.Now().Add(-10 * time.Minute)},
	})
	if !containsAll(prompt, "My State", "My Goal", "research", "done", "最多派发: 3") {
		t.Errorf("prompt missing expected sections: %q", prompt)
	}
}

func TestLLMPlanner_BuildPlanningPrompt_EmptyHistory(t *testing.T) {
	p := testLLMPlanner(LLMPlannerConfig{MaxDispatches: 5}, nil)
	prompt := p.buildPlanningPrompt("state", "", nil)
	if !containsAll(prompt, "无历史记录", "state") {
		t.Errorf("prompt missing expected content: %q", prompt)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

func containsAll(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if !contains(s, sub) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && searchString(s, sub)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
