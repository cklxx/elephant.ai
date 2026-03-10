package react

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	domain "alex/internal/domain/agent"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/shared/utils"
)

// InputRequests exposes external input requests when available.
func (m *BackgroundTaskManager) InputRequests() <-chan agent.InputRequest {
	return m.externalInputCh
}

// ReplyExternalInput forwards an external input response to the executor.
func (m *BackgroundTaskManager) ReplyExternalInput(ctx context.Context, resp agent.InputResponse) error {
	if m.inputExecutor == nil {
		return fmt.Errorf("external input responder not configured")
	}
	if err := m.inputExecutor.Reply(ctx, resp); err != nil {
		return err
	}
	if utils.HasContent(resp.TaskID) {
		m.mu.RLock()
		bt := m.tasks[resp.TaskID]
		m.mu.RUnlock()
		if bt != nil {
			bt.mu.Lock()
			if bt.pendingInput != nil && bt.pendingInput.RequestID == resp.RequestID {
				bt.pendingInput = nil
			}
			bt.mu.Unlock()
			if bt.emitEvent != nil && bt.baseEvent != nil {
				bt.emitEvent(domain.NewExternalInputResponseEvent(
					bt.baseEvent(ctx),
					resp.TaskID, resp.RequestID, resp.Approved, resp.OptionID, resp.Text,
				))
			}
		}
	}
	return nil
}

// InjectBackgroundInput forwards free-form input into a tmux-backed role pane.
func (m *BackgroundTaskManager) InjectBackgroundInput(ctx context.Context, taskID string, input string) error {
	id := strings.TrimSpace(taskID)
	if id == "" {
		return fmt.Errorf("task_id is required")
	}
	data := strings.TrimSpace(input)
	if data == "" {
		return fmt.Errorf("input is required")
	}

	m.mu.RLock()
	bt := m.tasks[id]
	m.mu.RUnlock()
	if bt == nil {
		return fmt.Errorf("%w: task %q", ErrBackgroundTaskNotFound, id)
	}

	bt.mu.Lock()
	pane := strings.TrimSpace(bt.config["tmux_pane"])
	bt.mu.Unlock()
	if pane == "" {
		err := fmt.Errorf("task %q is not bound to a tmux pane", id)
		recordTmuxInputInjectEvent(m.eventAppender, bt, "tmux_input_inject_failed", pane, data, err)
		return err
	}

	if err := m.tmuxSender.SendKeys(ctx, pane, data); err != nil {
		recordTmuxInputInjectEvent(m.eventAppender, bt, "tmux_input_inject_failed", pane, data, err)
		return err
	}
	recordTmuxInputInjectEvent(m.eventAppender, bt, "tmux_input_injected", pane, data, nil)
	return nil
}

func recordTmuxInputInjectEvent(appender agent.EventAppender, bt *backgroundTask, eventType string, pane string, input string, injectErr error) {
	if bt == nil {
		return
	}
	bt.mu.Lock()
	taskID := bt.id
	cfg := make(map[string]string, len(bt.config))
	for k, v := range bt.config {
		cfg[k] = v
	}
	bt.mu.Unlock()

	eventLogPath := strings.TrimSpace(cfg["team_event_log"])
	roleLogPath := strings.TrimSpace(cfg["role_log_path"])
	if eventLogPath == "" && roleLogPath == "" {
		return
	}

	payload := map[string]any{
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
		"type":      strings.TrimSpace(eventType),
		"task_id":   strings.TrimSpace(taskID),
		"input_len": len([]rune(strings.TrimSpace(input))),
	}
	if teamID := strings.TrimSpace(cfg["team_id"]); teamID != "" {
		payload["team_id"] = teamID
	}
	if roleID := strings.TrimSpace(cfg["role_id"]); roleID != "" {
		payload["role_id"] = roleID
	}
	if trimmedPane := strings.TrimSpace(pane); trimmedPane != "" {
		payload["pane"] = trimmedPane
	}
	if injectErr != nil {
		payload["error"] = injectErr.Error()
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	if appender == nil {
		return
	}
	line := string(data)
	appender.AppendLine(eventLogPath, line)
	appender.AppendLine(roleLogPath, line)
}

// MergeExternalWorkspace merges an external agent's workspace back into the base branch.
func (m *BackgroundTaskManager) MergeExternalWorkspace(ctx context.Context, taskID string, strategy agent.MergeStrategy) (*agent.MergeResult, error) {
	if m.workspaceMgr == nil {
		return nil, fmt.Errorf("workspace manager not available")
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, fmt.Errorf("task_id is required")
	}
	if strategy == "" {
		strategy = agent.MergeStrategyAuto
	}

	m.mu.RLock()
	bt := m.tasks[taskID]
	m.mu.RUnlock()
	if bt == nil {
		return nil, fmt.Errorf("background task %q not found", taskID)
	}
	bt.mu.Lock()
	alloc := bt.workspace
	bt.mu.Unlock()
	if alloc == nil {
		return nil, fmt.Errorf("task %q has no workspace to merge", taskID)
	}
	return m.workspaceMgr.Merge(ctx, alloc, strategy)
}

func (m *BackgroundTaskManager) forwardExternalInputRequests() {
	if m.inputExecutor == nil || m.externalInputCh == nil {
		return
	}
	inputs := m.inputExecutor.InputRequests()
	for {
		select {
		case <-m.taskCtx.Done():
			m.closeInputOnce.Do(func() { close(m.externalInputCh) })
			return
		case req, ok := <-inputs:
			if !ok {
				m.closeInputOnce.Do(func() { close(m.externalInputCh) })
				return
			}
			if utils.HasContent(req.TaskID) {
				m.mu.RLock()
				bt := m.tasks[req.TaskID]
				m.mu.RUnlock()
				if bt != nil {
					bt.mu.Lock()
					bt.pendingInput = &agent.InputRequestSummary{
						RequestID: req.RequestID,
						Type:      req.Type,
						Summary:   req.Summary,
						Since:     m.clock.Now(),
					}
					bt.mu.Unlock()
				}
			}
			select {
			case m.externalInputCh <- req:
			default:
				m.logger.Warn("external input channel full, dropping request %q", req.RequestID)
			}
		}
	}
}
