package domain

import (
	"context"
	"fmt"
	"sync"
	"time"

	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	tools "alex/internal/agent/ports/tools"
)

func newToolCallBatch(
	engine *ReactEngine,
	ctx context.Context,
	state *TaskState,
	iteration int,
	calls []ToolCall,
	registry tools.ToolRegistry,
	limiter tools.ToolExecutionLimiter,
	tracker *reactWorkflow,
) *toolCallBatch {
	expanded := make([]ToolCall, len(calls))
	subagentSnapshots := make([]*agent.TaskState, len(calls))
	for i, call := range calls {
		tc := call
		tc.Arguments = engine.expandToolCallArguments(tc.Name, tc.Arguments, state)
		expanded[i] = tc

		if tc.Name == "subagent" {
			subagentSnapshots[i] = buildSubagentStateSnapshot(state, tc)
		}
	}

	var nodes []string
	if tracker != nil {
		nodes = make([]string, len(expanded))
		for i, call := range expanded {
			nodes[i] = tracker.ensureToolCall(iteration, call)
		}
	}

	attachmentsSnapshot, iterationSnapshot := snapshotAttachments(state)

	return &toolCallBatch{
		engine:               engine,
		ctx:                  ctx,
		state:                state,
		iteration:            iteration,
		registry:             registry,
		limiter:              limiter,
		tracker:              tracker,
		attachments:          attachmentsSnapshot,
		attachmentIterations: iterationSnapshot,
		subagentSnapshots:    subagentSnapshots,
		calls:                expanded,
		callNodes:            nodes,
	}
}

func (b *toolCallBatch) execute() []ToolResult {
	b.results = make([]ToolResult, len(b.calls))
	if len(b.calls) == 0 {
		return b.results
	}
	limit := 1
	if b.limiter != nil && b.limiter.Limit() > 0 {
		limit = b.limiter.Limit()
	}
	if limit <= 1 || len(b.calls) == 1 {
		for i, call := range b.calls {
			b.runCall(i, call)
		}
		return b.results
	}

	jobs := make(chan int)
	var wg sync.WaitGroup
	wg.Add(len(b.calls))

	for i := 0; i < limit; i++ {
		go func() {
			for idx := range jobs {
				b.runCall(idx, b.calls[idx])
				wg.Done()
			}
		}()
	}

	for i := range b.calls {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	return b.results
}

func (b *toolCallBatch) runCall(idx int, tc ToolCall) {
	tc.SessionID = b.state.SessionID
	tc.TaskID = b.state.TaskID
	tc.ParentTaskID = b.state.ParentTaskID

	nodeID := ""
	if b.tracker != nil {
		nodeID = b.callNodes[idx]
		if nodeID == "" {
			nodeID = b.tracker.ensureToolCall(b.iteration, tc)
		}
		b.tracker.startToolCall(nodeID)
	}

	startTime := b.engine.clock.Now()

	b.engine.logger.Debug("Tool %d: Getting tool '%s' from registry", idx, tc.Name)
	tool, err := b.registry.Get(tc.Name)
	if err != nil {
		missing := fmt.Errorf("tool not found: %s", tc.Name)
		b.finalize(idx, tc, nodeID, ToolResult{Error: missing}, startTime)
		return
	}

	toolCtx := tools.WithAttachmentContext(b.ctx, b.attachments, b.attachmentIterations)
	toolCtx = tools.WithToolProgressEmitter(toolCtx, func(chunk string, isComplete bool) {
		if chunk == "" && !isComplete {
			return
		}
		b.engine.emitEvent(&WorkflowToolProgressEvent{
			BaseEvent:  b.engine.newBaseEvent(b.ctx, b.state.SessionID, b.state.TaskID, b.state.ParentTaskID),
			CallID:     tc.ID,
			Chunk:      chunk,
			IsComplete: isComplete,
		})
	})
	if tc.Name == "subagent" {
		if snapshot := b.subagentSnapshots[idx]; snapshot != nil {
			toolCtx = agent.WithClonedTaskStateSnapshot(toolCtx, snapshot)
		}
	}
	if tc.Name == "acp_executor" {
		if snapshot := buildExecutorStateSnapshot(b.state, tc); snapshot != nil {
			toolCtx = agent.WithClonedTaskStateSnapshot(toolCtx, snapshot)
		}
	}

	formattedArgs := formatToolArgumentsForLog(tc.Arguments)
	b.engine.logger.Debug("Tool %d: Executing '%s' with args: %s", idx, tc.Name, formattedArgs)
	result, execErr := tool.Execute(toolCtx, ports.ToolCall(tc))
	if execErr != nil {
		b.finalize(idx, tc, nodeID, ToolResult{Error: execErr}, startTime)
		return
	}

	if result == nil {
		b.finalize(idx, tc, nodeID, ToolResult{Error: fmt.Errorf("tool %s returned no result", tc.Name)}, startTime)
		return
	}

	result.Attachments = b.engine.applyToolAttachmentMutations(
		b.ctx,
		b.state,
		tc,
		result.Attachments,
		result.Metadata,
		&b.attachmentsMu,
	)
	if len(result.Metadata) > 0 {
		b.stateMu.Lock()
		b.engine.applyImportantNotes(b.state, tc, result.Metadata)
		b.stateMu.Unlock()
	}

	b.finalize(idx, tc, nodeID, *result, startTime)
}

func (b *toolCallBatch) finalize(idx int, tc ToolCall, nodeID string, result ToolResult, startTime time.Time) {
	normalized := b.engine.normalizeToolResult(tc, b.state, result)
	b.results[idx] = normalized

	duration := b.engine.clock.Now().Sub(startTime)
	b.engine.emitWorkflowToolCompletedEvent(b.ctx, b.state, tc, normalized, duration)

	if b.tracker != nil {
		b.tracker.completeToolCall(nodeID, b.iteration, tc, normalized, normalized.Error)
	}
}
