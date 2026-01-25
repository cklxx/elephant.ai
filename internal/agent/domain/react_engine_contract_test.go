package domain

import agent "alex/internal/agent/ports/agent"

var _ agent.ReactiveExecutor = (*ReactEngine)(nil)
