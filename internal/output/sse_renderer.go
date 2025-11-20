package output

import (
	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	"alex/internal/agent/types"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// SSERenderer renders output for SSE (Server-Sent Events) streaming
// Output is JSON-formatted events suitable for web clients
type SSERenderer struct {
	formatter *domain.ToolFormatter
}

// NewSSERenderer creates a new SSE renderer
func NewSSERenderer() *SSERenderer {
	return &SSERenderer{
		formatter: domain.NewToolFormatter(),
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

// RenderTaskAnalysis renders task analysis as SSE event with hierarchy
func (r *SSERenderer) RenderTaskAnalysis(ctx *types.OutputContext, event *domain.TaskAnalysisEvent) string {
	payload := map[string]interface{}{
		"action_name":    event.ActionName,
		"goal":           event.Goal,
		"level":          string(ctx.Level),
		"agent_id":       ctx.AgentID,
		"session_id":     ctx.SessionID,
		"task_id":        event.GetTaskID(),
		"parent_task_id": event.GetParentTaskID(),
	}
	if strings.TrimSpace(event.Approach) != "" {
		payload["approach"] = event.Approach
	}

	sseEvent := SSEEvent{
		Type:      "task_analysis",
		Timestamp: event.Timestamp(),
		Data:      payload,
	}
	return r.toSSE(sseEvent)
}

// RenderToolCallStart renders tool call start as SSE event with hierarchy
func (r *SSERenderer) RenderToolCallStart(ctx *types.OutputContext, toolName string, args map[string]interface{}) string {
	presentation := r.formatter.PrepareArgs(toolName, args)

	sseEvent := SSEEvent{
		Type:      "tool_call_start",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"tool":           toolName,
			"args":           args,
			"category":       string(CategorizeToolName(toolName)),
			"level":          string(ctx.Level),
			"agent_id":       ctx.AgentID,
			"session_id":     ctx.SessionID,
			"task_id":        ctx.TaskID,
			"parent_task_id": ctx.ParentTaskID,
		},
	}

	if len(presentation.Args) > 0 {
		sseEvent.Data["arguments"] = presentation.Args
	}
	if presentation.InlinePreview != "" {
		sseEvent.Data["arguments_preview"] = presentation.InlinePreview
	}

	return r.toSSE(sseEvent)
}

// RenderToolCallComplete renders tool call completion as SSE event with hierarchy
func (r *SSERenderer) RenderToolCallComplete(ctx *types.OutputContext, toolName string, result string, err error, duration time.Duration) string {
	data := map[string]interface{}{
		"tool":           toolName,
		"category":       string(CategorizeToolName(toolName)),
		"duration":       duration.Milliseconds(),
		"level":          string(ctx.Level),
		"agent_id":       ctx.AgentID,
		"session_id":     ctx.SessionID,
		"task_id":        ctx.TaskID,
		"parent_task_id": ctx.ParentTaskID,
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
			"answer":         result.Answer,
			"iterations":     result.Iterations,
			"tokens":         result.TokensUsed,
			"stop_reason":    result.StopReason,
			"level":          string(ctx.Level),
			"agent_id":       ctx.AgentID,
			"session_id":     ctx.SessionID,
			"task_id":        ctx.TaskID,
			"parent_task_id": ctx.ParentTaskID,
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
			"phase":          phase,
			"error":          err.Error(),
			"level":          string(ctx.Level),
			"agent_id":       ctx.AgentID,
			"session_id":     ctx.SessionID,
			"task_id":        ctx.TaskID,
			"parent_task_id": ctx.ParentTaskID,
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
			"completed":      completed,
			"total":          total,
			"tokens":         tokens,
			"tool_calls":     toolCalls,
			"agent_id":       ctx.AgentID,
			"session_id":     ctx.SessionID,
			"task_id":        ctx.TaskID,
			"parent_task_id": ctx.ParentTaskID,
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
			"total":          total,
			"success":        success,
			"failed":         failed,
			"tokens":         tokens,
			"tool_calls":     toolCalls,
			"agent_id":       ctx.AgentID,
			"session_id":     ctx.SessionID,
			"task_id":        ctx.TaskID,
			"parent_task_id": ctx.ParentTaskID,
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

func cloneStepsForSSE(steps []ports.TaskAnalysisStep) []map[string]any {
	if len(steps) == 0 {
		return nil
	}
	cloned := make([]map[string]any, 0, len(steps))
	for _, step := range steps {
		if strings.TrimSpace(step.Description) == "" {
			continue
		}
		entry := map[string]any{
			"description": step.Description,
		}
		if strings.TrimSpace(step.Rationale) != "" {
			entry["rationale"] = step.Rationale
		}
		if step.NeedsExternalContext {
			entry["needs_external_context"] = true
		}
		cloned = append(cloned, entry)
	}
	if len(cloned) == 0 {
		return nil
	}
	return cloned
}

func cloneRetrievalForSSE(plan ports.TaskRetrievalPlan) map[string]any {
	hasQueries := len(plan.LocalQueries) > 0 || len(plan.SearchQueries) > 0 || len(plan.CrawlURLs) > 0 || len(plan.KnowledgeGaps) > 0
	if !plan.ShouldRetrieve && !hasQueries && strings.TrimSpace(plan.Notes) == "" {
		return nil
	}

	payload := map[string]any{
		"should_retrieve": plan.ShouldRetrieve,
	}
	if len(plan.LocalQueries) > 0 {
		payload["local_queries"] = append([]string(nil), plan.LocalQueries...)
	}
	if len(plan.SearchQueries) > 0 {
		payload["search_queries"] = append([]string(nil), plan.SearchQueries...)
	}
	if len(plan.CrawlURLs) > 0 {
		payload["crawl_urls"] = append([]string(nil), plan.CrawlURLs...)
	}
	if len(plan.KnowledgeGaps) > 0 {
		payload["knowledge_gaps"] = append([]string(nil), plan.KnowledgeGaps...)
	}
	if strings.TrimSpace(plan.Notes) != "" {
		payload["notes"] = plan.Notes
	}
	if !plan.ShouldRetrieve && hasQueries {
		payload["should_retrieve"] = true
	}
	return payload
}
