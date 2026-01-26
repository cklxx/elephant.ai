package output

import (
	"time"

	"alex/internal/agent/domain"
	"alex/internal/agent/types"
)

// OutputTarget represents different output destinations
type OutputTarget string

const (
	TargetLLM OutputTarget = "llm" // For LLM consumption (complete, structured)
	TargetCLI OutputTarget = "cli" // For CLI display (concise, formatted)
	TargetSSE OutputTarget = "sse" // For SSE streaming (JSON events)
	TargetTUI OutputTarget = "tui" // For interactive TUI display
)

// Renderer defines the interface for rendering agent output
type Renderer interface {
	// Target returns the output target this renderer is for
	Target() OutputTarget

	// RenderToolCallStart renders tool call start
	RenderToolCallStart(ctx *types.OutputContext, toolName string, args map[string]interface{}) string

	// RenderToolCallComplete renders tool call completion
	RenderToolCallComplete(ctx *types.OutputContext, toolName string, result string, err error, duration time.Duration) string

	// RenderTaskComplete renders task completion
	RenderTaskComplete(ctx *types.OutputContext, result *domain.TaskResult) string

	// RenderError renders an error
	RenderError(ctx *types.OutputContext, phase string, err error) string

	// RenderSubagentProgress renders subagent progress update
	RenderSubagentProgress(ctx *types.OutputContext, completed, total int, tokens, toolCalls int) string

	// RenderSubagentComplete renders subagent completion
	RenderSubagentComplete(ctx *types.OutputContext, total, success, failed int, tokens, toolCalls int) string
}

// OutputManager manages different renderers for different targets
type OutputManager struct {
	renderers map[OutputTarget]Renderer
}

// NewOutputManager creates a new output manager
func NewOutputManager() *OutputManager {
	return &OutputManager{
		renderers: make(map[OutputTarget]Renderer),
	}
}

// RegisterRenderer registers a renderer for a target
func (m *OutputManager) RegisterRenderer(renderer Renderer) {
	m.renderers[renderer.Target()] = renderer
}

// GetRenderer gets a renderer for a target
func (m *OutputManager) GetRenderer(target OutputTarget) Renderer {
	return m.renderers[target]
}

// RenderFor renders content for a specific target
func (m *OutputManager) RenderFor(target OutputTarget, renderFunc func(Renderer) string) string {
	renderer := m.GetRenderer(target)
	if renderer == nil {
		return ""
	}
	return renderFunc(renderer)
}

// CategorizeToolName returns the category for a given tool name
func CategorizeToolName(toolName string) types.ToolCategory {
	categories := map[string]types.ToolCategory{
		"file_read":            types.CategoryFile,
		"file_write":           types.CategoryFile,
		"file_edit":            types.CategoryFile,
		"list_files":           types.CategoryFile,
		"grep":                 types.CategorySearch,
		"ripgrep":              types.CategorySearch,
		"find":                 types.CategorySearch,
		"code_search":          types.CategorySearch,
		"bash":                 types.CategoryShell,
		"sandbox_shell_exec":   types.CategoryShell,
		"code_execute":         types.CategoryExecution,
		"sandbox_code_execute": types.CategoryExecution,
		"web_search":           types.CategoryWeb,
		"web_fetch":            types.CategoryWeb,
		"music_play":           types.CategoryWeb,
		"think":                types.CategoryReasoning,
		"final":                types.CategoryReasoning,
		"todo_read":            types.CategoryTask,
		"todo_update":          types.CategoryTask,
		"request_user":         types.CategoryTask,
	}
	if cat, ok := categories[toolName]; ok {
		return cat
	}
	return types.CategoryOther
}
