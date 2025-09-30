package main

import (
	"context"

	"alex/internal/agent/app"
	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	ctxmgr "alex/internal/context"
	"alex/internal/llm"
	"alex/internal/messaging"
	"alex/internal/parser"
	"alex/internal/session/filestore"
	"alex/internal/tools"
)

// ListSessions is a helper method on Coordinator
func (c *Container) ListSessions(ctx context.Context) ([]string, error) {
	return []string{}, nil // TODO: Implement via session store
}

type Container struct {
	Coordinator  *app.AgentCoordinator
	SessionStore ports.SessionStore
}

func buildContainer() (*Container, error) {
	// Load configuration
	config := loadConfig()

	// Infrastructure Layer
	llmFactory := llm.NewFactory()
	toolRegistry := tools.NewRegistry()
	sessionStore := filestore.New("~/.alex-sessions")
	contextMgr := ctxmgr.NewManager()
	parserImpl := parser.New()
	messageQueue := messaging.NewQueue(100)

	// Domain Layer
	reactEngine := domain.NewReactEngine(config.MaxIterations)

	// Application Layer
	coordinator := app.NewAgentCoordinator(
		llmFactory,
		toolRegistry,
		sessionStore,
		contextMgr,
		parserImpl,
		messageQueue,
		reactEngine,
		app.Config{
			LLMProvider:   config.LLMProvider,
			LLMModel:      config.LLMModel,
			APIKey:        config.APIKey,
			BaseURL:       config.BaseURL,
			MaxTokens:     config.MaxTokens,
			MaxIterations: config.MaxIterations,
		},
	)

	return &Container{
		Coordinator:  coordinator,
		SessionStore: sessionStore,
	}, nil
}
