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

	var recallMessage *ports.Message
	for i := range env.State.Messages {
		msg := env.State.Messages[i]
		if msg.Source == ports.MessageSourceUserHistory {
			recallMessage = &msg
			break
		}
	}

	if recallMessage == nil {
		t.Fatalf("expected user history recall message to be injected")
	}

	if !strings.Contains(recallMessage.Content, "marketing plan") {
		t.Fatalf("expected recall message to mention marketing plan, got %q", recallMessage.Content)
	}
	if !strings.Contains(recallMessage.Content, "Assistant") {
		t.Fatalf("expected recall message to include assistant summary, got %q", recallMessage.Content)
	}

	if gate.signals.Query == "" {
		t.Fatalf("expected rag gate signals to capture base query")
	}
	if len(gate.signals.SearchSeeds) == 0 {
		t.Fatalf("expected history-derived search seeds to be forwarded to RAG gate")
	}
	foundSeed := false
	for _, seed := range gate.signals.SearchSeeds {
		if strings.Contains(seed, "marketing") {
			foundSeed = true
			break
		}
	}
	if !foundSeed {
		t.Fatalf("expected at least one history seed to reference marketing, got %#v", gate.signals.SearchSeeds)
	}
}

func TestPrepareSkipsHistoryRecallWhenNoMatch(t *testing.T) {
	session := &ports.Session{
		ID: "session-history-2",
		Messages: []ports.Message{
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
		RAGGate:       nil,
		EventEmitter:  ports.NoopEventListener{},
	}

	service := NewExecutionPreparationService(deps)
	env, err := service.Prepare(context.Background(), "Investigate flaky integration tests", session.ID)
	if err != nil {
		t.Fatalf("prepare execution failed: %v", err)
	}

	for _, msg := range env.State.Messages {
		if msg.Source == ports.MessageSourceUserHistory {
			t.Fatalf("did not expect history recall for unrelated task: %q", msg.Content)
		}
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
	expectedHistory := append([]ports.Message(nil), session.Messages...)

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

	if len(env.State.Messages) != len(expectedHistory) {
		t.Fatalf("expected state to preload %d historical messages, got %d", len(expectedHistory), len(env.State.Messages))
	}
	for i, msg := range expectedHistory {
		got := env.State.Messages[i]
		if got.Content != msg.Content || got.Role != msg.Role || got.Source != msg.Source {
			t.Fatalf("history mismatch at %d: want %#v, got %#v", i, msg, got)
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
	if len(env.State.Messages) != len(resultMessages) {
		t.Fatalf("expected state to preload %d messages, got %d", len(resultMessages), len(env.State.Messages))
	}
	for i, msg := range resultMessages {
		got := env.State.Messages[i]
		if got.Content != msg.Content || got.Source != msg.Source || got.Role != msg.Role || got.ToolCallID != msg.ToolCallID {
			t.Fatalf("state history mismatch at %d: want %#v, got %#v", i, msg, got)
		}
	}
}
