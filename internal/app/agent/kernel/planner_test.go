package kernel

import (
	"context"
	"strings"
	"testing"

	kerneldomain "alex/internal/domain/kernel"
)

func TestStaticPlanner_PlaceholderReplacement(t *testing.T) {
	p := NewStaticPlanner("k1", []AgentConfig{
		{AgentID: "a1", Prompt: "STATE={STATE}\nDo work.", Priority: 5, Enabled: true},
	})
	specs, err := p.Plan(context.Background(), "# my state", nil)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("expected 1 spec, got %d", len(specs))
	}
	if !strings.Contains(specs[0].Prompt, "STATE=# my state") {
		t.Errorf("placeholder not replaced: %q", specs[0].Prompt)
	}
}

func TestStaticPlanner_DisabledAgentSkipped(t *testing.T) {
	p := NewStaticPlanner("k1", []AgentConfig{
		{AgentID: "enabled", Prompt: "go", Priority: 5, Enabled: true},
		{AgentID: "disabled", Prompt: "go", Priority: 5, Enabled: false},
	})
	specs, err := p.Plan(context.Background(), "", nil)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("expected 1 spec, got %d", len(specs))
	}
	if specs[0].AgentID != "enabled" {
		t.Errorf("expected enabled, got %s", specs[0].AgentID)
	}
}

func TestStaticPlanner_RunningAgentSkipped(t *testing.T) {
	p := NewStaticPlanner("k1", []AgentConfig{
		{AgentID: "running", Prompt: "go", Priority: 5, Enabled: true},
		{AgentID: "idle", Prompt: "go", Priority: 5, Enabled: true},
	})
	recent := map[string]kerneldomain.Dispatch{
		"running": {AgentID: "running", Status: kerneldomain.DispatchRunning},
		"idle":    {AgentID: "idle", Status: kerneldomain.DispatchDone},
	}
	specs, err := p.Plan(context.Background(), "", recent)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("expected 1 spec, got %d", len(specs))
	}
	if specs[0].AgentID != "idle" {
		t.Errorf("expected idle, got %s", specs[0].AgentID)
	}
}

func TestStaticPlanner_EmptyAgents(t *testing.T) {
	p := NewStaticPlanner("k1", nil)
	specs, err := p.Plan(context.Background(), "state", nil)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(specs) != 0 {
		t.Errorf("expected 0 specs, got %d", len(specs))
	}
}
