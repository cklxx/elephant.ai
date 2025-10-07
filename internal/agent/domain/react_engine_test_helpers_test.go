package domain_test

import (
	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
)

func newReactEngineForTest(maxIterations int) *domain.ReactEngine {
	return domain.NewReactEngine(domain.ReactEngineConfig{
		MaxIterations: maxIterations,
		Logger:        ports.NoopLogger{},
		Clock:         ports.SystemClock{},
	})
}
