package eventhub

import (
	"fmt"
	"strings"
	"sync"

	"alex/cmd/alex/ui/state"
	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	"alex/internal/tools/builtin"
)

// Hub converts domain events into UI state updates and broadcasts them to subscribers.
type Hub struct {
	mu           sync.RWMutex
	subscribers  map[chan state.Update]struct{}
	closed       bool
	defaultQueue int
}

// NewHub creates a new Hub instance.
func NewHub() *Hub {
	return &Hub{
		subscribers:  make(map[chan state.Update]struct{}),
		defaultQueue: 32,
	}
}

// Subscribe registers a listener channel. The returned channel should be unsubscribed when no longer needed.
func (h *Hub) Subscribe(buffer int) chan state.Update {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		ch := make(chan state.Update)
		close(ch)
		return ch
	}

	if buffer <= 0 {
		buffer = h.defaultQueue
	}
	ch := make(chan state.Update, buffer)
	h.subscribers[ch] = struct{}{}
	return ch
}

// Unsubscribe removes a previously registered channel.
func (h *Hub) Unsubscribe(ch chan state.Update) {
	if ch == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, exists := h.subscribers[ch]; exists {
		delete(h.subscribers, ch)
		close(ch)
	}
}

// Close closes the hub and all subscriber channels.
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return
	}
	h.closed = true
	for ch := range h.subscribers {
		close(ch)
	}
	h.subscribers = nil
}

// PublishAgentEvent ingests coordinator domain events and emits UI updates.
func (h *Hub) PublishAgentEvent(event ports.AgentEvent) {
	if event == nil {
		return
	}

	var updates []state.Update

	if subtask, ok := event.(*builtin.SubtaskEvent); ok {
		updates = append(updates, translateSubtaskEvent(subtask)...)
		event = subtask.OriginalEvent
	}

	updates = append(updates, translateAgentEvent(event)...)
	h.broadcast(updates)
}

// PublishMCPDelta emits MCP status changes directly.
func (h *Hub) PublishMCPDelta(delta state.MCPServerDelta) {
	if delta.Name == "" {
		return
	}
	h.broadcast([]state.Update{delta})
}

func (h *Hub) broadcast(updates []state.Update) {
	if len(updates) == 0 {
		return
	}

	subscribers := h.snapshotSubscribers()
	if len(subscribers) == 0 {
		return
	}

	closed := make(map[chan state.Update]struct{})
	for _, update := range updates {
		for _, ch := range subscribers {
			if _, skip := closed[ch]; skip {
				continue
			}
			if !trySendUpdate(ch, update) {
				closed[ch] = struct{}{}
			}
		}
	}
}

func (h *Hub) snapshotSubscribers() []chan state.Update {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.closed || len(h.subscribers) == 0 {
		return nil
	}

	subs := make([]chan state.Update, 0, len(h.subscribers))
	for ch := range h.subscribers {
		subs = append(subs, ch)
	}
	return subs
}

func trySendUpdate(ch chan state.Update, update state.Update) (ok bool) {
	ok = true
	defer func() {
		if r := recover(); r != nil {
			if !isSendOnClosedChannelPanic(r) {
				panic(r)
			}
			ok = false
		}
	}()

	select {
	case ch <- update:
	default:
		// Drop when subscriber backlog is full to avoid blocking event loop.
	}
	return
}

func isSendOnClosedChannelPanic(p interface{}) bool {
	msg, ok := p.(string)
	return ok && msg == "send on closed channel"
}

func translateAgentEvent(event ports.AgentEvent) []state.Update {
	switch e := event.(type) {
	case *domain.TaskAnalysisEvent:
		content := strings.TrimSpace(fmt.Sprintf("%s\n%s", e.ActionName, e.Goal))
		if content == "" {
			return nil
		}
		return []state.Update{state.MessageAppend{Message: state.ChatMessage{
			Role:      state.RoleSystem,
			AgentID:   string(e.GetAgentLevel()),
			Content:   content,
			CreatedAt: e.Timestamp(),
		}}}
	case *domain.ThinkCompleteEvent:
		if strings.TrimSpace(e.Content) == "" {
			return nil
		}
		return []state.Update{state.MessageAppend{Message: state.ChatMessage{
			Role:      state.RoleAssistant,
			AgentID:   string(e.GetAgentLevel()),
			Content:   e.Content,
			CreatedAt: e.Timestamp(),
		}}}
	case *domain.ToolCallStartEvent:
		status := state.ToolStatusRunning
		startedAt := e.Timestamp()
		return []state.Update{state.ToolRunDelta{
			CallID:     e.CallID,
			ToolName:   e.ToolName,
			AgentID:    string(e.GetAgentLevel()),
			AgentLevel: string(e.GetAgentLevel()),
			Arguments:  e.Arguments,
			StartedAt:  &startedAt,
			Status:     &status,
			Timestamp:  e.Timestamp(),
		}}
	case *domain.ToolCallStreamEvent:
		chunk := e.Chunk
		if chunk == "" {
			return nil
		}
		ts := e.Timestamp()
		return []state.Update{state.ToolRunDelta{
			CallID:       e.CallID,
			AppendStream: chunk,
			Timestamp:    ts,
		}}
	case *domain.ToolCallCompleteEvent:
		completedAt := e.Timestamp()
		duration := e.Duration
		status := state.ToolStatusCompleted
		var errorText *string
		if e.Error != nil {
			status = state.ToolStatusFailed
			msg := e.Error.Error()
			errorText = &msg
		}
		result := e.Result
		var resultPtr *string
		if strings.TrimSpace(result) != "" {
			resultPtr = &result
		}
		updates := []state.Update{state.ToolRunDelta{
			CallID:      e.CallID,
			ToolName:    e.ToolName,
			AgentID:     string(e.GetAgentLevel()),
			AgentLevel:  string(e.GetAgentLevel()),
			CompletedAt: &completedAt,
			Duration:    &duration,
			Status:      &status,
			Result:      resultPtr,
			Error:       errorText,
			Timestamp:   e.Timestamp(),
		}}
		if errorText != nil {
			updates = append(updates, state.MessageAppend{Message: state.ChatMessage{
				Role:      state.RoleSystem,
				AgentID:   string(e.GetAgentLevel()),
				Content:   fmt.Sprintf("Tool %s failed: %s", e.ToolName, *errorText),
				CreatedAt: e.Timestamp(),
			}})
		} else if resultPtr != nil {
			updates = append(updates, state.MessageAppend{Message: state.ChatMessage{
				Role:      state.RoleTool,
				AgentID:   string(e.GetAgentLevel()),
				Content:   *resultPtr,
				CreatedAt: e.Timestamp(),
			}})
		}
		return updates
	case *domain.ErrorEvent:
		message := fmt.Sprintf("Error during %s: %v", e.Phase, e.Error)
		return []state.Update{state.MessageAppend{Message: state.ChatMessage{
			Role:      state.RoleSystem,
			AgentID:   string(e.GetAgentLevel()),
			Content:   message,
			CreatedAt: e.Timestamp(),
		}}}
	case *domain.TaskCompleteEvent:
		if strings.TrimSpace(e.FinalAnswer) == "" {
			var updates []state.Update
			if e.TotalTokens > 0 {
				updates = append(updates, state.MetricsDelta{
					AgentLevel: string(e.GetAgentLevel()),
					Tokens:     e.TotalTokens,
					Timestamp:  e.Timestamp(),
				})
			}
			if len(updates) == 0 {
				return nil
			}
			return updates
		}
		updates := []state.Update{state.MessageAppend{Message: state.ChatMessage{
			Role:      state.RoleAssistant,
			AgentID:   string(e.GetAgentLevel()),
			Content:   e.FinalAnswer,
			CreatedAt: e.Timestamp(),
		}}}
		if e.TotalTokens > 0 {
			updates = append(updates, state.MetricsDelta{
				AgentLevel: string(e.GetAgentLevel()),
				Tokens:     e.TotalTokens,
				Timestamp:  e.Timestamp(),
			})
		}
		return updates
	default:
		return nil
	}
}

func translateSubtaskEvent(event *builtin.SubtaskEvent) []state.Update {
	if event == nil || event.OriginalEvent == nil {
		return nil
	}
	base := state.SubtaskDelta{
		Index:      event.SubtaskIndex,
		Total:      event.TotalSubtasks,
		Preview:    event.SubtaskPreview,
		AgentLevel: string(event.OriginalEvent.GetAgentLevel()),
		Timestamp:  event.Timestamp(),
	}
	switch e := event.OriginalEvent.(type) {
	case *domain.ToolCallStartEvent:
		status := state.SubtaskStatusRunning
		currentTool := e.ToolName
		base.Status = &status
		base.CurrentTool = &currentTool
		startedAt := e.Timestamp()
		base.StartedAt = &startedAt
		return []state.Update{base}
	case *domain.ToolCallCompleteEvent:
		currentTool := ""
		base.CurrentTool = &currentTool
		base.ToolsCompletedDelta = 1
		return []state.Update{base}
	case *domain.TaskCompleteEvent:
		status := state.SubtaskStatusCompleted
		base.Status = &status
		tokens := e.TotalTokens
		base.TokensUsed = &tokens
		currentTool := ""
		base.CurrentTool = &currentTool
		completedAt := e.Timestamp()
		base.CompletedAt = &completedAt
		return []state.Update{base}
	case *domain.ErrorEvent:
		status := state.SubtaskStatusFailed
		base.Status = &status
		errText := ""
		if e.Error != nil {
			errText = e.Error.Error()
		}
		base.Error = &errText
		completedAt := e.Timestamp()
		base.CompletedAt = &completedAt
		return []state.Update{base}
	default:
		return nil
	}
}
