package react

import (
	"context"
	"fmt"
	"strings"
	"time"

	domain "alex/internal/domain/agent"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/shared/utils"
)

func (m *BackgroundTaskManager) captureProgress(ctx context.Context, bt *backgroundTask, p agent.ExternalAgentProgress) {
	now := m.clock.Now()
	shouldEmit := false
	bt.mu.Lock()
	bt.progress = &p
	bt.lastActivityAt = now
	if bt.emitEvent != nil && bt.baseEvent != nil {
		if bt.lastProgressEmit.IsZero() || now.Sub(bt.lastProgressEmit) >= 2*time.Second {
			bt.lastProgressEmit = now
			shouldEmit = true
		}
	}
	bt.mu.Unlock()

	if shouldEmit {
		elapsed := time.Duration(0)
		if !bt.startedAt.IsZero() {
			elapsed = now.Sub(bt.startedAt)
			if elapsed < 0 {
				elapsed = 0
			}
		}
		bt.emitEvent(domain.NewExternalAgentProgressEvent(
			bt.baseEvent(ctx),
			bt.id, bt.agentType, p.Iteration, p.MaxIter, p.TokensUsed, p.CostUSD,
			p.CurrentTool, p.CurrentArgs, append([]string(nil), p.FilesTouched...), p.LastActivity, elapsed,
		))
	}
}

const heartbeatInterval = 5 * time.Minute

// runHeartbeat emits lightweight progress events at heartbeatInterval to keep
// the SerializingEventListener queue alive during long idle periods (e.g. a
// bash command running for >10 minutes with no JSONL output).
func (m *BackgroundTaskManager) runHeartbeat(ctx context.Context, bt *backgroundTask) {
	// Snapshot immutable fields once to avoid reading bt fields without lock.
	bt.mu.Lock()
	taskID := bt.id
	agentType := bt.agentType
	startedAt := bt.startedAt
	bt.mu.Unlock()

	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			now := m.clock.Now()
			bt.mu.Lock()
			bt.lastActivityAt = now
			bt.mu.Unlock()
			elapsed := time.Duration(0)
			if !startedAt.IsZero() {
				elapsed = now.Sub(startedAt)
				if elapsed < 0 {
					elapsed = 0
				}
			}
			bt.emitEvent(domain.NewExternalAgentProgressEvent(
				bt.baseEvent(ctx),
				taskID, agentType, 0, 0, 0, 0,
				"__heartbeat__", "", nil, time.Time{}, elapsed,
			))
		case <-ctx.Done():
			return
		}
	}
}

func (m *BackgroundTaskManager) emitCompletionEvent(ctx context.Context, bt *backgroundTask) {
	if bt.emitEvent == nil || bt.baseEvent == nil {
		return
	}

	bt.mu.Lock()
	description := bt.description
	status := bt.status
	startedAt := bt.startedAt
	completedAt := bt.completedAt
	mergeStatus := bt.mergeStatus
	answer := ""
	iterations := 0
	tokensUsed := 0
	if bt.result != nil {
		answer = bt.result.Answer
		iterations = bt.result.Iterations
		tokensUsed = bt.result.TokensUsed
	}
	errMsg := ""
	if bt.err != nil {
		errMsg = bt.err.Error()
	}
	bt.mu.Unlock()

	duration := time.Duration(0)
	if !startedAt.IsZero() && !completedAt.IsZero() {
		duration = completedAt.Sub(startedAt)
		if duration < 0 {
			duration = 0
		}
	}

	completedEvent := domain.NewBackgroundTaskCompletedEvent(
		bt.baseEvent(ctx),
		bt.id, description, string(status), answer, errMsg, duration, iterations, tokensUsed,
	)
	completedEvent.Data.MergeStatus = mergeStatus

	// 1. Normal chain (may fail if SerializingEventListener queue timed out).
	bt.emitEvent(completedEvent)

	// 2. Direct parent notification — bypasses SerializingEventListener so
	//    completion is delivered even when the queue has been idle-closed.
	if bt.notifyParent != nil {
		bt.notifyParent(completedEvent)
	}
}

func (m *BackgroundTaskManager) signalCompletion(taskID string) {
	m.mu.RLock()
	bt := m.tasks[taskID]
	m.mu.RUnlock()
	if bt != nil {
		bt.mu.Lock()
		bt.completionSignaled = true
		bt.mu.Unlock()
	}

	m.completedMu.Lock()
	m.completedIDs = append(m.completedIDs, taskID)
	m.completedMu.Unlock()

	m.activeTasks.Add(-1)
	m.notifyDependencyWaiters()
}

// notifyDependencyWaiters wakes all goroutines blocked in awaitDependencies or awaitTasks.
func (m *BackgroundTaskManager) notifyDependencyWaiters() {
	m.depMu.Lock()
	ch := m.depNotify
	m.depNotify = make(chan struct{})
	m.depMu.Unlock()
	close(ch)
}

// depWaitChan returns the current dependency notification channel.
func (m *BackgroundTaskManager) depWaitChan() <-chan struct{} {
	m.depMu.Lock()
	ch := m.depNotify
	m.depMu.Unlock()
	return ch
}

func (m *BackgroundTaskManager) buildContextEnrichedPrompt(bt *backgroundTask) string {
	if len(bt.dependsOn) == 0 {
		return bt.prompt
	}
	var sb strings.Builder
	sb.WriteString("[Collaboration Context]\n")
	sb.WriteString("This task depends on completed tasks whose results are provided below.\n\n")
	for _, depID := range bt.dependsOn {
		m.mu.RLock()
		dep := m.tasks[depID]
		m.mu.RUnlock()
		if dep == nil {
			continue
		}
		dep.mu.Lock()
		status := dep.status
		answer := ""
		if dep.result != nil {
			answer = dep.result.Answer
		}
		errMsg := ""
		if dep.err != nil {
			errMsg = dep.err.Error()
		}
		dep.mu.Unlock()

		sb.WriteString(fmt.Sprintf("--- Task %q (%s) — %s ---\n", depID, dep.agentType, strings.ToUpper(string(status))))
		if answer != "" {
			sb.WriteString("Result summary: ")
			sb.WriteString(answer)
			sb.WriteString("\n")
		}
		if errMsg != "" {
			sb.WriteString("Error: ")
			sb.WriteString(errMsg)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}
	sb.WriteString("[Your Task]\n")
	sb.WriteString(bt.prompt)
	return sb.String()
}

func (m *BackgroundTaskManager) awaitDependencies(ctx context.Context, bt *backgroundTask) error {
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		allDone := true
		for _, depID := range bt.dependsOn {
			m.mu.RLock()
			dep := m.tasks[depID]
			m.mu.RUnlock()
			if dep == nil {
				return fmt.Errorf("dependency %q not found", depID)
			}
			dep.mu.Lock()
			status := dep.status
			errMsg := ""
			if dep.err != nil {
				errMsg = dep.err.Error()
			}
			dep.mu.Unlock()

			switch status {
			case agent.BackgroundTaskStatusCompleted:
				// ok
			case agent.BackgroundTaskStatusFailed, agent.BackgroundTaskStatusCancelled:
				if errMsg == "" {
					errMsg = "dependency failed"
				}
				return fmt.Errorf("dependency %q failed: %s", depID, errMsg)
			default:
				allDone = false
			}
			if !allDone {
				break
			}
		}
		if allDone {
			return nil
		}
		select {
		case <-m.depWaitChan():
		case <-time.After(2 * time.Second):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (m *BackgroundTaskManager) validateDependencies(taskID string, deps []string) error {
	if len(deps) == 0 {
		return nil
	}
	for _, dep := range deps {
		if utils.IsBlank(dep) {
			return fmt.Errorf("dependency task id must not be empty")
		}
		if dep == taskID {
			return fmt.Errorf("task %q cannot depend on itself", taskID)
		}
		if _, ok := m.tasks[dep]; !ok {
			return fmt.Errorf("dependency %q not found", dep)
		}
	}

	graph := make(map[string][]string, len(m.tasks)+1)
	for id, task := range m.tasks {
		graph[id] = append([]string(nil), task.dependsOn...)
	}
	graph[taskID] = append([]string(nil), deps...)

	const (
		unvisited = iota
		visiting
		done
	)
	state := make(map[string]int, len(graph))
	var visit func(string) error
	visit = func(node string) error {
		switch state[node] {
		case visiting:
			return fmt.Errorf("dependency cycle detected involving %q", node)
		case done:
			return nil
		}
		state[node] = visiting
		for _, next := range graph[node] {
			if err := visit(next); err != nil {
				return err
			}
		}
		state[node] = done
		return nil
	}
	return visit(taskID)
}
