package react

import (
	"context"
	"strings"
	"time"

	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
)

func (e *ReactEngine) observeToolResults(ctx context.Context, state *TaskState, iteration int, results []ToolResult) {
	if state == nil || len(results) == 0 {
		return
	}
	ensureWorldStateMap(state)
	updates := make([]map[string]any, 0, len(results))
	for _, result := range results {
		updates = append(updates, summarizeToolResultForWorld(result))
	}
	state.WorldState["last_tool_results"] = updates
	state.WorldState["last_iteration"] = iteration
	state.WorldState["last_updated_at"] = e.clock.Now().Format(time.RFC3339)
	state.WorldDiff = map[string]any{
		"iteration":    iteration,
		"tool_results": updates,
	}
	e.compactToolCallHistory(state, results)
	e.compactToolResultAttachments(ctx, state, results)
	offloadMessageAttachmentData(state)
	e.appendFeedbackSignals(state, results)
}

func (e *ReactEngine) compactToolResultAttachments(ctx context.Context, state *TaskState, results []ToolResult) {
	if state == nil {
		return
	}
	e.persistToolResultAttachments(ctx, results)
	e.persistToolResultAttachments(ctx, state.ToolResults)
	offloadToolResultAttachmentData(results)
	offloadToolResultAttachmentData(state.ToolResults)
	for idx := range state.Messages {
		msg := &state.Messages[idx]
		for j := range msg.ToolResults {
			offloadAttachmentMap(msg.ToolResults[j].Attachments)
		}
	}
}

func (e *ReactEngine) persistToolResultAttachments(ctx context.Context, results []ToolResult) {
	if e.attachmentPersister == nil || len(results) == 0 {
		return
	}
	for i := range results {
		if len(results[i].Attachments) == 0 {
			continue
		}
		for name, att := range results[i].Attachments {
			results[i].Attachments[name] = persistAttachmentIfNeeded(ctx, att, e.attachmentPersister)
		}
	}
}

func offloadToolResultAttachmentData(results []ToolResult) {
	if len(results) == 0 {
		return
	}
	for i := range results {
		offloadAttachmentMap(results[i].Attachments)
	}
}

func (e *ReactEngine) appendFeedbackSignals(state *TaskState, results []ToolResult) {
	if state == nil || len(results) == 0 {
		return
	}
	now := e.clock.Now()
	for _, result := range results {
		signal := agent.FeedbackSignal{
			Kind:      "tool_result",
			Message:   buildFeedbackMessage(result),
			Value:     deriveFeedbackValue(result),
			CreatedAt: now,
		}
		state.FeedbackSignals = append(state.FeedbackSignals, signal)
	}
	if len(state.FeedbackSignals) > maxFeedbackSignals {
		state.FeedbackSignals = state.FeedbackSignals[len(state.FeedbackSignals)-maxFeedbackSignals:]
	}
}

// offloadMessageAttachmentData strips inline Data from message attachments
// (both Message.Attachments and embedded ToolResult.Attachments) when a
// durable URI already exists. This prevents base64 blobs from persisting in
// the context window after L6 has already stored them.
func offloadMessageAttachmentData(state *TaskState) {
	if state == nil {
		return
	}
	for idx := range state.Messages {
		msg := &state.Messages[idx]
		if offloadAttachmentMap(msg.Attachments) {
			state.Messages[idx] = *msg
		}
		for i := range msg.ToolResults {
			offloadAttachmentMap(msg.ToolResults[i].Attachments)
		}
	}
}

// offloadAttachmentMap clears the Data field from attachments that already
// have a non-data-URI external reference. Returns true if any field changed.
func offloadAttachmentMap(atts map[string]ports.Attachment) bool {
	changed := false
	for key, att := range atts {
		if att.Data == "" {
			continue
		}
		uri := strings.TrimSpace(att.URI)
		if uri == "" || strings.HasPrefix(strings.ToLower(uri), "data:") {
			continue
		}
		att.Data = ""
		atts[key] = att
		changed = true
	}
	return changed
}

func (e *ReactEngine) compactToolCallHistory(state *TaskState, results []ToolResult) {
	if state == nil || len(results) == 0 {
		return
	}
	resultMap := make(map[string]ToolResult, len(results))
	for _, result := range results {
		if result.CallID == "" {
			continue
		}
		resultMap[result.CallID] = result
	}
	if len(resultMap) == 0 {
		return
	}

	for idx := range state.Messages {
		msg := state.Messages[idx]
		if len(msg.ToolCalls) == 0 {
			continue
		}
		changed := false
		for i, call := range msg.ToolCalls {
			result, ok := resultMap[call.ID]
			if !ok {
				continue
			}
			if compacted, updated := compactToolCallArguments(call, result); updated {
				call.Arguments = compacted
				msg.ToolCalls[i] = call
				changed = true
			}
		}
		if changed {
			state.Messages[idx] = msg
		}
	}
}
