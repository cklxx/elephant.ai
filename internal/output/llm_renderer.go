package output

import (
	"alex/internal/agent/types"
	"alex/internal/agent/domain"
	"fmt"
	"strings"
	"time"
)

// LLMRenderer renders output for LLM consumption
// Output is complete, structured, and optimized for LLM reasoning
// Focus: Organize tool information with hierarchical context
type LLMRenderer struct{}

// NewLLMRenderer creates a new LLM renderer
func NewLLMRenderer() *LLMRenderer {
	return &LLMRenderer{}
}

// Target returns the output target
func (r *LLMRenderer) Target() OutputTarget {
	return TargetLLM
}

// RenderTaskAnalysis renders task analysis (not needed for LLM)
func (r *LLMRenderer) RenderTaskAnalysis(ctx *types.OutputContext, event *domain.TaskAnalysisEvent) string {
	// Task analysis is for user display only, LLM doesn't need it
	return ""
}

// RenderToolCallStart renders tool call start (not needed for LLM)
func (r *LLMRenderer) RenderToolCallStart(ctx *types.OutputContext, toolName string, args map[string]interface{}) string {
	// Tool call visualization is for user only
	return ""
}

// RenderToolCallComplete renders tool result for LLM with hierarchical context
func (r *LLMRenderer) RenderToolCallComplete(ctx *types.OutputContext, toolName string, result string, err error, duration time.Duration) string {
	if err != nil {
		return r.formatToolError(ctx, toolName, err)
	}
	return r.formatToolResult(ctx, toolName, result)
}

// formatToolResult organizes tool output with hierarchical context
func (r *LLMRenderer) formatToolResult(ctx *types.OutputContext, toolName string, result string) string {
	// For LLM: raw result with minimal context markers
	var parts []string

	// Add hierarchical context for subagents
	if ctx.Level == types.LevelSubagent {
		parts = append(parts, fmt.Sprintf("[Subagent %s]", ctx.AgentID))
	}

	// Add category context for better organization
	category := CategorizeToolName(toolName)
	switch category {
	case types.CategoryFile:
		// File operations: include full content
		parts = append(parts, result)
	case types.CategorySearch:
		// Search results: include all matches for LLM analysis
		parts = append(parts, result)
	case types.CategoryShell, types.CategoryExecution:
		// Shell/execution: include full output
		parts = append(parts, result)
	case types.CategoryWeb:
		// Web content: include full content
		parts = append(parts, result)
	case types.CategoryTask:
		// Task info: include full details
		parts = append(parts, result)
	case types.CategoryReasoning:
		// Reasoning: include full analysis
		parts = append(parts, result)
	default:
		// Default: raw result
		parts = append(parts, result)
	}

	return strings.Join(parts, "\n")
}

// formatToolError formats tool errors with context
func (r *LLMRenderer) formatToolError(ctx *types.OutputContext, toolName string, err error) string {
	if ctx.Level == types.LevelSubagent {
		return fmt.Sprintf("[Subagent %s] Error executing %s: %v", ctx.AgentID, toolName, err)
	}
	return fmt.Sprintf("Error executing %s: %v", toolName, err)
}

// RenderTaskComplete renders task completion for LLM
func (r *LLMRenderer) RenderTaskComplete(ctx *types.OutputContext, result *domain.TaskResult) string {
	// LLM only needs the final answer
	return result.Answer
}

// RenderError renders error for LLM
func (r *LLMRenderer) RenderError(ctx *types.OutputContext, phase string, err error) string {
	if ctx.Level == types.LevelSubagent {
		return fmt.Sprintf("[Subagent %s] Error in %s: %v", ctx.AgentID, phase, err)
	}
	return fmt.Sprintf("Error in %s: %v", phase, err)
}

// RenderSubagentProgress renders subagent progress (not needed for LLM)
func (r *LLMRenderer) RenderSubagentProgress(ctx *types.OutputContext, completed, total int, tokens, toolCalls int) string {
	// Progress is for user display only
	return ""
}

// RenderSubagentComplete renders subagent results for LLM
func (r *LLMRenderer) RenderSubagentComplete(ctx *types.OutputContext, total, success, failed int, tokens, toolCalls int) string {
	// LLM only cares about success/failure counts, not visual formatting
	return fmt.Sprintf("Subagent completed %d/%d tasks (%d failed)", success, total, failed)
}
