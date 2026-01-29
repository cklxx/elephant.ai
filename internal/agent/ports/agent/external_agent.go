package agent

import "context"

// ExternalAgentExecutor abstracts execution of external code agent processes
// (e.g., Claude Code CLI, Cursor, custom agents).
type ExternalAgentExecutor interface {
	// Execute runs an external agent with the given prompt and returns the result.
	Execute(ctx context.Context, req ExternalAgentRequest) (*ExternalAgentResult, error)
	// SupportedTypes returns the agent types this executor handles.
	SupportedTypes() []string
}

// ExternalAgentRequest contains the parameters for an external agent invocation.
type ExternalAgentRequest struct {
	Prompt      string
	WorkingDir  string
	Config      map[string]string
	SessionID   string
	CausationID string
}

// ExternalAgentResult contains the output from an external agent execution.
type ExternalAgentResult struct {
	Answer     string
	Iterations int
	TokensUsed int
	Error      string
	Metadata   map[string]any
}
