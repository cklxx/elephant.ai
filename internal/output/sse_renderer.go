package output

import (
	"alex/internal/agent/domain"
	"alex/internal/agent/types"
	"encoding/json"
	"fmt"
	"time"
)

// SSERenderer renders output for SSE (Server-Sent Events) streaming
// Output is JSON-formatted events suitable for web clients
type SSERenderer struct{}

// NewSSERenderer creates a new SSE renderer
func NewSSERenderer() *SSERenderer {
	return &SSERenderer{}
}

// Target returns the output target
func (r *SSERenderer) Target() OutputTarget {
	return TargetSSE
}

// SSEEvent represents a server-sent event
type SSEEvent struct {
	Type      string                 `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// RenderTaskAnalysis renders task analysis as SSE event with hierarchy
func (r *SSERenderer) RenderTaskAnalysis(ctx *types.OutputContext, event *domain.TaskAnalysisEvent) string {
	sseEvent := SSEEvent{
		Type:      "task_analysis",
		Timestamp: event.Timestamp(),
		Data: map[string]interface{}{
			"action":   event.ActionName,
			"goal":     event.Goal,
			"level":    string(ctx.Level),
			"agent_id": ctx.AgentID,
		},
	}
	return r.toSSE(sseEvent)
}

// RenderToolCallStart renders tool call start as SSE event with hierarchy
func (r *SSERenderer) RenderToolCallStart(ctx *types.OutputContext, toolName string, args map[string]interface{}) string {
	sseEvent := SSEEvent{
		Type:      "tool_call_start",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"tool":     toolName,
			"args":     args,
			"category": string(CategorizeToolName(toolName)),
			"level":    string(ctx.Level),
			"agent_id": ctx.AgentID,
		},
	}
	return r.toSSE(sseEvent)
}

// RenderToolCallComplete renders tool call completion as SSE event with hierarchy
func (r *SSERenderer) RenderToolCallComplete(ctx *types.OutputContext, toolName string, result string, err error, duration time.Duration) string {
	data := map[string]interface{}{
		"tool":     toolName,
		"category": string(CategorizeToolName(toolName)),
		"duration": duration.Milliseconds(),
		"level":    string(ctx.Level),
		"agent_id": ctx.AgentID,
	}

	if err != nil {
		data["error"] = err.Error()
	} else {
		data["result"] = result
	}

	sseEvent := SSEEvent{
		Type:      "tool_call_complete",
		Timestamp: time.Now(),
		Data:      data,
	}
	return r.toSSE(sseEvent)
}

// RenderTaskComplete renders task completion as SSE event
func (r *SSERenderer) RenderTaskComplete(ctx *types.OutputContext, result *domain.TaskResult) string {
	sseEvent := SSEEvent{
		Type:      "task_complete",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"answer":      result.Answer,
			"iterations":  result.Iterations,
			"tokens":      result.TokensUsed,
			"stop_reason": result.StopReason,
			"level":       string(ctx.Level),
			"agent_id":    ctx.AgentID,
		},
	}
	return r.toSSE(sseEvent)
}

// RenderError renders error as SSE event with hierarchy
func (r *SSERenderer) RenderError(ctx *types.OutputContext, phase string, err error) string {
	sseEvent := SSEEvent{
		Type:      "error",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"phase":    phase,
			"error":    err.Error(),
			"level":    string(ctx.Level),
			"agent_id": ctx.AgentID,
		},
	}
	return r.toSSE(sseEvent)
}

// RenderSubagentProgress renders subagent progress as SSE event
func (r *SSERenderer) RenderSubagentProgress(ctx *types.OutputContext, completed, total int, tokens, toolCalls int) string {
	sseEvent := SSEEvent{
		Type:      "subagent_progress",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"completed":  completed,
			"total":      total,
			"tokens":     tokens,
			"tool_calls": toolCalls,
			"agent_id":   ctx.AgentID,
		},
	}
	return r.toSSE(sseEvent)
}

// RenderSubagentComplete renders subagent completion as SSE event
func (r *SSERenderer) RenderSubagentComplete(ctx *types.OutputContext, total, success, failed int, tokens, toolCalls int) string {
	sseEvent := SSEEvent{
		Type:      "subagent_complete",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"total":      total,
			"success":    success,
			"failed":     failed,
			"tokens":     tokens,
			"tool_calls": toolCalls,
			"agent_id":   ctx.AgentID,
		},
	}
	return r.toSSE(sseEvent)
}

// toSSE converts an SSEEvent to SSE format
func (r *SSERenderer) toSSE(event SSEEvent) string {
	jsonData, err := json.Marshal(event)
	if err != nil {
		return ""
	}
	// SSE format: data: <json>\n\n
	return fmt.Sprintf("data: %s\n\n", string(jsonData))
}
