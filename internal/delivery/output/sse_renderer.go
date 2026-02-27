package output

import (
	"encoding/json"
	"fmt"
	"time"

	"alex/internal/delivery/presentation/formatter"
	"alex/internal/domain/agent"
	"alex/internal/domain/agent/types"
)

// SSERenderer renders output for SSE (Server-Sent Events) streaming
// Output is JSON-formatted events suitable for web clients
type SSERenderer struct {
	formatter *formatter.ToolFormatter
}

// NewSSERenderer creates a new SSE renderer
func NewSSERenderer() *SSERenderer {
	return &SSERenderer{
		formatter: formatter.NewToolFormatter(),
	}
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

// contextData builds the common hierarchy fields shared by all SSE events.
func contextData(ctx *types.OutputContext) map[string]interface{} {
	return map[string]interface{}{
		"level":         string(ctx.Level),
		"agent_id":      ctx.AgentID,
		"session_id":    ctx.SessionID,
		"run_id":        ctx.TaskID,
		"parent_run_id": ctx.ParentTaskID,
	}
}

// RenderToolCallStart renders tool call start as SSE event with hierarchy
func (r *SSERenderer) RenderToolCallStart(ctx *types.OutputContext, toolName string, args map[string]interface{}) string {
	presentation := r.formatter.PrepareArgs(toolName, args)
	data := contextData(ctx)
	data["tool"] = toolName
	data["args"] = args
	data["category"] = string(CategorizeToolName(toolName))
	if len(presentation.Args) > 0 {
		data["arguments"] = presentation.Args
	}
	if presentation.InlinePreview != "" {
		data["arguments_preview"] = presentation.InlinePreview
	}
	return r.toSSE(SSEEvent{Type: "workflow.tool.started", Timestamp: time.Now(), Data: data})
}

// RenderToolCallComplete renders tool call completion as SSE event with hierarchy
func (r *SSERenderer) RenderToolCallComplete(ctx *types.OutputContext, toolName string, result string, err error, duration time.Duration) string {
	data := contextData(ctx)
	data["tool"] = toolName
	data["category"] = string(CategorizeToolName(toolName))
	data["duration"] = duration.Milliseconds()
	if err != nil {
		data["error"] = err.Error()
	} else {
		data["result"] = result
	}
	return r.toSSE(SSEEvent{Type: "workflow.tool.completed", Timestamp: time.Now(), Data: data})
}

// RenderTaskComplete renders task completion as SSE event
func (r *SSERenderer) RenderTaskComplete(ctx *types.OutputContext, result *domain.TaskResult) string {
	data := contextData(ctx)
	data["answer"] = result.Answer
	data["iterations"] = result.Iterations
	data["tokens"] = result.TokensUsed
	data["stop_reason"] = result.StopReason
	return r.toSSE(SSEEvent{Type: "workflow.result.final", Timestamp: time.Now(), Data: data})
}

// RenderError renders error as SSE event with hierarchy
func (r *SSERenderer) RenderError(ctx *types.OutputContext, phase string, err error) string {
	data := contextData(ctx)
	data["phase"] = phase
	data["error"] = err.Error()
	return r.toSSE(SSEEvent{Type: "workflow.node.failed", Timestamp: time.Now(), Data: data})
}

// RenderSubagentProgress renders subagent progress as SSE event
func (r *SSERenderer) RenderSubagentProgress(ctx *types.OutputContext, completed, total int, tokens, toolCalls int) string {
	data := contextData(ctx)
	data["completed"] = completed
	data["total"] = total
	data["tokens"] = tokens
	data["tool_calls"] = toolCalls
	return r.toSSE(SSEEvent{Type: "workflow.subflow.progress", Timestamp: time.Now(), Data: data})
}

// RenderSubagentComplete renders subagent completion as SSE event
func (r *SSERenderer) RenderSubagentComplete(ctx *types.OutputContext, total, success, failed int, tokens, toolCalls int) string {
	data := contextData(ctx)
	data["total"] = total
	data["success"] = success
	data["failed"] = failed
	data["tokens"] = tokens
	data["tool_calls"] = toolCalls
	return r.toSSE(SSEEvent{Type: "workflow.subflow.completed", Timestamp: time.Now(), Data: data})
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
