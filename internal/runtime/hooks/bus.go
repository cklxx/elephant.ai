// Package hooks provides the in-process event bus used by the Kaku runtime
// to route lifecycle events (heartbeat, completed, stalled, …) between the
// session adapters, stall detector, scheduler, and leader agent.
package hooks

import "time"

// EventType identifies what happened in a runtime session.
type EventType string

const (
	// EventHeartbeat is published when an adapter receives a sign-of-life
	// signal from the running member CLI (e.g. CC PostToolUse hook).
	EventHeartbeat EventType = "heartbeat"

	// EventStarted is published when a session successfully enters the running state.
	EventStarted EventType = "started"

	// EventCompleted is published when the member CLI finishes successfully.
	EventCompleted EventType = "completed"

	// EventFailed is published when the member CLI exits with an error.
	EventFailed EventType = "failed"

	// EventStalled is published by the StallDetector when a session has had
	// no heartbeat for longer than the configured threshold.
	EventStalled EventType = "stalled"

	// EventNeedsInput is published when the member CLI is waiting for human input.
	EventNeedsInput EventType = "needs_input"

	// EventHandoffRequired is published when the leader agent decides to escalate
	// a stalled session to a human operator.
	EventHandoffRequired EventType = "handoff_required"
)

// Event carries data about a single lifecycle occurrence in a runtime session.
type Event struct {
	Type      EventType
	SessionID string
	At        time.Time
	Payload   map[string]any
}

// Bus is the in-process pub/sub backbone for runtime events.
// Every session has its own channel; SubscribeAll returns events across all sessions.
type Bus interface {
	// Publish dispatches ev to all subscribers of sessionID and to all-session subscribers.
	Publish(sessionID string, ev Event)

	// Subscribe returns a channel that receives events for sessionID.
	// The returned cancel function must be called when the subscriber is done.
	Subscribe(sessionID string) (<-chan Event, func())

	// SubscribeAll returns a channel that receives events for every session.
	// The returned cancel function must be called when the subscriber is done.
	SubscribeAll() (<-chan Event, func())
}
