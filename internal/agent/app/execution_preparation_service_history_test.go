package app

import (
	"context"
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
	gate := &recordingGate{directives: ports.RAGDirectives{}}
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
		RAGGate:       gate,
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

	if gate.signals.Query == "" {
		t.Fatalf("expected rag gate signals to capture base query")
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
	if trimmed := strings.TrimSpace(historyMessages[0].Content); trimmed != fakeHistorySummaryResponse {
		t.Fatalf("expected summary content to match fake LLM output, got %q", trimmed)
	}
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

func collectHistoryMessages(messages []ports.Message) []ports.Message {
	var recalled []ports.Message
	for _, msg := range messages {
		if msg.Source == ports.MessageSourceUserHistory {
			recalled = append(recalled, msg)
		}
	}
	return recalled
}
