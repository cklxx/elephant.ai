package domain_test

import (
	"alex/internal/agent/domain"
	agent "alex/internal/agent/ports/agent"
)

func newReactEngineForTest(maxIterations int) *domain.ReactEngine {
	return domain.NewReactEngine(domain.ReactEngineConfig{
		MaxIterations: maxIterations,
		Logger:        agent.NoopLogger{},
		Clock:         agent.SystemClock{},
	})
}
