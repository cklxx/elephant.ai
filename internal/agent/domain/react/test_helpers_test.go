package react

import agent "alex/internal/agent/ports/agent"

func newReactEngineForTest(maxIterations int) *ReactEngine {
	return NewReactEngine(ReactEngineConfig{
		MaxIterations: maxIterations,
		Logger:        agent.NoopLogger{},
		Clock:         agent.SystemClock{},
	})
}
