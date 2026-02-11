package kernel

import (
	"context"
	"strings"

	kerneldomain "alex/internal/domain/kernel"
)

// Planner decides which agents to dispatch in a given cycle.
type Planner interface {
	Plan(ctx context.Context, stateContent string, recentByAgent map[string]kerneldomain.Dispatch) ([]kerneldomain.DispatchSpec, error)
}

// StaticPlanner generates dispatch specs from static agent configuration.
// It skips disabled agents and agents whose most recent dispatch is still running.
// The {STATE} placeholder in each agent's prompt is replaced with the current STATE.md content.
type StaticPlanner struct {
	kernelID string
	agents   []AgentConfig
}

// NewStaticPlanner creates a new StaticPlanner.
func NewStaticPlanner(kernelID string, agents []AgentConfig) *StaticPlanner {
	return &StaticPlanner{
		kernelID: kernelID,
		agents:   agents,
	}
}

// Plan produces dispatch specs for all eligible agents.
func (p *StaticPlanner) Plan(_ context.Context, stateContent string, recentByAgent map[string]kerneldomain.Dispatch) ([]kerneldomain.DispatchSpec, error) {
	var specs []kerneldomain.DispatchSpec
	for _, a := range p.agents {
		if !a.Enabled {
			continue
		}
		// Skip agents that are still running from a previous cycle.
		if d, ok := recentByAgent[a.AgentID]; ok && d.Status == kerneldomain.DispatchRunning {
			continue
		}
		prompt := strings.ReplaceAll(a.Prompt, "{STATE}", stateContent)
		specs = append(specs, kerneldomain.DispatchSpec{
			AgentID:  a.AgentID,
			Prompt:   prompt,
			Priority: a.Priority,
			Metadata: a.Metadata,
		})
	}
	return specs, nil
}
