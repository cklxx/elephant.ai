package kernel

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	core "alex/internal/domain/agent/ports"
	portsllm "alex/internal/domain/agent/ports/llm"
	kerneldomain "alex/internal/domain/kernel"
	runtimeconfig "alex/internal/shared/config"
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
	client      portsllm.LLMClient
	err         error
	onGetClient func(model string) // called with the requested model string
}

func (f *mockPlannerFactory) GetClient(_, model string, _ portsllm.LLMConfig) (portsllm.LLMClient, error) {
	if f.onGetClient != nil {
		f.onGetClient(model)
	}
	return f.client, f.err
}

func (f *mockPlannerFactory) GetIsolatedClient(_, model string, _ portsllm.LLMConfig) (portsllm.LLMClient, error) {
	if f.onGetClient != nil {
		f.onGetClient(model)
	}
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

func TestLLMPlanner_ToDispatchSpecs_RejectsPromptRequiringUserConfirmation_ExecutorPrompt(t *testing.T) {
	p := testLLMPlanner(LLMPlannerConfig{MaxDispatches: 5}, nil)
	decisions := []planningDecision{
		{AgentID: "a", Dispatch: true, Priority: 8, Prompt: "Before any tool action, call ask_user(action=\"clarify\", needs_user_input=true)", Reason: "blocked"},
	}
	specs := p.toDispatchSpecs(decisions, "state", map[string]kerneldomain.Dispatch{})
	if len(specs) != 0 {
		t.Fatalf("expected autonomy gate to reject confirmation-seeking prompt, got %d specs", len(specs))
	}
}

func TestLLMPlanner_ToDispatchSpecs_RejectsNoActionPrompt(t *testing.T) {
	p := testLLMPlanner(LLMPlannerConfig{MaxDispatches: 5}, nil)
	decisions := []planningDecision{
		{AgentID: "a", Dispatch: true, Priority: 8, Prompt: "Analysis only report; do not use tools and produce summary without tool action", Reason: "analysis"},
	}
	specs := p.toDispatchSpecs(decisions, "state", map[string]kerneldomain.Dispatch{})
	if len(specs) != 0 {
		t.Fatalf("expected autonomy gate to reject no-action prompt, got %d specs", len(specs))
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

func TestLLMPlanner_ToDispatchSpecs_SkipsPending(t *testing.T) {
	p := testLLMPlanner(LLMPlannerConfig{MaxDispatches: 5}, nil)
	decisions := []planningDecision{
		{AgentID: "pending-agent", Dispatch: true, Priority: 8, Prompt: "redo", Reason: "r"},
		{AgentID: "idle-agent", Dispatch: true, Priority: 5, Prompt: "go", Reason: "r"},
	}
	recent := map[string]kerneldomain.Dispatch{
		"pending-agent": {AgentID: "pending-agent", Status: kerneldomain.DispatchPending},
	}
	specs := p.toDispatchSpecs(decisions, "state", recent)
	if len(specs) != 1 {
		t.Fatalf("expected 1 spec (pending skipped), got %d", len(specs))
	}
	if specs[0].AgentID != "idle-agent" {
		t.Errorf("expected 'idle-agent', got %q", specs[0].AgentID)
	}
}

func TestLLMPlanner_ToDispatchSpecs_SkipsRunning_CaseInsensitiveRecentKey(t *testing.T) {
	p := testLLMPlanner(LLMPlannerConfig{MaxDispatches: 5}, nil)
	decisions := []planningDecision{
		{AgentID: "build-executor", Dispatch: true, Priority: 8, Prompt: "redo", Reason: "r"},
		{AgentID: "idle-agent", Dispatch: true, Priority: 5, Prompt: "go", Reason: "r"},
	}
	recent := map[string]kerneldomain.Dispatch{
		"Build-Executor": {AgentID: "Build-Executor", Status: kerneldomain.DispatchRunning},
	}
	specs := p.toDispatchSpecs(decisions, "state", recent)
	if len(specs) != 1 {
		t.Fatalf("expected 1 spec (running skipped via case-insensitive lookup), got %d", len(specs))
	}
	if specs[0].AgentID != "idle-agent" {
		t.Errorf("expected 'idle-agent', got %q", specs[0].AgentID)
	}
}

func TestLLMPlanner_ToDispatchSpecs_SkipsCooldown_CaseInsensitiveRecentKey(t *testing.T) {
	p := testLLMPlanner(LLMPlannerConfig{MaxDispatches: 5}, []AgentConfig{
		{AgentID: "build-executor", CooldownMinutes: 30, Enabled: true},
	})
	decisions := []planningDecision{{AgentID: "build-executor", Dispatch: true, Priority: 8, Prompt: "redo", Reason: "r"}}
	recent := map[string]kerneldomain.Dispatch{
		"Build-Executor": {
			AgentID:    "Build-Executor",
			Status:     kerneldomain.DispatchDone,
			UpdatedAt:  time.Now().Add(-5 * time.Minute),
			CreatedAt:  time.Now().Add(-6 * time.Minute),
		},
	}
	specs := p.toDispatchSpecs(decisions, "state", recent)
	if len(specs) != 0 {
		t.Fatalf("expected cooldown skip via case-insensitive lookup, got %d specs", len(specs))
	}
}

func TestLLMPlanner_ToDispatchSpecs_SkipsDuplicateAgentInSameCycle(t *testing.T) {
	p := testLLMPlanner(LLMPlannerConfig{MaxDispatches: 5}, nil)
	decisions := []planningDecision{
		{AgentID: "dup-agent", Dispatch: true, Priority: 8, Prompt: "first", Reason: "r1"},
		{AgentID: "dup-agent", Dispatch: true, Priority: 7, Prompt: "second", Reason: "r2"},
	}
	specs := p.toDispatchSpecs(decisions, "state", map[string]kerneldomain.Dispatch{})
	if len(specs) != 1 {
		t.Fatalf("expected 1 spec (duplicate skipped), got %d", len(specs))
	}
	if specs[0].Prompt != "first" {
		t.Fatalf("expected first decision to win, got prompt %q", specs[0].Prompt)
	}
}

func TestLLMPlanner_ToDispatchSpecs_RespectsAgentCooldown(t *testing.T) {
	p := testLLMPlanner(LLMPlannerConfig{MaxDispatches: 5}, []AgentConfig{
		{AgentID: "cool-agent", Enabled: true, CooldownMinutes: 30},
	})
	decisions := []planningDecision{
		{AgentID: "cool-agent", Dispatch: true, Priority: 8, Prompt: "redo", Reason: "r"},
	}
	recent := map[string]kerneldomain.Dispatch{
		"cool-agent": {
			AgentID:   "cool-agent",
			Status:    kerneldomain.DispatchDone,
			UpdatedAt: time.Now().Add(-5 * time.Minute),
		},
	}
	specs := p.toDispatchSpecs(decisions, "state", recent)
	if len(specs) != 0 {
		t.Fatalf("expected cooldown to skip dispatch, got %d spec(s)", len(specs))
	}
}

func TestLLMPlanner_ToDispatchSpecs_AllowsAfterCooldownWindow(t *testing.T) {
	p := testLLMPlanner(LLMPlannerConfig{MaxDispatches: 5}, []AgentConfig{
		{AgentID: "cool-agent", Enabled: true, CooldownMinutes: 10},
	})
	decisions := []planningDecision{
		{AgentID: "cool-agent", Dispatch: true, Priority: 8, Prompt: "redo", Reason: "r"},
	}
	recent := map[string]kerneldomain.Dispatch{
		"cool-agent": {
			AgentID:   "cool-agent",
			Status:    kerneldomain.DispatchDone,
			UpdatedAt: time.Now().Add(-15 * time.Minute),
		},
	}
	specs := p.toDispatchSpecs(decisions, "state", recent)
	if len(specs) != 1 {
		t.Fatalf("expected dispatch after cooldown expiry, got %d", len(specs))
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

func TestLLMPlanner_ToDispatchSpecs_TeamDispatchAllowed(t *testing.T) {
	p := testLLMPlanner(LLMPlannerConfig{
		MaxDispatches:        5,
		TeamDispatchEnabled:  true,
		MaxTeamsPerCycle:     1,
		TeamTimeoutSeconds:   180,
		AllowedTeamTemplates: []string{"kimi_research"},
	}, nil)
	decisions := []planningDecision{
		{
			Kind:         "team",
			AgentID:      "team:kimi_research",
			Dispatch:     true,
			Priority:     9,
			Reason:       "need multi-role synthesis",
			TeamTemplate: "kimi_research",
			TeamGoal:     "analyze distributed lock options",
		},
	}

	specs := p.toDispatchSpecs(decisions, "state", map[string]kerneldomain.Dispatch{})
	if len(specs) != 1 {
		t.Fatalf("expected 1 team spec, got %d", len(specs))
	}
	if specs[0].Kind != kerneldomain.DispatchKindTeam {
		t.Fatalf("expected team kind, got %q", specs[0].Kind)
	}
	if specs[0].Team == nil {
		t.Fatal("expected team payload")
	}
	if specs[0].Team.Template != "kimi_research" {
		t.Fatalf("unexpected template: %q", specs[0].Team.Template)
	}
	if specs[0].Team.TimeoutSeconds != 180 {
		t.Fatalf("unexpected timeout: %d", specs[0].Team.TimeoutSeconds)
	}
	if specs[0].Metadata["team_template"] != "kimi_research" {
		t.Fatalf("expected team template metadata, got %q", specs[0].Metadata["team_template"])
	}
}

func TestLLMPlanner_ToDispatchSpecs_TeamDispatchRejectedWhenTemplateNotAllowed(t *testing.T) {
	p := testLLMPlanner(LLMPlannerConfig{
		MaxDispatches:        5,
		TeamDispatchEnabled:  true,
		MaxTeamsPerCycle:     1,
		AllowedTeamTemplates: []string{"competitive_review"},
	}, nil)
	decisions := []planningDecision{
		{
			Kind:         "team",
			Dispatch:     true,
			Priority:     9,
			TeamTemplate: "kimi_research",
			TeamGoal:     "research topic",
		},
	}

	specs := p.toDispatchSpecs(decisions, "state", map[string]kerneldomain.Dispatch{})
	if len(specs) != 0 {
		t.Fatalf("expected 0 specs for non-allowed template, got %d", len(specs))
	}
}

func TestLLMPlanner_ToDispatchSpecs_MaxOneTeamPerCycle(t *testing.T) {
	p := testLLMPlanner(LLMPlannerConfig{
		MaxDispatches:        5,
		TeamDispatchEnabled:  true,
		MaxTeamsPerCycle:     1,
		AllowedTeamTemplates: []string{"team_a", "team_b"},
	}, nil)
	decisions := []planningDecision{
		{Kind: "team", Dispatch: true, TeamTemplate: "team_a", TeamGoal: "goal a", Priority: 8},
		{Kind: "team", Dispatch: true, TeamTemplate: "team_b", TeamGoal: "goal b", Priority: 8},
	}

	specs := p.toDispatchSpecs(decisions, "state", map[string]kerneldomain.Dispatch{})
	if len(specs) != 1 {
		t.Fatalf("expected exactly 1 team dispatch, got %d", len(specs))
	}
	if specs[0].Team == nil || specs[0].Team.Template != "team_a" {
		t.Fatalf("expected first team to be kept, got %#v", specs[0].Team)
	}
}

func TestLLMPlanner_ToDispatchSpecs_RejectsPromptRequiringUserConfirmation_BuildExecutorVariant(t *testing.T) {
	p := testLLMPlanner(LLMPlannerConfig{MaxDispatches: 5}, nil)
	decisions := []planningDecision{
		{AgentID: "build-executor", Dispatch: true, Priority: 8, Prompt: "Please ask_user(action=\"clarify\", needs_user_input=true) before proceeding", Reason: "needs human"},
	}

	specs := p.toDispatchSpecs(decisions, "state", map[string]kerneldomain.Dispatch{})
	if len(specs) != 0 {
		t.Fatalf("expected autonomy gate to reject prompt requiring user confirmation, got %d specs", len(specs))
	}
}

func TestLLMPlanner_ToDispatchSpecs_RejectsPromptWithNoConcreteToolAction(t *testing.T) {
	p := testLLMPlanner(LLMPlannerConfig{MaxDispatches: 5}, nil)
	decisions := []planningDecision{
		{AgentID: "build-executor", Dispatch: true, Priority: 8, Prompt: "analysis only; do not use tools in this cycle", Reason: "non-actionable"},
	}

	specs := p.toDispatchSpecs(decisions, "state", map[string]kerneldomain.Dispatch{})
	if len(specs) != 0 {
		t.Fatalf("expected autonomy gate to reject non-actionable prompt, got %d specs", len(specs))
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
		Profile:       runtimeconfig.LLMProfile{Provider: "openai", Model: "gpt-4o-mini"},
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
		Profile: runtimeconfig.LLMProfile{Provider: "openai", Model: "gpt-4o-mini"},
		Timeout: 5 * time.Second,
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
		Profile: runtimeconfig.LLMProfile{Provider: "openai", Model: "gpt-4o-mini"},
		Timeout: 5 * time.Second,
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
		Profile:       runtimeconfig.LLMProfile{Provider: "openai", Model: "gpt-4o-mini"},
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

func TestLLMPlanner_Plan_ProfileFuncOverridesStaticProfile(t *testing.T) {
	// ProfileFunc should be called on each Plan() invocation and its result
	// should take precedence over the static Profile field.
	callCount := 0
	dynamicModel := "dynamic-model-v2"
	profileFunc := func() runtimeconfig.LLMProfile {
		callCount++
		return runtimeconfig.LLMProfile{Provider: "openai", Model: dynamicModel}
	}

	var capturedModel string
	client := &mockPlannerClient{
		response: &core.CompletionResponse{Content: "[]"},
		model:    dynamicModel,
	}
	factory := &mockPlannerFactory{
		client: client,
		onGetClient: func(model string) {
			capturedModel = model
		},
	}
	p := NewLLMPlanner("test-kernel", factory, LLMPlannerConfig{
		Profile:       runtimeconfig.LLMProfile{Provider: "openai", Model: "static-model"},
		ProfileFunc:   profileFunc,
		MaxDispatches: 3,
		Timeout:       5 * time.Second,
	}, nil, nil)

	_, err := p.Plan(context.Background(), "state", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if callCount != 1 {
		t.Errorf("ProfileFunc called %d times, want 1", callCount)
	}
	// The dynamic profile model should win over the static one.
	if capturedModel != dynamicModel {
		t.Errorf("profile model = %q, want %q (dynamic override)", capturedModel, dynamicModel)
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
		Profile:       runtimeconfig.LLMProfile{Provider: "openai", Model: "gpt-4o-mini"},
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
		Profile: runtimeconfig.LLMProfile{Provider: "openai", Model: "gpt-4o-mini"},
		Timeout: 5 * time.Second,
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
		Profile:       runtimeconfig.LLMProfile{Provider: "openai", Model: "gpt-4o-mini"},
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
	content, _ := p.readGoalFile()
	if content == "" {
		t.Fatal("expected non-empty goal content")
	}
	if !containsAll(content, "Build a website", "MVP") {
		t.Errorf("goal content missing expected text: %q", content)
	}
}

func TestLLMPlanner_ReadGoalFile_Missing(t *testing.T) {
	p := testLLMPlanner(LLMPlannerConfig{GoalFilePath: "/nonexistent/GOAL.md"}, nil)
	content, _ := p.readGoalFile()
	if content != "" {
		t.Errorf("expected empty string for missing file, got %q", content)
	}
}

func TestLLMPlanner_ReadGoalFile_Empty(t *testing.T) {
	p := testLLMPlanner(LLMPlannerConfig{GoalFilePath: ""}, nil)
	content, _ := p.readGoalFile()
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
	prompt := p.buildPlanningPrompt("# My State\nactive", "# My Goal\nbuild site", "goal_context_loaded", map[string]kerneldomain.Dispatch{
		"research": {AgentID: "research", Status: kerneldomain.DispatchDone, UpdatedAt: time.Now().Add(-10 * time.Minute)},
	})
	if !containsAll(prompt, "My State", "My Goal", "research", "done", "Max dispatches this cycle: 3") {
		t.Errorf("prompt missing expected sections: %q", prompt)
	}
}

func TestLLMPlanner_BuildPlanningPrompt_EmptyHistory(t *testing.T) {
	p := testLLMPlanner(LLMPlannerConfig{MaxDispatches: 5}, nil)
	prompt := p.buildPlanningPrompt("state", "", "goal_context_not_configured", nil)
	if !containsAll(prompt, "no history", "state") {
		t.Errorf("prompt missing expected content: %q", prompt)
	}
}

func TestLLMPlanner_BuildPlanningPrompt_IncludesTeamConstraints(t *testing.T) {
	p := testLLMPlanner(LLMPlannerConfig{
		MaxDispatches:        5,
		TeamDispatchEnabled:  true,
		MaxTeamsPerCycle:     1,
		TeamTimeoutSeconds:   300,
		AllowedTeamTemplates: []string{"kimi_research"},
	}, nil)
	prompt := p.buildPlanningPrompt("state", "", "goal_context_not_configured", nil)
	if !containsAll(prompt, "Team Dispatch Constraints", "Max team dispatches this cycle: 1", "kimi_research") {
		t.Errorf("prompt missing team constraints: %q", prompt)
	}
}

func TestLLMPlanner_BuildPlanningPrompt_ShowsAwaitingInputAnnotation(t *testing.T) {
	p := testLLMPlanner(LLMPlannerConfig{MaxDispatches: 5}, nil)
	prompt := p.buildPlanningPrompt("state", "", "goal_context_not_configured", map[string]kerneldomain.Dispatch{
		"lark-poster": {
			AgentID:   "lark-poster",
			Status:    kerneldomain.DispatchFailed,
			Error:     "[awaiting_input] kernel dispatch completed while still awaiting user confirmation",
			UpdatedAt: time.Now().Add(-5 * time.Minute),
		},
	})
	if !containsAll(prompt, "lark-poster", "[awaiting_input]") {
		t.Errorf("prompt should contain [awaiting_input] annotation for failed dispatch, got: %q", prompt)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// compactStateForDispatch tests
// ─────────────────────────────────────────────────────────────────────────────

// realisticStateMD simulates a real STATE.md with identity, recent_actions (5 entries),
// and a full kernel_runtime block including cycle_history.
const realisticStateMD = `## identity
- name: alex
- kernel_id: production

## recent_actions
- 2026-03-02T10:00:00Z: deployed v2.1 to staging → success
- 2026-03-02T09:30:00Z: ran test suite → 142 passed, 0 failed
- 2026-03-02T09:00:00Z: code review for PR #45 → approved
- 2026-03-02T08:30:00Z: research on cache invalidation → wrote summary
- 2026-03-02T08:00:00Z: fixed login timeout bug → committed fix

## next_steps
- Deploy to production after staging soak

<!-- KERNEL_RUNTIME:START -->
## kernel_runtime
- updated_at: 2026-03-02T10:05:00Z
- latest_cycle_id: run-abc123
- latest_status: success
- latest_dispatched: 3
- latest_succeeded: 3
- latest_failed: 0
- latest_failed_agents: (none)
- latest_agent_summary: build-executor[done]: deployed v2.1 / research-executor[done]: cache analysis / audit-executor[done]: security scan clean
- latest_duration_ms: 45200
- latest_error: (none)

### cycle_history
| cycle_id | status | dispatched | succeeded | failed | summary | updated_at |
|----------|--------|------------|-----------|--------|---------|------------|
| run-abc123 | success | 3 | 3 | 0 | deployed + researched + audited | 2026-03-02T10:05:00Z |
| run-def456 | partial_success | 2 | 1 | 1 | build ok / outreach failed | 2026-03-02T09:35:00Z |
| run-ghi789 | success | 1 | 1 | 0 | test suite run | 2026-03-02T09:05:00Z |
<!-- KERNEL_RUNTIME:END -->
`

func TestCompactStateForDispatch_StripsRuntimeBlock(t *testing.T) {
	compact := compactStateForDispatch(realisticStateMD)
	if strings.Contains(compact, kernelRuntimeSectionStart) {
		t.Error("compact state should not contain KERNEL_RUNTIME:START marker")
	}
	if strings.Contains(compact, kernelRuntimeSectionEnd) {
		t.Error("compact state should not contain KERNEL_RUNTIME:END marker")
	}
	if strings.Contains(compact, "latest_cycle_id") {
		t.Error("compact state should not contain runtime stats")
	}
	if strings.Contains(compact, "cycle_history") {
		t.Error("compact state should not contain cycle_history table")
	}
}

func TestCompactStateForDispatch_PreservesIdentityAndNextSteps(t *testing.T) {
	compact := compactStateForDispatch(realisticStateMD)
	if !strings.Contains(compact, "## identity") {
		t.Error("compact state should preserve identity section")
	}
	if !strings.Contains(compact, "kernel_id: production") {
		t.Error("compact state should preserve identity content")
	}
	if !strings.Contains(compact, "## next_steps") {
		t.Error("compact state should preserve next_steps section")
	}
	if !strings.Contains(compact, "Deploy to production") {
		t.Error("compact state should preserve next_steps content")
	}
}

func TestCompactStateForDispatch_TruncatesRecentActions(t *testing.T) {
	compact := compactStateForDispatch(realisticStateMD)
	if !strings.Contains(compact, "## recent_actions") {
		t.Fatal("compact state should preserve recent_actions header")
	}
	// Should keep first 3 entries (most recent).
	if !strings.Contains(compact, "deployed v2.1") {
		t.Error("should keep 1st recent action")
	}
	if !strings.Contains(compact, "ran test suite") {
		t.Error("should keep 2nd recent action")
	}
	if !strings.Contains(compact, "code review") {
		t.Error("should keep 3rd recent action")
	}
	// Should drop entries 4 and 5.
	if strings.Contains(compact, "cache invalidation") {
		t.Error("should drop 4th recent action")
	}
	if strings.Contains(compact, "login timeout") {
		t.Error("should drop 5th recent action")
	}
}

func TestCompactStateForDispatch_SignificantSizeReduction(t *testing.T) {
	fullLen := len(realisticStateMD)
	compactLen := len(compactStateForDispatch(realisticStateMD))
	reduction := float64(fullLen-compactLen) / float64(fullLen) * 100
	t.Logf("STATE.md compaction: %d → %d chars (%.1f%% reduction)", fullLen, compactLen, reduction)
	if compactLen >= fullLen {
		t.Errorf("compact state (%d chars) should be smaller than full state (%d chars)", compactLen, fullLen)
	}
	// Expect at least 30% reduction from a realistic STATE with runtime block.
	if reduction < 30 {
		t.Errorf("expected at least 30%% reduction, got %.1f%%", reduction)
	}
}

func TestCompactStateForDispatch_NoRuntimeBlock(t *testing.T) {
	simple := "## identity\n- name: alex\n\n## recent_actions\n- did something\n"
	compact := compactStateForDispatch(simple)
	if compact != strings.TrimSpace(simple) {
		t.Errorf("state without runtime block should pass through cleanly, got %q", compact)
	}
}

func TestTruncateRecentActions_KeepsOnlyN(t *testing.T) {
	content := "## Recent_Actions\n- a\n- b\n- c\n- d\n- e\n\n## next\nstuff"
	result := truncateRecentActions(content, 2)
	if !strings.Contains(result, "- a") || !strings.Contains(result, "- b") {
		t.Error("should keep first 2 entries")
	}
	if strings.Contains(result, "- c") || strings.Contains(result, "- d") || strings.Contains(result, "- e") {
		t.Error("should drop entries beyond limit")
	}
	if !strings.Contains(result, "## next") {
		t.Error("should preserve subsequent sections")
	}
}

func TestTruncateRecentActions_NoSection(t *testing.T) {
	content := "## identity\n- name: alex\n"
	result := truncateRecentActions(content, 3)
	if result != content {
		t.Error("content without recent_actions should pass through unchanged")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// E2E: dispatch prompt uses compact STATE
// ─────────────────────────────────────────────────────────────────────────────

func TestLLMPlanner_ToDispatchSpecs_DispatchPromptUsesCompactState(t *testing.T) {
	p := testLLMPlanner(LLMPlannerConfig{MaxDispatches: 5}, []AgentConfig{
		{AgentID: "build-executor", Prompt: "Build task.\n\nState:\n{STATE}", Enabled: true, Priority: 8},
	})
	decisions := []planningDecision{
		{AgentID: "build-executor", Dispatch: true, Priority: 8, Prompt: "", Reason: "deploy to prod"},
	}
	specs := p.toDispatchSpecs(decisions, realisticStateMD, map[string]kerneldomain.Dispatch{})
	if len(specs) != 1 {
		t.Fatalf("expected 1 spec, got %d", len(specs))
	}
	prompt := specs[0].Prompt
	// Runtime block should be stripped from dispatch prompt.
	if strings.Contains(prompt, kernelRuntimeSectionStart) {
		t.Error("dispatch prompt should NOT contain KERNEL_RUNTIME markers")
	}
	if strings.Contains(prompt, "latest_cycle_id") {
		t.Error("dispatch prompt should NOT contain runtime stats")
	}
	// Identity and next_steps should be preserved.
	if !strings.Contains(prompt, "kernel_id: production") {
		t.Error("dispatch prompt should contain identity")
	}
	if !strings.Contains(prompt, "Deploy to production") {
		t.Error("dispatch prompt should contain next_steps")
	}
	// recent_actions should be truncated.
	if strings.Contains(prompt, "login timeout") {
		t.Error("dispatch prompt should NOT contain old recent_actions (5th entry)")
	}
	t.Logf("dispatch prompt length: %d chars (full STATE was %d chars)", len(prompt), len(realisticStateMD))
}

func TestLLMPlanner_ToDispatchSpecs_DynamicPromptUsesCompactState(t *testing.T) {
	p := testLLMPlanner(LLMPlannerConfig{MaxDispatches: 5}, nil)
	decisions := []planningDecision{
		{AgentID: "ad-hoc-agent", Dispatch: true, Priority: 7, Prompt: "Do work.\nContext: {STATE}", Reason: "ad hoc"},
	}
	specs := p.toDispatchSpecs(decisions, realisticStateMD, map[string]kerneldomain.Dispatch{})
	if len(specs) != 1 {
		t.Fatalf("expected 1 spec, got %d", len(specs))
	}
	prompt := specs[0].Prompt
	if strings.Contains(prompt, kernelRuntimeSectionStart) {
		t.Error("dynamic dispatch prompt should NOT contain runtime block")
	}
	if !strings.Contains(prompt, "kernel_id: production") {
		t.Error("dynamic dispatch prompt should contain identity")
	}
}

func TestLLMPlanner_ToDispatchSpecs_FallbackPromptUsesCompactState(t *testing.T) {
	p := testLLMPlanner(LLMPlannerConfig{MaxDispatches: 5}, nil)
	decisions := []planningDecision{
		{AgentID: "new-agent", Dispatch: true, Priority: 7, Prompt: "", Reason: "build something"},
	}
	specs := p.toDispatchSpecs(decisions, realisticStateMD, map[string]kerneldomain.Dispatch{})
	if len(specs) != 1 {
		t.Fatalf("expected 1 spec, got %d", len(specs))
	}
	prompt := specs[0].Prompt
	if strings.Contains(prompt, kernelRuntimeSectionStart) {
		t.Error("fallback prompt should NOT contain runtime block")
	}
	if !strings.Contains(prompt, "build something") {
		t.Error("fallback prompt should contain the reason")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// E2E: planner still sees full STATE
// ─────────────────────────────────────────────────────────────────────────────

func TestLLMPlanner_BuildPlanningPrompt_ContainsFullRuntimeBlock(t *testing.T) {
	p := testLLMPlanner(LLMPlannerConfig{MaxDispatches: 3}, nil)
	prompt := p.buildPlanningPrompt(realisticStateMD, "", "goal_context_not_configured", nil)
	if !strings.Contains(prompt, "latest_cycle_id") {
		t.Error("planner prompt should contain full runtime stats")
	}
	if !strings.Contains(prompt, "cycle_history") {
		t.Error("planner prompt should contain cycle_history table")
	}
	if !strings.Contains(prompt, "login timeout") {
		t.Error("planner prompt should contain ALL recent_actions (not truncated)")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// E2E: disabled agents filtered from planner
// ─────────────────────────────────────────────────────────────────────────────

func TestLLMPlanner_BuildPlanningPrompt_ExcludesDisabledAgents(t *testing.T) {
	p := testLLMPlanner(LLMPlannerConfig{MaxDispatches: 5}, []AgentConfig{
		{AgentID: "build-executor", Priority: 8, Enabled: true},
		{AgentID: "legacy-agent", Priority: 3, Enabled: false},
		{AgentID: "research-executor", Priority: 5, Enabled: true},
	})
	prompt := p.buildPlanningPrompt("state", "", "goal_context_not_configured", nil)
	if !strings.Contains(prompt, "build-executor") {
		t.Error("enabled agent should appear in planner prompt")
	}
	if !strings.Contains(prompt, "research-executor") {
		t.Error("enabled agent should appear in planner prompt")
	}
	if strings.Contains(prompt, "legacy-agent") {
		t.Error("disabled agent should NOT appear in planner prompt")
	}
	if strings.Contains(prompt, "disabled") {
		t.Error("planner prompt should not mention 'disabled' status at all")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// E2E: cycle_history compaction
// ─────────────────────────────────────────────────────────────────────────────

func TestBuildCycleHistoryEntry_CompactSummaryLimit(t *testing.T) {
	longSummary := strings.Repeat("word ", 50) // 250 chars
	result := &kerneldomain.CycleResult{
		CycleID: "run-test",
		Status:  kerneldomain.CycleSuccess,
		AgentSummary: []kerneldomain.AgentCycleSummary{
			{AgentID: "agent-a", Status: kerneldomain.DispatchDone, Summary: longSummary},
		},
	}
	entry := buildCycleHistoryEntry(result, nil, time.Now())
	// Summary in cycle_history should be compacted to 80 chars.
	if len(entry.Summary) > 100 {
		t.Errorf("cycle history summary too long (%d chars), expected compact ≤80+ellipsis: %q", len(entry.Summary), entry.Summary)
	}
	t.Logf("cycle history entry summary: %d chars", len(entry.Summary))
}

func TestRenderStateAgentSummary_CompactSummaryLimit(t *testing.T) {
	longSummary := strings.Repeat("detail ", 40) // 280 chars
	entries := []kerneldomain.AgentCycleSummary{
		{AgentID: "agent-x", Status: kerneldomain.DispatchDone, Summary: longSummary},
	}
	rendered := renderStateAgentSummary(entries)
	// Each agent summary within the runtime block should be ≤80 chars.
	// Format: "agent-x[done]: <summary>" — so total should be manageable.
	if len(rendered) > 120 {
		t.Errorf("agent summary too long (%d chars), expected compact: %q", len(rendered), rendered)
	}
	t.Logf("runtime agent summary: %d chars", len(rendered))
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
