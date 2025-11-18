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
