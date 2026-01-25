package ports

import agent "alex/internal/agent/ports/agent"

// SSEBroadcaster manages client connections and broadcasts events
type SSEBroadcaster interface {
	// RegisterClient registers a new client for a session
	RegisterClient(sessionID string, ch chan agent.AgentEvent)

	// UnregisterClient removes a client from the session
	UnregisterClient(sessionID string, ch chan agent.AgentEvent)

	// GetClientCount returns the number of clients subscribed to a session
	GetClientCount(sessionID string) int
}
