package coordinator

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	appconfig "alex/internal/app/agent/config"
	coretape "alex/internal/core/tape"
	domain "alex/internal/domain/agent"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/types"
	"alex/internal/domain/workflow"
	"alex/internal/infra/llm"
	infratape "alex/internal/infra/tape"
)

func newIntegrationCoordinator(opts ...CoordinatorOption) *AgentCoordinator {
	return NewAgentCoordinator(
		llm.NewFactory(),
		stubToolRegistry{},
		&stubSessionStore{},
		stubContextManager{},
		nil,
		stubParser{},
		nil,
		appconfig.Config{LLMProvider: "mock", LLMModel: "integration", MaxIterations: 2},
		opts...,
	)
}

// TestIntegration_HappyPath verifies tape, workflow, and SSE events in one execution.
func TestIntegration_HappyPath(t *testing.T) {
	store := infratape.NewMemoryStore()
	tapeMgr := coretape.NewTapeManager(store, coretape.TapeContext{TapeName: "e2e"})
	listener := &eventCollector{}

	coordinator := newIntegrationCoordinator(WithTapeManager(tapeMgr))

	ctx := agent.WithOutputContext(context.Background(), &agent.OutputContext{Level: agent.LevelCore})
	result, err := coordinator.ExecuteTask(ctx, "integration test", "", listener)
	if err != nil {
		t.Fatalf("ExecuteTask: %v", err)
	}

	// --- Tape ---
	entries, err := tapeMgr.Query(ctx, coretape.Query())
	if err != nil {
		t.Fatalf("tape query: %v", err)
	}
	labels := anchorLabels(filterKind(entries, coretape.KindAnchor))
	if !sliceContains(labels, "turn_start") || !sliceContains(labels, "turn_end") {
		t.Errorf("tape: want turn_start+turn_end; got %v", labels)
	}
	startIdx, endIdx := labelIndex(entries, "turn_start"), labelIndex(entries, "turn_end")
	if startIdx < 0 || endIdx < 0 || startIdx >= endIdx {
		t.Errorf("turn_start (idx=%d) must precede turn_end (idx=%d)", startIdx, endIdx)
	}
	if errs := filterKind(entries, coretape.KindError); len(errs) != 0 {
		t.Errorf("expected 0 tape errors on success, got %d", len(errs))
	}

	// --- Workflow ---
	if result.Workflow == nil {
		t.Fatal("nil workflow snapshot")
	}
	if result.Workflow.Phase != workflow.PhaseSucceeded {
		t.Fatalf("workflow phase=%s, want succeeded", result.Workflow.Phase)
	}
	nodes := make(map[string]workflow.NodeStatus, len(result.Workflow.Nodes))
	for _, n := range result.Workflow.Nodes {
		nodes[n.ID] = n.Status
	}
	for _, forbidden := range []string{"prepare", "execute", "summarize", "persist"} {
		if _, found := nodes[forbidden]; found {
			t.Errorf("old stage node %q must not exist", forbidden)
		}
	}
	for _, want := range []string{"react:context", "react:finalize"} {
		if s, ok := nodes[want]; !ok || s != workflow.NodeStatusSucceeded {
			t.Errorf("node %q: found=%v status=%s", want, ok, s)
		}
	}
	for id := range nodes {
		if !strings.HasPrefix(id, "react:") {
			t.Errorf("non-react node: %s", id)
		}
	}

	// --- SSE Events ---
	hasLifecycle, hasNode, hasResult := false, false, false
	for _, evt := range listener.all() {
		switch e := evt.(type) {
		case *domain.Event:
			switch e.Kind {
			case types.EventLifecycleUpdated:
				hasLifecycle = true
			case types.EventNodeStarted, types.EventNodeCompleted:
				hasNode = true
			case types.EventResultFinal:
				hasResult = true
			}
		case *domain.WorkflowEventEnvelope:
			switch e.Event {
			case types.EventLifecycleUpdated:
				hasLifecycle = true
			case types.EventNodeStarted, types.EventNodeCompleted:
				hasNode = true
			case types.EventResultFinal:
				hasResult = true
			}
		}
	}
	if !hasLifecycle {
		t.Error("no lifecycle events")
	}
	if !hasNode {
		t.Error("no node events")
	}
	if !hasResult {
		t.Error("no result final event")
	}
}

// TestIntegration_ErrorPath verifies tape records errors when preparation fails.
func TestIntegration_ErrorPath(t *testing.T) {
	store := infratape.NewMemoryStore()
	tapeMgr := coretape.NewTapeManager(store, coretape.TapeContext{TapeName: "error"})

	coordinator := newIntegrationCoordinator(WithTapeManager(tapeMgr))
	coordinator.prepService = &stubPreparationService{err: fmt.Errorf("injected failure")}

	ctx := agent.WithOutputContext(context.Background(), &agent.OutputContext{Level: agent.LevelCore})
	_, err := coordinator.ExecuteTask(ctx, "fail", "", nil)
	if err == nil {
		t.Fatal("expected error")
	}

	entries, _ := tapeMgr.Query(ctx, coretape.Query())
	errs := filterKind(entries, coretape.KindError)
	if len(errs) == 0 {
		t.Fatalf("expected error entries; total=%d", len(entries))
	}
	hasLoadState := false
	for _, e := range errs {
		if code, _ := e.Payload["code"].(string); code == "load_state" {
			hasLoadState = true
		}
	}
	if !hasLoadState {
		t.Error("expected error with code=load_state")
	}
}

// --- helpers ---

type eventCollector struct {
	mu     sync.Mutex
	events []agent.AgentEvent
}

func (c *eventCollector) OnEvent(evt agent.AgentEvent) {
	c.mu.Lock()
	c.events = append(c.events, evt)
	c.mu.Unlock()
}

func (c *eventCollector) all() []agent.AgentEvent {
	c.mu.Lock()
	defer c.mu.Unlock()
	cp := make([]agent.AgentEvent, len(c.events))
	copy(cp, c.events)
	return cp
}

func filterKind(entries []coretape.TapeEntry, kind coretape.EntryKind) []coretape.TapeEntry {
	var out []coretape.TapeEntry
	for _, e := range entries {
		if e.Kind == kind {
			out = append(out, e)
		}
	}
	return out
}

func anchorLabels(anchors []coretape.TapeEntry) []string {
	var out []string
	for _, a := range anchors {
		if label, ok := a.Payload["label"].(string); ok {
			out = append(out, label)
		}
	}
	return out
}

func labelIndex(entries []coretape.TapeEntry, label string) int {
	for i, e := range entries {
		if e.Kind == coretape.KindAnchor {
			if l, ok := e.Payload["label"].(string); ok && l == label {
				return i
			}
		}
	}
	return -1
}

func sliceContains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
