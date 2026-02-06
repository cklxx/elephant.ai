package preparation

import (
	"context"
	"fmt"
	"testing"
	"time"

	appconfig "alex/internal/app/agent/config"
	"alex/internal/app/agent/cost"
	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	storage "alex/internal/domain/agent/ports/storage"
)

func TestPrepareSeedsPlanBeliefAndKnowledgeRefs(t *testing.T) {
	session := &storage.Session{ID: "session-plan-1", Messages: []ports.Message{}, Metadata: map[string]string{}}
	store := &stubSessionStore{session: session}
	deps := ExecutionPreparationDeps{
		LLMFactory:    &fakeLLMFactory{client: fakeLLMClient{}},
		ToolRegistry:  &registryWithList{},
		SessionStore:  store,
		ContextMgr:    stubContextManager{},
		Parser:        stubParser{},
		Config:        appconfig.Config{LLMProvider: "mock", LLMModel: "test-model", MaxIterations: 3},
		Logger:        agent.NoopLogger{},
		Clock:         agent.ClockFunc(func() time.Time { return time.Date(2024, time.June, 2, 9, 0, 0, 0, time.UTC) }),
		CostDecorator: cost.NewCostTrackingDecorator(nil, agent.NoopLogger{}, agent.ClockFunc(time.Now)),
		EventEmitter:  agent.NoopEventListener{},
	}

	service := NewExecutionPreparationService(deps)
	env, err := service.Prepare(context.Background(), "Research current marketing trends", session.ID)
	if err != nil {
		t.Fatalf("prepare execution failed: %v", err)
	}

	if env.State == nil {
		t.Fatalf("expected state to be populated")
	}

	if len(env.State.Plans) != 0 {
		t.Fatalf("expected no derived plans after task analysis removal, got %+v", env.State.Plans)
	}

	if len(env.State.Beliefs) != 0 {
		t.Fatalf("expected no beliefs seeded without task analysis, got %+v", env.State.Beliefs)
	}

	if len(env.State.KnowledgeRefs) != 0 {
		t.Fatalf("expected no knowledge references seeded without task analysis, got %+v", env.State.KnowledgeRefs)
	}
}

func TestPrepareCapturesWorldStateFromContextManager(t *testing.T) {
	session := &storage.Session{ID: "session-world", Messages: []ports.Message{}, Metadata: map[string]string{}}
	store := &stubSessionStore{session: session}
	deps := ExecutionPreparationDeps{
		LLMFactory:    &fakeLLMFactory{client: fakeLLMClient{}},
		ToolRegistry:  &registryWithList{},
		SessionStore:  store,
		ContextMgr:    worldAwareContextManager{},
		Parser:        stubParser{},
		Config:        appconfig.Config{LLMProvider: "mock", LLMModel: "test-model", MaxIterations: 3, EnvironmentSummary: "Local"},
		Logger:        agent.NoopLogger{},
		Clock:         agent.ClockFunc(func() time.Time { return time.Date(2024, time.June, 3, 10, 0, 0, 0, time.UTC) }),
		CostDecorator: cost.NewCostTrackingDecorator(nil, agent.NoopLogger{}, agent.ClockFunc(time.Now)),
		EventEmitter:  agent.NoopEventListener{},
	}
	service := NewExecutionPreparationService(deps)
	env, err := service.Prepare(context.Background(), "Audit world config", session.ID)
	if err != nil {
		t.Fatalf("prepare execution failed: %v", err)
	}
	if env.State == nil {
		t.Fatalf("expected state to be populated")
	}
	if env.State.WorldState == nil {
		t.Fatalf("expected world state to be seeded")
	}
	profile, ok := env.State.WorldState["profile"].(map[string]any)
	if !ok || profile["id"] != "local" {
		t.Fatalf("expected world profile metadata, got %+v", env.State.WorldState)
	}
	if env.State.WorldDiff == nil || env.State.WorldDiff["profile_loaded"] != "local" {
		t.Fatalf("expected world diff to capture profile load, got %+v", env.State.WorldDiff)
	}
}

type worldAwareContextManager struct{}

func (worldAwareContextManager) EstimateTokens(messages []ports.Message) int { return len(messages) }
func (worldAwareContextManager) Compress(messages []ports.Message, targetTokens int) ([]ports.Message, error) {
	return messages, nil
}
func (worldAwareContextManager) AutoCompact(messages []ports.Message, limit int) ([]ports.Message, bool) {
	return messages, false
}
func (worldAwareContextManager) ShouldCompress(messages []ports.Message, limit int) bool {
	return false
}
func (worldAwareContextManager) Preload(context.Context) error { return nil }
func (worldAwareContextManager) BuildWindow(ctx context.Context, session *storage.Session, cfg agent.ContextWindowConfig) (agent.ContextWindow, error) {
	if session == nil {
		return agent.ContextWindow{}, fmt.Errorf("session required")
	}
	return agent.ContextWindow{
		SessionID: session.ID,
		Messages:  append([]ports.Message(nil), session.Messages...),
		Static: agent.StaticContext{
			World: agent.WorldProfile{
				ID:           "local",
				Environment:  "ci",
				Capabilities: []string{"fs"},
				Limits:       []string{"rate"},
			},
			EnvironmentSummary: cfg.EnvironmentSummary,
		},
	}, nil
}
func (worldAwareContextManager) RecordTurn(context.Context, agent.ContextTurnRecord) error {
	return nil
}
