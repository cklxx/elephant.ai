package react

import (
	"fmt"
	"strings"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
)

func newReactWorkflow(tracker WorkflowTracker) *reactWorkflow {
	return &reactWorkflow{tracker: tracker}
}

func (rw *reactWorkflow) ensure(nodeID string, input any) string {
	if rw.tracker == nil || nodeID == "" {
		return ""
	}
	rw.tracker.EnsureNode(nodeID, input)
	return nodeID
}

func (rw *reactWorkflow) start(nodeID string, input any) {
	if rw.ensure(nodeID, input) == "" {
		return
	}
	rw.tracker.StartNode(nodeID)
}

func (rw *reactWorkflow) complete(nodeID string, output any, err error) {
	if rw.tracker == nil || nodeID == "" {
		return
	}
	if err != nil {
		rw.tracker.CompleteNodeFailure(nodeID, err)
		return
	}
	rw.tracker.CompleteNodeSuccess(nodeID, output)
}

func (rw *reactWorkflow) startContext(task string) {
	rw.start(workflowNodeContext, map[string]any{"task": task})
}

func (rw *reactWorkflow) completeContext(output map[string]any) {
	rw.complete(workflowNodeContext, output, nil)
}

func (rw *reactWorkflow) startThink(iteration int) {
	rw.start(iterationThinkNode(iteration), map[string]any{"iteration": iteration})
}

func (rw *reactWorkflow) startPlan(iteration, requested int) {
	rw.start(iterationPlanNode(iteration), map[string]any{"iteration": iteration, "requested_calls": requested})
}

func (rw *reactWorkflow) completePlan(iteration int, planned []ToolCall, err error) {
	rw.complete(iterationPlanNode(iteration), workflowPlanOutput(iteration, planned), err)
}

func (rw *reactWorkflow) completeThink(iteration int, thought Message, toolCalls []ToolCall, err error) {
	rw.complete(iterationThinkNode(iteration), workflowThinkOutput(iteration, thought, toolCalls), err)
}

func (rw *reactWorkflow) startTools(iteration int, nodeID string, calls int) {
	rw.start(nodeID, map[string]any{"iteration": iteration, "calls": calls})
}

func (rw *reactWorkflow) completeTools(iteration int, nodeID string, results []ToolResult, err error) {
	rw.complete(nodeID, workflowToolOutput(iteration, results), err)
}

func (rw *reactWorkflow) ensureToolCall(iteration int, call ToolCall) string {
	return rw.ensure(iterationToolCallNode(iteration, call.ID), workflowToolCallInput(iteration, call))
}

func (rw *reactWorkflow) startToolCall(nodeID string) {
	rw.start(nodeID, nil)
}

func (rw *reactWorkflow) completeToolCall(nodeID string, iteration int, call ToolCall, result ToolResult, err error) {
	rw.complete(nodeID, workflowToolCallOutput(iteration, call, result), err)
}

func (rw *reactWorkflow) finalize(stopReason string, result *TaskResult, err error) {
	rw.start(workflowNodeFinalize, map[string]any{"stop_reason": stopReason})
	rw.complete(workflowNodeFinalize, workflowFinalizeOutput(result), err)
}

func iterationThinkNode(iteration int) string {
	return fmt.Sprintf("react:iter:%d:think", iteration)
}

func iterationPlanNode(iteration int) string {
	return fmt.Sprintf("react:iter:%d:plan", iteration)
}

func iterationToolsNode(iteration int) string {
	return fmt.Sprintf("react:iter:%d:tools", iteration)
}

func iterationToolCallNode(iteration int, callID string) string {
	return fmt.Sprintf("react:iter:%d:tool:%s", iteration, callID)
}

func workflowContextOutput(state *TaskState) map[string]any {
	if state == nil {
		return nil
	}

	snapshot := agent.CloneTaskState(state)
	if snapshot == nil {
		return nil
	}

	pending := snapshot.PendingUserAttachments
	if pending == nil && len(snapshot.Attachments) > 0 {
		pending = snapshot.Attachments
	}

	return map[string]any{
		"messages":              snapshot.Messages,
		"attachments":           snapshot.Attachments,
		"pending_attachments":   pending,
		"iteration":             snapshot.Iterations,
		"token_count":           snapshot.TokenCount,
		"attachment_iterations": snapshot.AttachmentIterations,
	}
}

func workflowThinkOutput(iteration int, thought Message, toolCalls []ToolCall) map[string]any {
	output := map[string]any{
		"iteration":  iteration,
		"tool_calls": len(toolCalls),
	}

	if trimmed := strings.TrimSpace(thought.Content); trimmed != "" {
		output["content"] = trimmed
	}
	if len(thought.Attachments) > 0 {
		output["attachments"] = ports.CloneAttachmentMap(thought.Attachments)
	}

	return output
}

func workflowPlanOutput(iteration int, toolCalls []ToolCall) map[string]any {
	output := map[string]any{"iteration": iteration}
	if len(toolCalls) == 0 {
		return output
	}
	output["tool_calls"] = len(toolCalls)
	names := make([]string, 0, len(toolCalls))
	for _, call := range toolCalls {
		if call.Name != "" {
			names = append(names, call.Name)
		}
	}
	if len(names) > 0 {
		output["tools"] = names
	}
	return output
}

func workflowToolCallInput(iteration int, call ToolCall) map[string]any {
	input := map[string]any{
		"iteration": iteration,
		"call_id":   call.ID,
		"tool":      call.Name,
	}

	if len(call.Arguments) > 0 {
		args := make(map[string]any, len(call.Arguments))
		for k, v := range call.Arguments {
			args[k] = v
		}
		input["arguments"] = args
	}

	return input
}

func workflowToolCallOutput(iteration int, call ToolCall, result ToolResult) map[string]any {
	output := map[string]any{
		"iteration": iteration,
		"call_id":   call.ID,
		"tool":      call.Name,
	}

	cloned := agent.CloneToolResults([]ToolResult{result})
	if len(cloned) > 0 {
		output["result"] = cloned[0]
	}

	return output
}

func workflowToolOutput(iteration int, results []ToolResult) map[string]any {
	output := map[string]any{
		"iteration": iteration,
	}

	if len(results) > 0 {
		output["results"] = agent.CloneToolResults(results)
	}

	successes := 0
	failures := 0
	for _, result := range results {
		if result.Error != nil {
			failures++
			continue
		}
		successes++
	}

	output["success"] = successes
	output["failed"] = failures

	return output
}

func workflowFinalizeOutput(result *TaskResult) map[string]any {
	if result == nil {
		return map[string]any{"stop_reason": "error"}
	}

	output := map[string]any{
		"stop_reason": result.StopReason,
		"iterations":  result.Iterations,
		"tokens_used": result.TokensUsed,
	}

	if trimmed := strings.TrimSpace(result.Answer); trimmed != "" {
		output["answer_preview"] = trimmed
	}
	if len(result.Messages) > 0 {
		output["messages"] = agent.CloneMessages(result.Messages)
	}

	return output
}
