package preparation

import (
	"context"
	"testing"
	"time"

	appconfig "alex/internal/app/agent/config"
	"alex/internal/app/agent/cost"
	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	storage "alex/internal/domain/agent/ports/storage"
)

func TestPrepare_AutoEnablesStewardModeForLarkSessions(t *testing.T) {
	session := &storage.Session{
		ID:       "lark-session-42",
		Messages: []ports.Message{},
		Metadata: map[string]string{"channel": "lark"},
	}
	store := &stubSessionStore{session: session}
	ctxMgr := &capturingStewardContextManager{}

	deps := ExecutionPreparationDeps{
		LLMFactory:    &fakeLLMFactory{client: fakeLLMClient{}},
		ToolRegistry:  &registryWithList{},
		SessionStore:  store,
		ContextMgr:    ctxMgr,
		Parser:        stubParser{},
		Config:        appconfig.Config{LLMProvider: "mock", LLMModel: "test-model", MaxIterations: 3},
		Logger:        agent.NoopLogger{},
		Clock:         agent.ClockFunc(func() time.Time { return time.Unix(0, 0) }),
		CostDecorator: cost.NewCostTrackingDecorator(nil, agent.NoopLogger{}, agent.ClockFunc(time.Now)),
		EventEmitter:  agent.NoopEventListener{},
	}

	service := NewExecutionPreparationService(deps)
	env, err := service.Prepare(context.Background(), "check steward activation", session.ID)
	if err != nil {
		t.Fatalf("prepare failed: %v", err)
	}

	if !ctxMgr.lastCfg.StewardMode {
		t.Fatal("expected context manager steward mode to be enabled")
	}
	if env.State == nil || !env.State.StewardMode {
		t.Fatalf("expected state steward mode enabled, got %+v", env.State)
	}
	if env.State.StewardState == nil {
		t.Fatal("expected steward state to be initialized when steward mode is active")
	}
}

func TestPrepare_AutoEnablesStewardModeForStewardPersona(t *testing.T) {
	session := &storage.Session{
		ID:       "web-session-42",
		Messages: []ports.Message{},
		Metadata: map[string]string{"channel": "web"},
	}
	store := &stubSessionStore{session: session}
	ctxMgr := &capturingStewardContextManager{}

	deps := ExecutionPreparationDeps{
		LLMFactory:    &fakeLLMFactory{client: fakeLLMClient{}},
		ToolRegistry:  &registryWithList{},
		SessionStore:  store,
		ContextMgr:    ctxMgr,
		Parser:        stubParser{},
		Config:        appconfig.Config{LLMProvider: "mock", LLMModel: "test-model", MaxIterations: 3, AgentPreset: "steward"},
		Logger:        agent.NoopLogger{},
		Clock:         agent.ClockFunc(func() time.Time { return time.Unix(0, 0) }),
		CostDecorator: cost.NewCostTrackingDecorator(nil, agent.NoopLogger{}, agent.ClockFunc(time.Now)),
		EventEmitter:  agent.NoopEventListener{},
	}

	service := NewExecutionPreparationService(deps)
	env, err := service.Prepare(context.Background(), "check steward persona activation", session.ID)
	if err != nil {
		t.Fatalf("prepare failed: %v", err)
	}

	if !ctxMgr.lastCfg.StewardMode {
		t.Fatal("expected context manager steward mode to be enabled for steward persona")
	}
	if env.State == nil || !env.State.StewardMode {
		t.Fatalf("expected state steward mode enabled, got %+v", env.State)
	}
}

func TestPrepare_DoesNotEnableStewardModeOutsideTargetContexts(t *testing.T) {
	session := &storage.Session{
		ID:       "web-session-7",
		Messages: []ports.Message{},
		Metadata: map[string]string{"channel": "web"},
	}
	store := &stubSessionStore{session: session}
	ctxMgr := &capturingStewardContextManager{}

	deps := ExecutionPreparationDeps{
		LLMFactory:    &fakeLLMFactory{client: fakeLLMClient{}},
		ToolRegistry:  &registryWithList{},
		SessionStore:  store,
		ContextMgr:    ctxMgr,
		Parser:        stubParser{},
		Config:        appconfig.Config{LLMProvider: "mock", LLMModel: "test-model", MaxIterations: 3, AgentPreset: "default"},
		Logger:        agent.NoopLogger{},
		Clock:         agent.ClockFunc(func() time.Time { return time.Unix(0, 0) }),
		CostDecorator: cost.NewCostTrackingDecorator(nil, agent.NoopLogger{}, agent.ClockFunc(time.Now)),
		EventEmitter:  agent.NoopEventListener{},
	}

	service := NewExecutionPreparationService(deps)
	env, err := service.Prepare(context.Background(), "check steward disabled", session.ID)
	if err != nil {
		t.Fatalf("prepare failed: %v", err)
	}

	if ctxMgr.lastCfg.StewardMode {
		t.Fatal("expected context manager steward mode disabled")
	}
	if env.State == nil {
		t.Fatal("expected state to be populated")
	}
	if env.State.StewardMode {
		t.Fatal("expected steward mode disabled")
	}
	if env.State.StewardState != nil {
		t.Fatalf("expected steward state nil when steward mode disabled, got %+v", env.State.StewardState)
	}
}

type capturingStewardContextManager struct {
	lastCfg agent.ContextWindowConfig
}

func (m *capturingStewardContextManager) EstimateTokens(messages []ports.Message) int {
	return len(messages)
}

func (m *capturingStewardContextManager) Compress(messages []ports.Message, targetTokens int) ([]ports.Message, error) {
	return messages, nil
}

func (m *capturingStewardContextManager) AutoCompact(messages []ports.Message, limit int) ([]ports.Message, bool) {
	return messages, false
}

func (m *capturingStewardContextManager) ShouldCompress(messages []ports.Message, limit int) bool {
	return false
}

func (m *capturingStewardContextManager) Preload(context.Context) error { return nil }

func (m *capturingStewardContextManager) BuildWindow(_ context.Context, session *storage.Session, cfg agent.ContextWindowConfig) (agent.ContextWindow, error) {
	m.lastCfg = cfg
	return agent.ContextWindow{
		SessionID:    session.ID,
		Messages:     append([]ports.Message(nil), session.Messages...),
		SystemPrompt: "sys",
		Dynamic:      agent.DynamicContext{},
	}, nil
}

func (m *capturingStewardContextManager) RecordTurn(context.Context, agent.ContextTurnRecord) error {
	return nil
}
