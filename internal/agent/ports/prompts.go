package ports

// PromptLoader handles loading and rendering prompt templates
// This interface abstracts the concrete prompts.Loader implementation
type PromptLoader interface {
	// Get returns a prompt template by name
	Get(name string) (string, error)

	// Render renders a prompt template with variable substitution
	Render(name string, variables map[string]string) (string, error)

	// GetSystemPrompt returns the system prompt with context
	GetSystemPrompt(workingDir, goal string, analysis *TaskAnalysisInfo) (string, error)

	// List returns all available prompt template names
	List() []string
}

// TaskAnalysisInfo contains task analysis data for prompt injection
type TaskAnalysisInfo struct {
	Action   string
	Goal     string
	Approach string
}
