package domain

import "time"

// ExternalInputRequestEvent is emitted when an external agent requests input.
type ExternalInputRequestEvent struct {
	BaseEvent
	TaskID    string
	AgentType string
	RequestID string
	Type      string
	Summary   string
}

func (e *ExternalInputRequestEvent) EventType() string {
	return "external.input.requested"
}

// ExternalInputResponseEvent is emitted when the main agent responds to input.
type ExternalInputResponseEvent struct {
	BaseEvent
	TaskID    string
	RequestID string
	Approved  bool
	OptionID  string
	Message   string
}

func (e *ExternalInputResponseEvent) EventType() string {
	return "external.input.responded"
}

// ExternalAgentProgressEvent is emitted periodically for external agent progress.
type ExternalAgentProgressEvent struct {
	BaseEvent
	TaskID      string
	AgentType   string
	Iteration   int
	MaxIter     int
	TokensUsed  int
	CostUSD     float64
	CurrentTool string
	Elapsed     time.Duration
}

func (e *ExternalAgentProgressEvent) EventType() string {
	return "external.agent.progress"
}
