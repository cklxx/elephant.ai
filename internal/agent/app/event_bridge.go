package app

import (
	"time"

	"alex/internal/agent/domain"
	tea "github.com/charmbracelet/bubbletea"
)

// EventBridge converts domain events to Bubble Tea messages
type EventBridge struct {
	program *tea.Program
}

// NewEventBridge creates a new event bridge
func NewEventBridge(program *tea.Program) *EventBridge {
	return &EventBridge{program: program}
}

// OnEvent implements domain.EventListener
func (b *EventBridge) OnEvent(event domain.AgentEvent) {
	// Convert to Bubble Tea message and send
	msg := b.convertToTUIMessage(event)
	if msg != nil {
		b.program.Send(msg)
	}
}

func (b *EventBridge) convertToTUIMessage(event domain.AgentEvent) tea.Msg {
	switch e := event.(type) {
	case *domain.IterationStartEvent:
		return IterationStartMsg{
			Timestamp:  e.Timestamp(),
			Iteration:  e.Iteration,
			TotalIters: e.TotalIters,
		}

	case *domain.ThinkingEvent:
		return ThinkingMsg{
			Timestamp: e.Timestamp(),
			Iteration: e.Iteration,
		}

	case *domain.ThinkCompleteEvent:
		return ThinkCompleteMsg{
			Timestamp:     e.Timestamp(),
			Iteration:     e.Iteration,
			Content:       e.Content,
			ToolCallCount: e.ToolCallCount,
		}

	case *domain.ToolCallStartEvent:
		return ToolCallStartMsg{
			Timestamp: e.Timestamp(),
			Iteration: e.Iteration,
			CallID:    e.CallID,
			ToolName:  e.ToolName,
			Arguments: e.Arguments,
		}

	case *domain.ToolCallStreamEvent:
		return ToolCallStreamMsg{
			Timestamp:  e.Timestamp(),
			CallID:     e.CallID,
			Chunk:      e.Chunk,
			IsComplete: e.IsComplete,
		}

	case *domain.ToolCallCompleteEvent:
		return ToolCallCompleteMsg{
			Timestamp: e.Timestamp(),
			CallID:    e.CallID,
			ToolName:  e.ToolName,
			Result:    e.Result,
			Error:     e.Error,
			Duration:  e.Duration,
		}

	case *domain.IterationCompleteEvent:
		return IterationCompleteMsg{
			Timestamp:  e.Timestamp(),
			Iteration:  e.Iteration,
			TokensUsed: e.TokensUsed,
			ToolsRun:   e.ToolsRun,
		}

	case *domain.TaskCompleteEvent:
		return TaskCompleteMsg{
			Timestamp:       e.Timestamp(),
			FinalAnswer:     e.FinalAnswer,
			TotalIterations: e.TotalIterations,
			TotalTokens:     e.TotalTokens,
			StopReason:      e.StopReason,
			Duration:        e.Duration,
		}

	case *domain.ErrorEvent:
		return ErrorMsg{
			Timestamp:   e.Timestamp(),
			Iteration:   e.Iteration,
			Phase:       e.Phase,
			Error:       e.Error,
			Recoverable: e.Recoverable,
		}

	default:
		return nil
	}
}

// TUI Messages (what Bubble Tea models receive)

type IterationStartMsg struct {
	Timestamp  time.Time
	Iteration  int
	TotalIters int
}

type ThinkingMsg struct {
	Timestamp time.Time
	Iteration int
}

type ThinkCompleteMsg struct {
	Timestamp     time.Time
	Iteration     int
	Content       string
	ToolCallCount int
}

type ToolCallStartMsg struct {
	Timestamp time.Time
	Iteration int
	CallID    string
	ToolName  string
	Arguments map[string]interface{}
}

type ToolCallStreamMsg struct {
	Timestamp  time.Time
	CallID     string
	Chunk      string
	IsComplete bool
}

type ToolCallCompleteMsg struct {
	Timestamp time.Time
	CallID    string
	ToolName  string
	Result    string
	Error     error
	Duration  time.Duration
}

type IterationCompleteMsg struct {
	Timestamp  time.Time
	Iteration  int
	TokensUsed int
	ToolsRun   int
}

type TaskCompleteMsg struct {
	Timestamp       time.Time
	FinalAnswer     string
	TotalIterations int
	TotalTokens     int
	StopReason      string
	Duration        time.Duration
}

type ErrorMsg struct {
	Timestamp   time.Time
	Iteration   int
	Phase       string
	Error       error
	Recoverable bool
}
