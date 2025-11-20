package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"alex/internal/agent/ports"
)

func TestPrepareInjectsUserHistoryRecall(t *testing.T) {
	session := &ports.Session{
		ID: "session-history-1",
		Messages: []ports.Message{
			{
				Role:    "user",
				Source:  ports.MessageSourceUserInput,
				Content: "Draft a marketing plan for the Q2 launch and collect competitor insights.",
			},
			{
				Role:    "assistant",
				Source:  ports.MessageSourceAssistantReply,
				Content: "Recommended steps included outlining positioning, interviewing sales, and gathering benchmarks.",
			},
			{
				Role:    "user",
				Source:  ports.MessageSourceUserInput,
				Content: "Fix the database migration failure that blocks auth logins.",
			},
			{
				Role:    "assistant",
				Source:  ports.MessageSourceAssistantReply,
				Content: "Resetting the goose sequence resolved the migration issue.",
			},
		},
		Metadata: map[string]string{},
	}

	store := &stubSessionStore{session: session}
	registry := &registryWithList{defs: []ports.ToolDefinition{{Name: "web_search"}, {Name: "web_fetch"}}}

	deps := ExecutionPreparationDeps{
		LLMFactory:    &fakeLLMFactory{client: fakeLLMClient{}},
		ToolRegistry:  registry,
		SessionStore:  store,
		ContextMgr:    stubContextManager{},
		Parser:        stubParser{},
		Config:        Config{LLMProvider: "mock", LLMModel: "test", MaxIterations: 3},
		Logger:        ports.NoopLogger{},
		Clock:         ports.ClockFunc(func() time.Time { return time.Date(2024, time.June, 1, 10, 0, 0, 0, time.UTC) }),
		CostDecorator: NewCostTrackingDecorator(nil, ports.NoopLogger{}, ports.ClockFunc(time.Now)),
		EventEmitter:  ports.NoopEventListener{},
	}

	service := NewExecutionPreparationService(deps)
	env, err := service.Prepare(context.Background(), "Summarize the Q2 marketing strategy", session.ID)
	if err != nil {
		t.Fatalf("prepare execution failed: %v", err)
	}

	if env == nil || env.State == nil {
		t.Fatalf("expected environment with state")
	}

	historyMessages := collectHistoryMessages(env.State.Messages)
	if len(historyMessages) < 2 {
		t.Fatalf("expected both user and assistant turns to be recalled, got %d", len(historyMessages))
	}
	if !strings.EqualFold(historyMessages[0].Role, "user") {
		t.Fatalf("expected first recalled message to retain user role, got %q", historyMessages[0].Role)
	}
	if !strings.Contains(historyMessages[0].Content, "marketing plan") {
		t.Fatalf("expected recall message to mention marketing plan, got %q", historyMessages[0].Content)
	}
	if !strings.EqualFold(historyMessages[1].Role, "assistant") {
		t.Fatalf("expected assistant role for second recalled message, got %q", historyMessages[1].Role)
	}
	if !strings.Contains(historyMessages[1].Content, "Recommended steps") {
		t.Fatalf("expected assistant recall to include earlier reply, got %q", historyMessages[1].Content)
	}

}

func TestPrepareHistoryRecallOmitsSystemMessages(t *testing.T) {
	session := &ports.Session{
		ID: "session-history-2",
		Messages: []ports.Message{
			{Role: "system", Content: "You are a helpful assistant."},
			{
				Role:    "user",
				Source:  ports.MessageSourceUserInput,
				Content: "Document the legacy payroll process for auditing.",
			},
			{
				Role:    "assistant",
				Source:  ports.MessageSourceAssistantReply,
				Content: "Captured steps for payroll processing.",
			},
		},
		Metadata: map[string]string{},
	}
	store := &stubSessionStore{session: session}
	deps := ExecutionPreparationDeps{
		LLMFactory:    &fakeLLMFactory{client: fakeLLMClient{}},
		ToolRegistry:  &registryWithList{defs: []ports.ToolDefinition{{Name: "web_search"}}},
		SessionStore:  store,
		ContextMgr:    stubContextManager{},
		Parser:        stubParser{},
		Config:        Config{LLMProvider: "mock", LLMModel: "test", MaxIterations: 3},
		Logger:        ports.NoopLogger{},
		Clock:         ports.ClockFunc(func() time.Time { return time.Date(2024, time.June, 1, 10, 0, 0, 0, time.UTC) }),
		CostDecorator: NewCostTrackingDecorator(nil, ports.NoopLogger{}, ports.ClockFunc(time.Now)),
		EventEmitter:  ports.NoopEventListener{},
	}

	service := NewExecutionPreparationService(deps)
	env, err := service.Prepare(context.Background(), "Investigate flaky integration tests", session.ID)
	if err != nil {
		t.Fatalf("prepare execution failed: %v", err)
	}

	historyMessages := collectHistoryMessages(env.State.Messages)
	if len(historyMessages) != 2 {
		t.Fatalf("expected only non-system messages to be recalled, got %d", len(historyMessages))
	}
	for _, msg := range historyMessages {
		if strings.EqualFold(msg.Role, "system") {
			t.Fatalf("did not expect system messages in history recall: %q", msg.Content)
		}
	}
}

func TestPrepareHistoryRecallReplacesOriginalTurns(t *testing.T) {
	session := &ports.Session{
		ID: "session-history-duplicates",
		Messages: []ports.Message{
			{
				Role:    "user",
				Source:  ports.MessageSourceUserInput,
				Content: "Draft a marketing plan for the Q3 launch.",
			},
			{
				Role:    "assistant",
				Source:  ports.MessageSourceAssistantReply,
				Content: "Outlined launch phases and KPIs.",
			},
		},
		Metadata: map[string]string{},
	}
	store := &stubSessionStore{session: session}
	deps := ExecutionPreparationDeps{
		LLMFactory:   &fakeLLMFactory{client: fakeLLMClient{}},
		ToolRegistry: &registryWithList{},
		SessionStore: store,
		ContextMgr:   stubContextManager{},
		Parser:       stubParser{},
		Config:       Config{LLMProvider: "mock", LLMModel: "test", MaxIterations: 3},
		Logger:       ports.NoopLogger{},
		Clock:        ports.ClockFunc(func() time.Time { return time.Date(2024, time.June, 1, 10, 0, 0, 0, time.UTC) }),
		EventEmitter: ports.NoopEventListener{},
	}

	service := NewExecutionPreparationService(deps)
	env, err := service.Prepare(context.Background(), "Summarize marketing plan", session.ID)
	if err != nil {
		t.Fatalf("prepare execution failed: %v", err)
	}

	expected := map[string]int{
		"marketing plan":         0,
		"launch phases and KPIs": 0,
	}
	for _, msg := range env.State.Messages {
		for needle := range expected {
			if strings.Contains(msg.Content, needle) {
				if msg.Source != ports.MessageSourceUserHistory {
					t.Fatalf("expected recalled message %q to use user history source, got %s", needle, msg.Source)
				}
				expected[needle]++
			}
		}
	}
	for needle, count := range expected {
		if count != 1 {
			t.Fatalf("expected exactly one recalled entry for %q, got %d", needle, count)
		}
	}
}

func TestHistoryRecallSummarizesWhenThresholdExceeded(t *testing.T) {
	session := &ports.Session{
		ID: "session-history-3",
		Messages: []ports.Message{
			{
				Role:    "user",
				Source:  ports.MessageSourceUserInput,
				Content: "Capture marketing OKRs for the quarter.",
			},
			{
				Role:    "assistant",
				Source:  ports.MessageSourceAssistantReply,
				Content: "Outlined OKRs by channel and owner.",
			},
			{
				Role:    "user",
				Source:  ports.MessageSourceUserInput,
				Content: "Summarize marketing experiments focused on retention.",
			},
			{
				Role:    "assistant",
				Source:  ports.MessageSourceAssistantReply,
				Content: "Documented experiment matrix with hypotheses.",
			},
			{
				Role:    "user",
				Source:  ports.MessageSourceUserInput,
				Content: "Share marketing automation learnings from onboarding emails.",
			},
			{
				Role:    "assistant",
				Source:  ports.MessageSourceAssistantReply,
				Content: "Reported uplift metrics for triggered sequences.",
			},
		},
		Metadata: map[string]string{},
	}
	store := &stubSessionStore{session: session}
	deps := ExecutionPreparationDeps{
		LLMFactory:    &fakeLLMFactory{client: fakeLLMClient{}},
		ToolRegistry:  &registryWithList{},
		SessionStore:  store,
		ContextMgr:    stubContextManager{},
		Parser:        stubParser{},
		Config:        Config{LLMProvider: "mock", LLMModel: "test", MaxIterations: 3, MaxTokens: 20},
		Logger:        ports.NoopLogger{},
		Clock:         ports.ClockFunc(func() time.Time { return time.Date(2024, time.June, 1, 10, 0, 0, 0, time.UTC) }),
		CostDecorator: NewCostTrackingDecorator(nil, ports.NoopLogger{}, ports.ClockFunc(time.Now)),
		EventEmitter:  ports.NoopEventListener{},
	}

	service := NewExecutionPreparationService(deps)
	env, err := service.Prepare(context.Background(), "Summarize the marketing initiatives", session.ID)
	if err != nil {
		t.Fatalf("prepare execution failed: %v", err)
	}

	historyMessages := collectHistoryMessages(env.State.Messages)
	if len(historyMessages) != 1 {
		t.Fatalf("expected summarized history message, got %d entries", len(historyMessages))
	}
	if !strings.EqualFold(historyMessages[0].Role, "system") {
		t.Fatalf("expected summary to use system role, got %q", historyMessages[0].Role)
	}
	if trimmed := strings.TrimSpace(historyMessages[0].Content); trimmed != historySummaryResponse() {
		t.Fatalf("expected summary content to match fake LLM output, got %q", trimmed)
	}
}

type fakeLLMFactory struct {
	client ports.LLMClient
}

func (f *fakeLLMFactory) GetClient(provider, model string, config ports.LLMConfig) (ports.LLMClient, error) {
	if f.client == nil {
		return nil, errors.New("no client")
	}
	return f.client, nil
}

func (f *fakeLLMFactory) GetIsolatedClient(provider, model string, config ports.LLMConfig) (ports.LLMClient, error) {
	if f.client == nil {
		return nil, errors.New("no client")
	}
	return f.client, nil
}

func (f *fakeLLMFactory) DisableRetry() {}

type fakeLLMClient struct{}

func (fakeLLMClient) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	if req.Metadata != nil {
		if intent, ok := req.Metadata["intent"].(string); ok && intent == historySummaryIntent {
			return &ports.CompletionResponse{Content: historySummaryResponse()}, nil
		}
	}
	return &ports.CompletionResponse{Content: `<task_analysis>
  <action>Researching marketing landscape</action>
  <goal>Summarize current marketing trends</goal>
  <approach>Review internal assets then synthesize web insights</approach>
  <success_criteria>
    <criterion>List at least three trend themes</criterion>
    <criterion>Cite internal and external sources</criterion>
  </success_criteria>
  <task_breakdown>
    <step requires_external_research="false" requires_retrieval="true">
      <description>Review recent marketing briefs in the repo</description>
      <reason>Need latest internal positioning</reason>
    </step>
    <step requires_external_research="true" requires_retrieval="true">
      <description>Collect public reports on 2024 marketing trends</description>
      <reason>Fresh data lives outside the workspace</reason>
    </step>
  </task_breakdown>
  <retrieval_plan should_retrieve="true">
    <local_queries>
      <query>marketing brief</query>
      <query>campaign roadmap</query>
    </local_queries>
    <search_queries>
      <query>2024 marketing trends</query>
      <query>consumer engagement benchmarks</query>
    </search_queries>
    <crawl_urls>
      <url>https://example.com/report</url>
    </crawl_urls>
    <knowledge_gaps>
      <gap>Latest consumer engagement statistics</gap>
    </knowledge_gaps>
    <notes>Prioritize sources updated within the last quarter</notes>
  </retrieval_plan>
</task_analysis>`}, nil
}

func (fakeLLMClient) Model() string { return "stub-model" }

type registryWithList struct {
	defs []ports.ToolDefinition
}

func (r *registryWithList) Register(tool ports.ToolExecutor) error { return nil }

func (r *registryWithList) Get(name string) (ports.ToolExecutor, error) {
	return nil, errors.New("not implemented")
}

func (r *registryWithList) List() []ports.ToolDefinition {
	return append([]ports.ToolDefinition(nil), r.defs...)
}

func (r *registryWithList) Unregister(name string) error { return nil }

func (r *registryWithList) WithoutSubagent() ports.ToolRegistry {
	filtered := make([]ports.ToolDefinition, 0, len(r.defs))
	for _, def := range r.defs {
		if def.Name == "subagent" {
			continue
		}
		filtered = append(filtered, def)
	}
	return &registryWithList{defs: filtered}
}

func TestPrepareCarriesSessionHistoryIntoState(t *testing.T) {
	session := &ports.Session{
		ID: "session-history-merge",
		Messages: []ports.Message{
			{Role: "system", Content: "Legacy persona", Source: ports.MessageSourceSystemPrompt},
			{Role: "user", Content: "第一轮：分析日志", Source: ports.MessageSourceUserInput},
			{Role: "tool", Content: "log_parser: Found 2 errors", Source: ports.MessageSourceToolResult, ToolCallID: "logs-1"},
			{Role: "assistant", Content: "我会根据错误代码修复", Source: ports.MessageSourceAssistantReply},
		},
		Metadata: map[string]string{},
	}
	store := &stubSessionStore{session: session}
	deps := ExecutionPreparationDeps{
		LLMFactory:    &fakeLLMFactory{client: fakeLLMClient{}},
		ToolRegistry:  &registryWithList{defs: []ports.ToolDefinition{{Name: "shell"}}},
		SessionStore:  store,
		ContextMgr:    stubContextManager{},
		Parser:        stubParser{},
		Config:        Config{LLMProvider: "mock", LLMModel: "test", MaxIterations: 3},
		Logger:        ports.NoopLogger{},
		Clock:         ports.ClockFunc(func() time.Time { return time.Unix(0, 0) }),
		CostDecorator: NewCostTrackingDecorator(nil, ports.NoopLogger{}, ports.ClockFunc(time.Now)),
		EventEmitter:  ports.NoopEventListener{},
	}

	service := NewExecutionPreparationService(deps)
	env, err := service.Prepare(context.Background(), "第二轮：生成修复计划", session.ID)
	if err != nil {
		t.Fatalf("prepare execution failed: %v", err)
	}

	for _, msg := range session.Messages {
		if msg.Source == ports.MessageSourceSystemPrompt {
			continue
		}
		found := false
		for _, stateMsg := range env.State.Messages {
			if stateMsg.Source != ports.MessageSourceUserHistory {
				continue
			}
			if stateMsg.Content == msg.Content && stateMsg.Role == msg.Role {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected to find recalled entry for %q", msg.Content)
		}
	}
}

func TestSessionHistoryAccumulatesAcrossTurns(t *testing.T) {
	session := &ports.Session{
		ID: "session-history-stack",
		Messages: []ports.Message{
			{Role: "system", Content: "Legacy persona", Source: ports.MessageSourceSystemPrompt},
			{Role: "user", Content: "第一轮：分析日志", Source: ports.MessageSourceUserInput},
			{Role: "tool", Content: "shell[logs-1]: grep found 2 errors", Source: ports.MessageSourceToolResult, ToolCallID: "logs-1"},
			{Role: "assistant", Content: "我会根据错误代码修复", Source: ports.MessageSourceAssistantReply},
		},
		Metadata: map[string]string{},
	}
	store := &stubSessionStore{session: session}
	coordinator := &AgentCoordinator{
		sessionStore: store,
		logger:       ports.NoopLogger{},
		clock:        ports.ClockFunc(func() time.Time { return time.Date(2024, time.July, 1, 10, 0, 0, 0, time.UTC) }),
	}
	secondRound := []ports.Message{
		{Role: "user", Content: "第二轮：生成修复计划", Source: ports.MessageSourceUserInput},
		{Role: "tool", Content: "shell[plan-1]: patched failing test", Source: ports.MessageSourceToolResult, ToolCallID: "plan-1"},
		{Role: "assistant", Content: "已完成计划", Source: ports.MessageSourceAssistantReply},
	}
	resultMessages := append([]ports.Message(nil), session.Messages...)
	resultMessages = append(resultMessages, secondRound...)
	result := &ports.TaskResult{SessionID: session.ID, TaskID: "task-second", Messages: resultMessages}
	if err := coordinator.SaveSessionAfterExecution(context.Background(), session, result); err != nil {
		t.Fatalf("save session after execution failed: %v", err)
	}
	if len(store.session.Messages) != len(resultMessages) {
		t.Fatalf("expected persisted history to include both rounds, got %d entries", len(store.session.Messages))
	}
	for i, msg := range resultMessages {
		persisted := store.session.Messages[i]
		if persisted.Content != msg.Content || persisted.Source != msg.Source || persisted.Role != msg.Role || persisted.ToolCallID != msg.ToolCallID {
			t.Fatalf("history mismatch at %d: want %#v, got %#v", i, msg, persisted)
		}
	}
	deps := ExecutionPreparationDeps{
		LLMFactory:    &fakeLLMFactory{client: fakeLLMClient{}},
		ToolRegistry:  &registryWithList{defs: []ports.ToolDefinition{{Name: "shell"}}},
		SessionStore:  store,
		ContextMgr:    stubContextManager{},
		Parser:        stubParser{},
		Config:        Config{LLMProvider: "mock", LLMModel: "test", MaxIterations: 3},
		Logger:        ports.NoopLogger{},
		Clock:         ports.ClockFunc(func() time.Time { return time.Unix(0, 0) }),
		CostDecorator: NewCostTrackingDecorator(nil, ports.NoopLogger{}, ports.ClockFunc(time.Now)),
		EventEmitter:  ports.NoopEventListener{},
	}
	service := NewExecutionPreparationService(deps)
	env, err := service.Prepare(context.Background(), "第三轮：提交修复", session.ID)
	if err != nil {
		t.Fatalf("prepare execution failed: %v", err)
	}
	if len(env.State.Messages) == 0 {
		t.Fatalf("expected state to preload previous turns")
	}
}

func TestPrepareUsesInheritedStateForSubagent(t *testing.T) {
	session := &ports.Session{ID: "session-inherited", Metadata: map[string]string{}}
	store := &stubSessionStore{session: session}
	deps := ExecutionPreparationDeps{
		LLMFactory:    &fakeLLMFactory{client: fakeLLMClient{}},
		ToolRegistry:  &registryWithList{},
		SessionStore:  store,
		ContextMgr:    stubContextManager{},
		Parser:        stubParser{},
		Config:        Config{LLMProvider: "mock", LLMModel: "test", MaxIterations: 2},
		Logger:        ports.NoopLogger{},
		Clock:         ports.ClockFunc(func() time.Time { return time.Unix(0, 0) }),
		CostDecorator: NewCostTrackingDecorator(nil, ports.NoopLogger{}, ports.ClockFunc(time.Now)),
		EventEmitter:  ports.NoopEventListener{},
	}

	service := NewExecutionPreparationService(deps)
	snapshot := &ports.TaskState{
		SystemPrompt: "You are the orchestrator",
		Messages: []ports.Message{{
			Role:    "system",
			Content: "Previous reasoning",
			Source:  ports.MessageSourceSystemPrompt,
		}},
		Attachments: map[string]ports.Attachment{
			"report.md": {Name: "report.md", Data: "YmFzZQ=="},
		},
		AttachmentIterations: map[string]int{"report.md": 4},
		Plans:                []ports.PlanNode{{ID: "plan-1", Title: "Investigate"}},
		Beliefs:              []ports.Belief{{Statement: "Delegation works"}},
		KnowledgeRefs:        []ports.KnowledgeReference{{ID: "rag-1", Description: "Docs"}},
		WorldState:           map[string]any{"last_tool": "think"},
		WorldDiff:            map[string]any{"iteration": 2},
		FeedbackSignals:      []ports.FeedbackSignal{{Kind: "info"}},
	}

	ctx := MarkSubagentContext(context.Background())
	ctx = ports.WithTaskStateSnapshot(ctx, snapshot)

	env, err := service.Prepare(ctx, "Break down the delegated task", "")
	if err != nil {
		t.Fatalf("prepare execution failed: %v", err)
	}

	if env.State.SystemPrompt != snapshot.SystemPrompt {
		t.Fatalf("expected system prompt to inherit, got %q", env.State.SystemPrompt)
	}
	if len(env.State.Messages) != len(snapshot.Messages) {
		t.Fatalf("expected inherited messages, got %d entries", len(env.State.Messages))
	}
	if env.State.Attachments["report.md"].Data != "YmFzZQ==" {
		t.Fatalf("expected inherited attachment payload")
	}
	if env.State.AttachmentIterations["report.md"] != 4 {
		t.Fatalf("expected inherited attachment iteration, got %d", env.State.AttachmentIterations["report.md"])
	}
	if len(env.State.Plans) != 1 || env.State.Plans[0].ID != "plan-1" {
		t.Fatalf("expected inherited plan nodes")
	}
	if len(env.State.Beliefs) != 1 || env.State.Beliefs[0].Statement != "Delegation works" {
		t.Fatalf("expected inherited beliefs")
	}
	if len(env.State.KnowledgeRefs) != 1 || env.State.KnowledgeRefs[0].ID != "rag-1" {
		t.Fatalf("expected inherited knowledge references")
	}
	if env.State.WorldState["last_tool"] != "think" {
		t.Fatalf("expected inherited world state")
	}
	if env.State.WorldDiff["iteration"] != 2 {
		t.Fatalf("expected inherited world diff")
	}
	if len(env.State.FeedbackSignals) != 1 || env.State.FeedbackSignals[0].Kind != "info" {
		t.Fatalf("expected inherited feedback signals")
	}
}

func collectHistoryMessages(messages []ports.Message) []ports.Message {
	var recalled []ports.Message
	for _, msg := range messages {
		if msg.Source == ports.MessageSourceUserHistory {
			recalled = append(recalled, msg)
		}
	}
	return recalled
}
