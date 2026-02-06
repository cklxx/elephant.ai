package react

import agent "alex/internal/domain/agent/ports/agent"

var _ agent.ReactiveExecutor = (*ReactEngine)(nil)
