package kernel

import "testing"

func TestDefaultRuntimeSettings_OutreachExecutorDisabledByDefault(t *testing.T) {
	runtimeSettings := DefaultRuntimeSettings()

	var outreach *AgentConfig
	for i := range runtimeSettings.Agents {
		agent := &runtimeSettings.Agents[i]
		if agent.AgentID == "outreach-executor" {
			outreach = agent
			break
		}
	}

	if outreach == nil {
		t.Fatal("expected outreach-executor in default runtime agents")
	}
	if outreach.Enabled {
		t.Fatal("expected outreach-executor to be disabled by default")
	}
}
