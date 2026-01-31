package coordinator

import (
	"context"
	"fmt"
	"testing"
	"time"

	appconfig "alex/internal/agent/app/config"
	appcontext "alex/internal/agent/app/context"
	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	storage "alex/internal/agent/ports/storage"
	tools "alex/internal/agent/ports/tools"
	"alex/internal/llm"
	id "alex/internal/utils/id"
)

type stubSessionStore struct {
	session *storage.Session
}

func (s *stubSessionStore) Create(ctx context.Context) (*storage.Session, error) {
	sess := &storage.Session{
		ID:        "session-stub",
		Messages:  nil,
		Metadata:  make(map[string]string),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	s.session = sess
	return sess, nil
}

func (s *stubSessionStore) Get(ctx context.Context, id string) (*storage.Session, error) {
	if id == "" {
		return s.Create(ctx)
	}
	if s.session != nil && s.session.ID == id {
		return s.session, nil
	}
	sess := &storage.Session{
		ID:        id,
		Messages:  nil,
		Metadata:  make(map[string]string),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	s.session = sess
	return sess, nil
}

func (s *stubSessionStore) Save(ctx context.Context, session *storage.Session) error {
	s.session = session
	return nil
}

func (s *stubSessionStore) List(ctx context.Context, limit int, offset int) ([]string, error) {
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

type stubHistoryManager struct {
	clearedSessionID string
	appendCalled     bool
}

func (s *stubHistoryManager) AppendTurn(context.Context, string, []ports.Message) error {
	s.appendCalled = true
	return nil
}

func (s *stubHistoryManager) Replay(context.Context, string, int) ([]ports.Message, error) {
	return nil, nil
}

func (s *stubHistoryManager) ClearSession(_ context.Context, sessionID string) error {
	s.clearedSessionID = sessionID
	return nil
}

type stubContextManager struct{}

func (stubContextManager) EstimateTokens(messages []ports.Message) int { return len(messages) * 10 }
func (stubContextManager) Compress(messages []ports.Message, targetTokens int) ([]ports.Message, error) {
	return messages, nil
}
func (stubContextManager) AutoCompact(messages []ports.Message, limit int) ([]ports.Message, bool) {
	return messages, false
}
func (stubContextManager) ShouldCompress(messages []ports.Message, limit int) bool { return false }
func (stubContextManager) Preload(context.Context) error                           { return nil }
func (stubContextManager) BuildWindow(ctx context.Context, session *storage.Session, cfg agent.ContextWindowConfig) (agent.ContextWindow, error) {
	if session == nil {
		return agent.ContextWindow{}, fmt.Errorf("session required")
	}
	return agent.ContextWindow{SessionID: session.ID, Messages: session.Messages}, nil
}
func (stubContextManager) RecordTurn(context.Context, agent.ContextTurnRecord) error { return nil }

type stubToolRegistry struct{}

func (stubToolRegistry) Register(tool tools.ToolExecutor) error { return nil }
func (stubToolRegistry) Get(name string) (tools.ToolExecutor, error) {
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
		nil,
		stubParser{},
		nil,
		appconfig.Config{LLMProvider: "mock", LLMModel: "test-model", MaxIterations: 5},
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

	result := &agent.TaskResult{Messages: []ports.Message{{Role: "assistant", Content: "done"}}}
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
					Data:      "ZmFrZV9iYXNlNjQ=",
					URI:       "/api/attachments/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.png",
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

	persisted, ok := attachments["diagram.png"]
	if !ok {
		t.Fatalf("expected diagram.png attachment to be persisted, got keys %v", attachments)
	}

	if persisted.Data != "" {
		t.Fatalf("expected persisted attachment data to be cleared when URI is set, got %q", persisted.Data)
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
		nil,
		stubParser{},
		nil,
		appconfig.Config{
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
		nil,
		stubParser{},
		nil,
		appconfig.Config{
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

func TestPersistSessionSnapshotPersistsMessagesWithFallbackIDs(t *testing.T) {
	sessionStore := &stubSessionStore{}
	coordinator := &AgentCoordinator{
		sessionStore: sessionStore,
		logger:       agent.NoopLogger{},
		clock:        agent.SystemClock{},
	}

	env := &agent.ExecutionEnvironment{
		Session: &storage.Session{
			ID:        "session-generated",
			Metadata:  map[string]string{},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		State: &agent.TaskState{
			Messages:   []ports.Message{{Role: "assistant", Source: ports.MessageSourceAssistantReply, Content: "partial"}},
			Iterations: 2,
			TokenCount: 42,
		},
	}

	coordinator.persistSessionSnapshot(context.Background(), env, "task-123", "parent-456", "error")

	saved := sessionStore.session
	if saved == nil {
		t.Fatalf("expected session to be saved")
	}
	if saved.Metadata["last_task_id"] != "task-123" {
		t.Fatalf("expected last_task_id metadata to be persisted, got %q", saved.Metadata["last_task_id"])
	}
	if saved.Metadata["last_parent_task_id"] != "parent-456" {
		t.Fatalf("expected last_parent_task_id metadata, got %q", saved.Metadata["last_parent_task_id"])
	}
	if saved.Metadata["session_id"] != env.Session.ID {
		t.Fatalf("expected session metadata to include session_id, got %q", saved.Metadata["session_id"])
	}
	if len(saved.Messages) == 0 {
		t.Fatalf("expected persisted messages to be stored")
	}
}

func TestPersistSessionSnapshotSkipsWhenStateMissing(t *testing.T) {
	sessionStore := &stubSessionStore{}
	coordinator := &AgentCoordinator{
		sessionStore: sessionStore,
		logger:       agent.NoopLogger{},
		clock:        agent.SystemClock{},
	}

	coordinator.persistSessionSnapshot(context.Background(), nil, "task-123", "parent-456", "error")
	if sessionStore.session != nil {
		t.Fatalf("expected no session to be saved when env is nil")
	}

	env := &agent.ExecutionEnvironment{
		Session: &storage.Session{
			ID:        "session-generated",
			Metadata:  map[string]string{},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}
	coordinator.persistSessionSnapshot(context.Background(), env, "task-123", "parent-456", "error")
	if sessionStore.session != nil {
		t.Fatalf("expected no session to be saved when state is nil")
	}
}

func TestResolveUserID(t *testing.T) {
	coordinator := &AgentCoordinator{logger: agent.NoopLogger{}}

	t.Run("nil session", func(t *testing.T) {
		if got := coordinator.resolveUserID(context.Background(), nil); got != "" {
			t.Fatalf("expected empty, got %q", got)
		}
	})

	t.Run("nil metadata", func(t *testing.T) {
		session := &storage.Session{ID: "test-session"}
		if got := coordinator.resolveUserID(context.Background(), session); got != "" {
			t.Fatalf("expected empty, got %q", got)
		}
	})

	t.Run("context user_id takes precedence", func(t *testing.T) {
		ctx := id.WithUserID(context.Background(), "ctx-user")
		session := &storage.Session{
			ID:       "lark-abc123",
			Metadata: map[string]string{"user_id": "meta-user"},
		}
		if got := coordinator.resolveUserID(ctx, session); got != "ctx-user" {
			t.Fatalf("expected 'ctx-user', got %q", got)
		}
	})

	t.Run("user_id from metadata", func(t *testing.T) {
		session := &storage.Session{
			ID:       "lark-abc123",
			Metadata: map[string]string{"user_id": "ou_user_42"},
		}
		if got := coordinator.resolveUserID(context.Background(), session); got != "ou_user_42" {
			t.Fatalf("expected 'ou_user_42', got %q", got)
		}
	})

	t.Run("lark- prefix fallback", func(t *testing.T) {
		session := &storage.Session{
			ID:       "lark-abc123",
			Metadata: map[string]string{},
		}
		if got := coordinator.resolveUserID(context.Background(), session); got != "lark-abc123" {
			t.Fatalf("expected 'lark-abc123', got %q", got)
		}
	})

	t.Run("wechat- prefix fallback", func(t *testing.T) {
		session := &storage.Session{
			ID:       "wechat-def456",
			Metadata: map[string]string{},
		}
		if got := coordinator.resolveUserID(context.Background(), session); got != "wechat-def456" {
			t.Fatalf("expected 'wechat-def456', got %q", got)
		}
	})

	t.Run("lark colon prefix no longer matches", func(t *testing.T) {
		session := &storage.Session{
			ID:       "lark:old-format",
			Metadata: map[string]string{},
		}
		if got := coordinator.resolveUserID(context.Background(), session); got != "" {
			t.Fatalf("expected empty for 'lark:' prefix (wrong separator), got %q", got)
		}
	})

	t.Run("non-channel session returns empty", func(t *testing.T) {
		session := &storage.Session{
			ID:       "cli-session-xyz",
			Metadata: map[string]string{},
		}
		if got := coordinator.resolveUserID(context.Background(), session); got != "" {
			t.Fatalf("expected empty for non-channel session, got %q", got)
		}
	})

	t.Run("user_id takes precedence over prefix fallback", func(t *testing.T) {
		session := &storage.Session{
			ID:       "lark-abc123",
			Metadata: map[string]string{"user_id": "ou_real_user"},
		}
		if got := coordinator.resolveUserID(context.Background(), session); got != "ou_real_user" {
			t.Fatalf("expected 'ou_real_user', got %q", got)
		}
	})
}

func TestResetSessionClearsState(t *testing.T) {
	session := &storage.Session{
		ID:       "session-reset",
		Messages: []ports.Message{{Role: "user", Content: "hello"}},
		Metadata: map[string]string{"user_id": "ou_user"},
		Todos:    []storage.Todo{{ID: "todo-1", Description: "check"}},
		Attachments: map[string]ports.Attachment{
			"file.txt": {Name: "file.txt"},
		},
		Important: map[string]ports.ImportantNote{
			"note": {Content: "remember"},
		},
	}
	store := &stubSessionStore{session: session}
	history := &stubHistoryManager{}

	coordinator := NewAgentCoordinator(
		llm.NewFactory(),
		stubToolRegistry{},
		store,
		stubContextManager{},
		history,
		stubParser{},
		nil,
		appconfig.Config{LLMProvider: "mock", LLMModel: "test-model", MaxIterations: 3},
	)

	if err := coordinator.ResetSession(context.Background(), session.ID); err != nil {
		t.Fatalf("reset session failed: %v", err)
	}
	if store.session == nil {
		t.Fatal("expected session to persist")
	}
	if len(store.session.Messages) != 0 {
		t.Fatalf("expected messages cleared, got %d", len(store.session.Messages))
	}
	if store.session.Metadata != nil {
		t.Fatalf("expected metadata cleared")
	}
	if store.session.Todos != nil {
		t.Fatalf("expected todos cleared")
	}
	if store.session.Attachments != nil {
		t.Fatalf("expected attachments cleared")
	}
	if store.session.Important != nil {
		t.Fatalf("expected important notes cleared")
	}
	if history.clearedSessionID != session.ID {
		t.Fatalf("expected history cleared for %q, got %q", session.ID, history.clearedSessionID)
	}
}

func TestSaveSessionAfterExecutionSkipsHistoryWhenDisabled(t *testing.T) {
	session := &storage.Session{
		ID:       "session-skip",
		Metadata: map[string]string{},
	}
	store := &stubSessionStore{session: session}
	history := &stubHistoryManager{}

	coordinator := NewAgentCoordinator(
		llm.NewFactory(),
		stubToolRegistry{},
		store,
		stubContextManager{},
		history,
		stubParser{},
		nil,
		appconfig.Config{LLMProvider: "mock", LLMModel: "test-model", MaxIterations: 3},
	)

	result := &agent.TaskResult{
		SessionID:  session.ID,
		StopReason: "complete",
		Messages: []ports.Message{
			{Role: "user", Content: "hello"},
		},
	}

	ctx := appcontext.WithSessionHistory(context.Background(), false)
	if err := coordinator.SaveSessionAfterExecution(ctx, session, result); err != nil {
		t.Fatalf("save session failed: %v", err)
	}
	if history.appendCalled {
		t.Fatalf("expected history append to be skipped")
	}
	if store.session == nil || len(store.session.Messages) != 0 {
		t.Fatalf("expected session messages to be cleared")
	}
}

// Ensure the coordinator continues to satisfy the AgentCoordinator port contract.
var _ agent.AgentCoordinator = (*AgentCoordinator)(nil)
