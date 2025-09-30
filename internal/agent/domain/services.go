package domain

import "alex/internal/agent/ports"

// Services aggregates all injected dependencies for domain logic
type Services struct {
	LLM          ports.LLMClient
	ToolExecutor ports.ToolRegistry
	Parser       ports.FunctionCallParser
	Context      ports.ContextManager
}
