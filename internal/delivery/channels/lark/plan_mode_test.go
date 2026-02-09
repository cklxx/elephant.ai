package lark

import (
	"strings"
	"testing"
)

func TestIsPlanCommand(t *testing.T) {
	g := &Gateway{}
	tests := []struct {
		input string
		want  bool
	}{
		{"/plan on", true},
		{"/plan off", true},
		{"/plan auto", true},
		{"/plan status", true},
		{"/plan", true},
		{"/Plan On", true},
		{"/planning", false}, // exact match only
		{"/model", false},
		{"/tasks", false},
		{"plan on", false},
	}
	for _, tt := range tests {
		got := g.isPlanCommand(tt.input)
		if got != tt.want {
			t.Errorf("isPlanCommand(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestDefaultPlanMode(t *testing.T) {
	g := &Gateway{}
	if got := g.defaultPlanMode(); got != PlanModeAuto {
		t.Errorf("defaultPlanMode() = %q, want %q", got, PlanModeAuto)
	}

	g.cfg.DefaultPlanMode = PlanModeOn
	if got := g.defaultPlanMode(); got != PlanModeOn {
		t.Errorf("defaultPlanMode() = %q, want %q", got, PlanModeOn)
	}
}

func TestPlanModeUsage(t *testing.T) {
	usage := planModeUsage()
	if usage == "" {
		t.Error("planModeUsage should not be empty")
	}
	if !strings.Contains(usage, "/plan on") {
		t.Error("usage should mention /plan on")
	}
}

func TestPlanModeScopes(t *testing.T) {
	msg := &incomingMessage{chatID: "chat123"}
	scopes := planModeScopes(msg)
	if len(scopes) != 2 {
		t.Fatalf("expected 2 scopes, got %d", len(scopes))
	}
	if scopes[0].ChatID != "chat123" {
		t.Errorf("first scope should be chat-level, got %+v", scopes[0])
	}
	if scopes[1].ChatID != "" {
		t.Errorf("second scope should be channel-level, got %+v", scopes[1])
	}
}

