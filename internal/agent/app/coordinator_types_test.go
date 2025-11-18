package app

import (
	"context"
	"fmt"
	"testing"
	"time"

	"alex/internal/agent/ports"
	"alex/internal/llm"
)

type stubSessionStore struct {
	session *ports.Session
}

func (s *stubSessionStore) Create(ctx context.Context) (*ports.Session, error) {
	sess := &ports.Session{
		ID:        "session-stub",
		Messages:  nil,
		Metadata:  make(map[string]string),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	s.session = sess
	return sess, nil
}

func (s *stubSessionStore) Get(ctx context.Context, id string) (*ports.Session, error) {
	if id == "" {
		return s.Create(ctx)
	}
	if s.session != nil && s.session.ID == id {
		return s.session, nil
	}
	sess := &ports.Session{
		ID:        id,
		Messages:  nil,
		Metadata:  make(map[string]string),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	s.session = sess
	return sess, nil
}

func (s *stubSessionStore) Save(ctx context.Context, session *ports.Session) error {
	s.session = session
	return nil
}

func (s *stubSessionStore) List(ctx context.Context) ([]string, error) {
	if s.session == nil {
		return []string{}, nil
	}
	return []string{s.session.ID}, nil
}

func (s *stubSessionStore) Delete(ctx context.Context, id string) error {
	if s.session != nil && s.session.ID == id {
		s.session = nil
	}
	return nil
}

type stubContextManager struct{}

func (stubContextManager) EstimateTokens(messages []ports.Message) int { return len(messages) * 10 }
func (stubContextManager) Compress(messages []ports.Message, targetTokens int) ([]ports.Message, error) {
	return messages, nil
}
func (stubContextManager) ShouldCompress(messages []ports.Message, limit int) bool { return false }
func (stubContextManager) Preload(context.Context) error                           { return nil }
func (stubContextManager) BuildWindow(ctx context.Context, session *ports.Session, cfg ports.ContextWindowConfig) (ports.ContextWindow, error) {
	if session == nil {
		return ports.ContextWindow{}, fmt.Errorf("session required")
	}
	return ports.ContextWindow{SessionID: session.ID, Messages: session.Messages}, nil
}
func (stubContextManager) RecordTurn(context.Context, ports.ContextTurnRecord) error { return nil }

type stubToolRegistry struct{}

func (stubToolRegistry) Register(tool ports.ToolExecutor) error { return nil }
func (stubToolRegistry) Get(name string) (ports.ToolExecutor, error) {
	return nil, fmt.Errorf("tool %s not found", name)
}
func (stubToolRegistry) List() []ports.ToolDefinition { return nil }
func (stubToolRegistry) Unregister(name string) error { return nil }

type stubParser struct{}

func (stubParser) Parse(content string) ([]ports.ToolCall, error)                      { return nil, nil }
func (stubParser) Validate(call ports.ToolCall, definition ports.ToolDefinition) error { return nil }

func TestPrepareExecutionReturnsTypedEnvironment(t *testing.T) {
llmFactory := llm.NewFactory()
sessionStore := &stubSessionStore{}
coordinator := NewAgentCoordinator(
llmFactory,
stubToolRegistry{},
sessionStore,
stubContextManager{},
stubParser{},
nil,
Config{LLMProvider: "mock", LLMModel: "test-model", MaxIterations: 5},
)

	env, err := coordinator.PrepareExecution(context.Background(), "test", "")
	if err != nil {
		t.Fatalf("prepare execution failed: %v", err)
	}

	if env.State == nil {
		t.Fatal("expected non-nil state")
	}
	env.State.Iterations = 1 // ensure direct access without type assertion

	if env.Services.LLM == nil {
		t.Fatal("expected LLM client in services bundle")
	}

	result := &ports.TaskResult{Messages: []ports.Message{{Role: "assistant", Content: "done"}}}
	if err := coordinator.SaveSessionAfterExecution(context.Background(), env.Session, result); err != nil {
		t.Fatalf("save session failed: %v", err)
	}

	if len(env.Session.Messages) != 1 {
		t.Fatalf("expected session messages to be copied, got %d", len(env.Session.Messages))
	}
}

func TestSanitizeMessagesForPersistenceSkipsUserHistory(t *testing.T) {
	messages := []ports.Message{
		{
			Role:    "system",
			Source:  ports.MessageSourceSystemPrompt,
			Content: "persist me",
		},
		{
			Role:    "system",
			Source:  ports.MessageSourceUserHistory,
			Content: "transient summary",
			Attachments: map[string]ports.Attachment{
				"summary.txt": {
					Name: "summary.txt",
				},
			},
		},
		{
			Role:    "assistant",
			Source:  ports.MessageSourceAssistantReply,
			Content: "final answer",
			Attachments: map[string]ports.Attachment{
				"diagram.png": {
					Name:      "diagram.png",
					MediaType: "image/png",
				},
			},
		},
	}

	sanitized, attachments := sanitizeMessagesForPersistence(messages)

	if len(sanitized) != 2 {
		t.Fatalf("expected 2 sanitized messages, got %d", len(sanitized))
	}

	if sanitized[0].Source != ports.MessageSourceSystemPrompt {
		t.Fatalf("expected first sanitized message to be the system prompt, got %s", sanitized[0].Source)
	}

	if sanitized[1].Source != ports.MessageSourceAssistantReply {
		t.Fatalf("expected second sanitized message to be the assistant reply, got %s", sanitized[1].Source)
	}

	if sanitized[1].Attachments != nil {
		t.Fatalf("expected attachments to be stripped from sanitized message, got %+v", sanitized[1].Attachments)
	}

	if len(attachments) != 1 {
		t.Fatalf("expected 1 persisted attachment, got %d", len(attachments))
	}

	if _, ok := attachments["diagram.png"]; !ok {
		t.Fatalf("expected diagram.png attachment to be persisted, got keys %v", attachments)
	}
}

func TestSanitizeMessagesForPersistenceAllUserHistory(t *testing.T) {
	messages := []ports.Message{
		{
			Role:    "system",
			Source:  ports.MessageSourceUserHistory,
			Content: "transient summary",
		},
	}

	sanitized, attachments := sanitizeMessagesForPersistence(messages)

	if sanitized != nil {
		t.Fatalf("expected sanitized messages to be nil, got %+v", sanitized)
	}

	if attachments != nil {
		t.Fatalf("expected attachments to be nil, got %+v", attachments)
	}
}

func TestCoordinatorGetConfigIncludesCompletionDefaults(t *testing.T) {
	llmFactory := llm.NewFactory()
	sessionStore := &stubSessionStore{}
	stopSeqs := []string{"<<END>>"}

coordinator := NewAgentCoordinator(
llmFactory,
stubToolRegistry{},
sessionStore,
stubContextManager{},
stubParser{},
nil,
		Config{
			LLMProvider:         "mock",
			LLMModel:            "config-check",
			MaxIterations:       3,
			MaxTokens:           2048,
			Temperature:         0.55,
			TemperatureProvided: true,
			TopP:                0.9,
			StopSequences:       stopSeqs,
		},
	)

	cfg := coordinator.GetConfig()

	if cfg.Temperature != 0.55 {
		t.Fatalf("expected temperature 0.55, got %.2f", cfg.Temperature)
	}
	if cfg.TopP != 0.9 {
		t.Fatalf("expected top-p 0.9, got %.2f", cfg.TopP)
	}
	if len(cfg.StopSequences) != len(stopSeqs) || cfg.StopSequences[0] != stopSeqs[0] {
		t.Fatalf("expected stop sequences %v, got %v", stopSeqs, cfg.StopSequences)
	}

	// Mutating the returned slice should not affect coordinator state.
	cfg.StopSequences[0] = "mutated"
	cfg2 := coordinator.GetConfig()
	if cfg2.StopSequences[0] != stopSeqs[0] {
		t.Fatalf("expected coordinator to protect stop sequences copy, got %v", cfg2.StopSequences)
	}
}

func TestNewAgentCoordinatorHonorsZeroTemperature(t *testing.T) {
	llmFactory := llm.NewFactory()
	sessionStore := &stubSessionStore{}

coordinator := NewAgentCoordinator(
llmFactory,
stubToolRegistry{},
sessionStore,
stubContextManager{},
stubParser{},
nil,
		Config{
			LLMProvider:         "mock",
			LLMModel:            "deterministic",
			Temperature:         0,
			TemperatureProvided: true,
			MaxIterations:       1,
		},
	)

	cfg := coordinator.GetConfig()
	if cfg.Temperature != 0 {
		t.Fatalf("expected zero temperature to be preserved, got %.2f", cfg.Temperature)
	}

	defaults := buildCompletionDefaultsFromConfig(coordinator.config)
	if defaults.Temperature == nil {
		t.Fatalf("expected completion defaults to include explicit zero temperature override")
	}
	if got := *defaults.Temperature; got != 0 {
		t.Fatalf("expected completion defaults temperature 0, got %.2f", got)
	}
}

// Ensure the coordinator continues to satisfy the AgentCoordinator port contract.
var _ ports.AgentCoordinator = (*AgentCoordinator)(nil)
