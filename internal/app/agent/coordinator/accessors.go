package coordinator

import agent "alex/internal/domain/agent/ports/agent"

// TeamRunRecorder returns the configured team run recorder for orchestration.
func (c *AgentCoordinator) TeamRunRecorder() agent.TeamRunRecorder {
	return c.teamRunRecorder
}
