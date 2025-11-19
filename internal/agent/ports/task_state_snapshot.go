package ports

import "context"

type taskStateSnapshotKey struct{}

// WithTaskStateSnapshot stores a deep copy of the provided task state on the
// context so downstream tools (e.g. the subagent executor) can reuse the
// parent agent's full conversation when delegating work.
func WithTaskStateSnapshot(ctx context.Context, state *TaskState) context.Context {
	if state == nil {
		return ctx
	}
	snapshot := CloneTaskState(state)
	return context.WithValue(ctx, taskStateSnapshotKey{}, snapshot)
}

// WithClonedTaskStateSnapshot stores an already cloned snapshot on the context
// without performing another copy. Callers must ensure the provided state will
// not be mutated.
func WithClonedTaskStateSnapshot(ctx context.Context, snapshot *TaskState) context.Context {
	if snapshot == nil {
		return ctx
	}
	return context.WithValue(ctx, taskStateSnapshotKey{}, snapshot)
}

// GetTaskStateSnapshot retrieves the task state snapshot from the context. The
// returned value is a deep clone to ensure callers cannot mutate the stored
// snapshot.
func GetTaskStateSnapshot(ctx context.Context) *TaskState {
	if ctx == nil {
		return nil
	}
	snapshot, ok := ctx.Value(taskStateSnapshotKey{}).(*TaskState)
	if !ok || snapshot == nil {
		return nil
	}
	return CloneTaskState(snapshot)
}

// CloneTaskState creates a deep copy of the provided task state, including its
// conversation, attachments, plans, beliefs, and world state metadata.
func CloneTaskState(state *TaskState) *TaskState {
	if state == nil {
		return nil
	}
	cloned := &TaskState{
		SystemPrompt:           state.SystemPrompt,
		Iterations:             state.Iterations,
		TokenCount:             state.TokenCount,
		Complete:               state.Complete,
		FinalAnswer:            state.FinalAnswer,
		SessionID:              state.SessionID,
		TaskID:                 state.TaskID,
		ParentTaskID:           state.ParentTaskID,
		PendingUserAttachments: cloneAttachmentMap(state.PendingUserAttachments),
	}
	if len(state.Messages) > 0 {
		cloned.Messages = CloneMessages(state.Messages)
	}
	if len(state.ToolResults) > 0 {
		cloned.ToolResults = CloneToolResults(state.ToolResults)
	}
	if len(state.Attachments) > 0 {
		cloned.Attachments = cloneAttachmentMap(state.Attachments)
	}
	if len(state.AttachmentIterations) > 0 {
		cloned.AttachmentIterations = cloneIterationMap(state.AttachmentIterations)
	}
	if len(state.Plans) > 0 {
		cloned.Plans = ClonePlanNodes(state.Plans)
	}
	if len(state.Beliefs) > 0 {
		cloned.Beliefs = CloneBeliefs(state.Beliefs)
	}
	if len(state.KnowledgeRefs) > 0 {
		cloned.KnowledgeRefs = CloneKnowledgeReferences(state.KnowledgeRefs)
	}
	if len(state.WorldState) > 0 {
		cloned.WorldState = cloneMapAny(state.WorldState)
	}
	if len(state.WorldDiff) > 0 {
		cloned.WorldDiff = cloneMapAny(state.WorldDiff)
	}
	if len(state.FeedbackSignals) > 0 {
		cloned.FeedbackSignals = CloneFeedbackSignals(state.FeedbackSignals)
	}
	return cloned
}

// CloneMessages performs a deep copy of the provided message slice.
func CloneMessages(messages []Message) []Message {
	if len(messages) == 0 {
		return nil
	}
	cloned := make([]Message, len(messages))
	for i := range messages {
		cloned[i] = cloneMessage(messages[i])
	}
	return cloned
}

func cloneMessage(msg Message) Message {
	cloned := msg
	if len(msg.ToolCalls) > 0 {
		cloned.ToolCalls = cloneToolCalls(msg.ToolCalls)
	}
	if len(msg.ToolResults) > 0 {
		cloned.ToolResults = CloneToolResults(msg.ToolResults)
	}
	if len(msg.Metadata) > 0 {
		metadata := make(map[string]any, len(msg.Metadata))
		for key, value := range msg.Metadata {
			metadata[key] = value
		}
		cloned.Metadata = metadata
	}
	if len(msg.Attachments) > 0 {
		cloned.Attachments = cloneAttachmentMap(msg.Attachments)
	}
	return cloned
}

func cloneToolCalls(calls []ToolCall) []ToolCall {
	if len(calls) == 0 {
		return nil
	}
	cloned := make([]ToolCall, len(calls))
	for i := range calls {
		cloned[i] = calls[i]
		if len(calls[i].Arguments) > 0 {
			args := make(map[string]any, len(calls[i].Arguments))
			for key, value := range calls[i].Arguments {
				args[key] = value
			}
			cloned[i].Arguments = args
		}
	}
	return cloned
}

// CloneToolResults deep copies the provided tool results.
func CloneToolResults(results []ToolResult) []ToolResult {
	if len(results) == 0 {
		return nil
	}
	cloned := make([]ToolResult, len(results))
	for i := range results {
		cloned[i] = results[i]
		if len(results[i].Metadata) > 0 {
			metadata := make(map[string]any, len(results[i].Metadata))
			for key, value := range results[i].Metadata {
				metadata[key] = value
			}
			cloned[i].Metadata = metadata
		}
		if len(results[i].Attachments) > 0 {
			cloned[i].Attachments = cloneAttachmentMap(results[i].Attachments)
		}
	}
	return cloned
}

// ClonePlanNodes copies plan nodes with their nested slices.
func ClonePlanNodes(nodes []PlanNode) []PlanNode {
	if len(nodes) == 0 {
		return nil
	}
	cloned := make([]PlanNode, len(nodes))
	for i := range nodes {
		cloned[i] = PlanNode{
			ID:          nodes[i].ID,
			Title:       nodes[i].Title,
			Status:      nodes[i].Status,
			Description: nodes[i].Description,
		}
		if len(nodes[i].Children) > 0 {
			cloned[i].Children = ClonePlanNodes(nodes[i].Children)
		}
	}
	return cloned
}

// CloneBeliefs deep copies belief slices.
func CloneBeliefs(beliefs []Belief) []Belief {
	if len(beliefs) == 0 {
		return nil
	}
	cloned := make([]Belief, len(beliefs))
	copy(cloned, beliefs)
	return cloned
}

// CloneKnowledgeReferences deep copies knowledge references.
func CloneKnowledgeReferences(refs []KnowledgeReference) []KnowledgeReference {
	if len(refs) == 0 {
		return nil
	}
	cloned := make([]KnowledgeReference, len(refs))
	for i := range refs {
		cloned[i] = KnowledgeReference{
			ID:             refs[i].ID,
			Description:    refs[i].Description,
			SOPRefs:        append([]string(nil), refs[i].SOPRefs...),
			RAGCollections: append([]string(nil), refs[i].RAGCollections...),
			MemoryKeys:     append([]string(nil), refs[i].MemoryKeys...),
		}
	}
	return cloned
}

// CloneFeedbackSignals copies the provided feedback signals.
func CloneFeedbackSignals(signals []FeedbackSignal) []FeedbackSignal {
	if len(signals) == 0 {
		return nil
	}
	cloned := make([]FeedbackSignal, len(signals))
	copy(cloned, signals)
	return cloned
}

func cloneMapAny(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(input))
	for key, value := range input {
		cloned[key] = cloneWorldValue(value)
	}
	return cloned
}

func cloneWorldValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		return cloneMapAny(v)
	case []map[string]any:
		if len(v) == 0 {
			return nil
		}
		cloned := make([]map[string]any, len(v))
		for i := range v {
			cloned[i] = cloneMapAny(v[i])
		}
		return cloned
	case []string:
		return append([]string(nil), v...)
	case []any:
		if len(v) == 0 {
			return nil
		}
		cloned := make([]any, len(v))
		for i := range v {
			cloned[i] = cloneWorldValue(v[i])
		}
		return cloned
	default:
		return v
	}
}
