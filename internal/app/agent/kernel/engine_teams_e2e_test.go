package kernel

import (
	"context"
	"fmt"
	"strings"
	"testing"

	kerneldomain "alex/internal/domain/kernel"
	"alex/internal/shared/logging"
)

// TestTeamsKernel_TeamDispatchE2E verifies the full plan→dispatch→execute→feedback
// loop for a team dispatch, including structured role results in the cycle summary.
func TestTeamsKernel_TeamDispatchE2E(t *testing.T) {
	exec := &mockExecutor{
		summaries: []string{"team completed all roles"},
		teamRoles: []TeamRoleResult{
			{RoleID: "team-researcher", Status: "completed", Elapsed: "15s"},
			{RoleID: "team-writer", Status: "completed", Elapsed: "20s"},
			{RoleID: "team-reviewer", Status: "completed", Elapsed: "10s"},
		},
	}
	dir := t.TempDir()
	store := newMemStore()
	sf := NewStateFile(dir)
	planner := plannerFunc(func(_ context.Context, _ string, _ map[string]kerneldomain.Dispatch) ([]kerneldomain.DispatchSpec, error) {
		return []kerneldomain.DispatchSpec{
			{
				AgentID:  "team:research_team",
				Prompt:   "run research team",
				Priority: 9,
				Kind:     kerneldomain.DispatchKindTeam,
				Team: &kerneldomain.TeamDispatchSpec{
					Template: "research_team",
					Goal:     "analyze distributed caching",
					Wait:     true,
				},
			},
		}, nil
	})
	cfg := KernelConfig{
		KernelID:      "e2e-kernel",
		Schedule:      "*/10 * * * *",
		SeedState:     "# STATE\ne2e test\n",
		MaxConcurrent: 2,
	}
	engine := NewEngine(cfg, sf, store, planner, exec, logging.NewComponentLogger("e2e"))

	result, err := engine.RunCycle(context.Background())
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if result.Status != kerneldomain.CycleSuccess {
		t.Fatalf("expected success, got %s", result.Status)
	}
	if result.Dispatched != 1 || result.Succeeded != 1 {
		t.Fatalf("expected 1 dispatched + 1 succeeded, got %d/%d", result.Dispatched, result.Succeeded)
	}
	if len(result.AgentSummary) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(result.AgentSummary))
	}
	summary := result.AgentSummary[0]
	if summary.AgentID != "team:research_team" {
		t.Fatalf("expected agent_id 'team:research_team', got %q", summary.AgentID)
	}
	if len(summary.TeamRoles) != 3 {
		t.Fatalf("expected 3 team roles, got %d", len(summary.TeamRoles))
	}
	for _, role := range summary.TeamRoles {
		if role.Status != "completed" {
			t.Errorf("role %s: expected completed, got %q", role.RoleID, role.Status)
		}
	}
}

// TestTeamsKernel_PlannerFeedbackLoop verifies that team dispatch failures from cycle 1
// appear in the planner's input for cycle 2, enabling corrective action.
func TestTeamsKernel_PlannerFeedbackLoop(t *testing.T) {
	// Cycle 1: team dispatch partially fails.
	failExec := &mockExecutor{err: fmt.Errorf("partial team failure")}
	dir := t.TempDir()
	store := newMemStore()
	sf := NewStateFile(dir)

	cycle := 0
	planner := plannerFunc(func(_ context.Context, _ string, recentByAgent map[string]kerneldomain.Dispatch) ([]kerneldomain.DispatchSpec, error) {
		cycle++
		if cycle == 1 {
			return []kerneldomain.DispatchSpec{
				{
					AgentID:  "team:data_team",
					Prompt:   "run data pipeline",
					Priority: 9,
					Kind:     kerneldomain.DispatchKindTeam,
					Team:     &kerneldomain.TeamDispatchSpec{Template: "data_team", Goal: "ETL", Wait: true},
				},
			}, nil
		}
		// Cycle 2: verify planner sees failure from cycle 1.
		recent, ok := recentByAgent["team:data_team"]
		if !ok {
			t.Error("cycle 2: planner should see team:data_team in recentByAgent")
			return nil, nil
		}
		if recent.Status != kerneldomain.DispatchFailed {
			t.Errorf("cycle 2: expected failed status, got %s", recent.Status)
		}
		if !strings.Contains(recent.Error, "partial team failure") {
			t.Errorf("cycle 2: expected error info, got %q", recent.Error)
		}
		// Issue corrective dispatch.
		return []kerneldomain.DispatchSpec{
			{
				AgentID:  "fallback-agent",
				Prompt:   "manual fallback for ETL",
				Priority: 10,
				Kind:     kerneldomain.DispatchKindAgent,
			},
		}, nil
	})

	cfg := KernelConfig{
		KernelID:      "feedback-kernel",
		Schedule:      "*/10 * * * *",
		SeedState:     "# STATE\nfeedback test\n",
		MaxConcurrent: 1,
	}
	engine := NewEngine(cfg, sf, store, planner, failExec, logging.NewComponentLogger("e2e"))

	// Cycle 1: should fail.
	r1, err := engine.RunCycle(context.Background())
	if err != nil {
		t.Fatalf("cycle 1: %v", err)
	}
	if r1.Failed != 1 {
		t.Fatalf("cycle 1: expected 1 failed, got %d", r1.Failed)
	}

	// Cycle 2: planner sees failure, issues corrective dispatch.
	// Replace executor with one that succeeds.
	successExec := &mockExecutor{summaries: []string{"fallback done"}}
	engine.executor = successExec

	r2, err := engine.RunCycle(context.Background())
	if err != nil {
		t.Fatalf("cycle 2: %v", err)
	}
	if r2.Dispatched != 1 || r2.Succeeded != 1 {
		t.Fatalf("cycle 2: expected 1/1, got %d/%d", r2.Dispatched, r2.Succeeded)
	}
	if r2.AgentSummary[0].AgentID != "fallback-agent" {
		t.Fatalf("cycle 2: expected fallback-agent, got %q", r2.AgentSummary[0].AgentID)
	}
}

// TestTeamsKernel_FailureClassFeedback verifies that FailureClass=awaiting_input
// is set on dispatch failures and the [awaiting_input] prefix appears in the
// error message visible to subsequent cycles.
func TestTeamsKernel_FailureClassFeedback(t *testing.T) {
	awaitExec := &awaitingInputExecutor{}
	dir := t.TempDir()
	store := newMemStore()
	sf := NewStateFile(dir)

	cycle := 0
	planner := plannerFunc(func(_ context.Context, _ string, recentByAgent map[string]kerneldomain.Dispatch) ([]kerneldomain.DispatchSpec, error) {
		cycle++
		if cycle == 1 {
			return []kerneldomain.DispatchSpec{
				{AgentID: "lark-poster", Prompt: "post message", Priority: 8, Kind: kerneldomain.DispatchKindAgent},
			}, nil
		}
		// Cycle 2: verify [awaiting_input] prefix in error.
		recent, ok := recentByAgent["lark-poster"]
		if !ok {
			t.Error("cycle 2: missing lark-poster in history")
			return nil, nil
		}
		if !strings.Contains(recent.Error, "[awaiting_input]") {
			t.Errorf("cycle 2: expected [awaiting_input] prefix, got %q", recent.Error)
		}
		return nil, nil
	})

	cfg := KernelConfig{
		KernelID:      "fc-kernel",
		Schedule:      "*/10 * * * *",
		SeedState:     "# STATE\ntest\n",
		MaxConcurrent: 1,
	}
	engine := NewEngine(cfg, sf, store, planner, awaitExec, logging.NewComponentLogger("e2e"))

	// Cycle 1: awaiting_input failure.
	r1, err := engine.RunCycle(context.Background())
	if err != nil {
		t.Fatalf("cycle 1: %v", err)
	}
	if r1.Failed != 1 {
		t.Fatalf("expected 1 failed, got %d", r1.Failed)
	}
	if r1.AgentSummary[0].FailureClass != kernelAutonomyAwaiting {
		t.Fatalf("expected FailureClass=%q, got %q", kernelAutonomyAwaiting, r1.AgentSummary[0].FailureClass)
	}

	// Cycle 2: planner checks [awaiting_input] prefix.
	_, err = engine.RunCycle(context.Background())
	if err != nil {
		t.Fatalf("cycle 2: %v", err)
	}
	if cycle != 2 {
		t.Fatalf("expected 2 planner cycles, got %d", cycle)
	}
}

// TestTeamsKernel_MixedTeamAndAgentDispatch verifies that a single cycle can
// dispatch both a team and an agent, and both complete independently.
func TestTeamsKernel_MixedTeamAndAgentDispatch(t *testing.T) {
	exec := &mockExecutor{
		summaries: []string{"agent done", "team done"},
		teamRoles: []TeamRoleResult{
			{RoleID: "team-worker", Status: "completed"},
		},
	}
	dir := t.TempDir()
	store := newMemStore()
	sf := NewStateFile(dir)
	planner := plannerFunc(func(_ context.Context, _ string, _ map[string]kerneldomain.Dispatch) ([]kerneldomain.DispatchSpec, error) {
		return []kerneldomain.DispatchSpec{
			{
				AgentID:  "standalone-agent",
				Prompt:   "do standalone work",
				Priority: 8,
				Kind:     kerneldomain.DispatchKindAgent,
			},
			{
				AgentID:  "team:mixed_team",
				Prompt:   "run team template",
				Priority: 9,
				Kind:     kerneldomain.DispatchKindTeam,
				Team: &kerneldomain.TeamDispatchSpec{
					Template: "mixed_team",
					Goal:     "parallel test",
					Wait:     true,
				},
			},
		}, nil
	})
	cfg := KernelConfig{
		KernelID:      "mixed-kernel",
		Schedule:      "*/10 * * * *",
		SeedState:     "# STATE\nmixed test\n",
		MaxConcurrent: 2,
	}
	engine := NewEngine(cfg, sf, store, planner, exec, logging.NewComponentLogger("e2e"))

	result, err := engine.RunCycle(context.Background())
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if result.Status != kerneldomain.CycleSuccess {
		t.Fatalf("expected success, got %s", result.Status)
	}
	if result.Dispatched != 2 {
		t.Fatalf("expected 2 dispatched, got %d", result.Dispatched)
	}
	if result.Succeeded != 2 {
		t.Fatalf("expected 2 succeeded, got %d", result.Succeeded)
	}

	// Verify both summaries present.
	agents := map[string]bool{}
	for _, s := range result.AgentSummary {
		agents[s.AgentID] = true
	}
	if !agents["standalone-agent"] {
		t.Error("missing standalone-agent summary")
	}
	if !agents["team:mixed_team"] {
		t.Error("missing team:mixed_team summary")
	}

	// Verify call routing: agent via Execute, team via ExecuteTeam.
	if exec.callCount() != 1 {
		t.Fatalf("expected 1 regular Execute call, got %d", exec.callCount())
	}
	if exec.teamCallCount() != 1 {
		t.Fatalf("expected 1 ExecuteTeam call, got %d", exec.teamCallCount())
	}
}

// TestTeamsKernel_MultiTeamDispatch verifies that multiple team dispatches
// can execute concurrently in a single cycle when MaxTeamsPerCycle > 1.
func TestTeamsKernel_MultiTeamDispatch(t *testing.T) {
	exec := &mockExecutor{
		summaries: []string{"team a done", "team b done"},
	}
	dir := t.TempDir()
	store := newMemStore()
	sf := NewStateFile(dir)
	planner := plannerFunc(func(_ context.Context, _ string, _ map[string]kerneldomain.Dispatch) ([]kerneldomain.DispatchSpec, error) {
		return []kerneldomain.DispatchSpec{
			{
				AgentID:  "team:alpha",
				Prompt:   "run alpha",
				Priority: 9,
				Kind:     kerneldomain.DispatchKindTeam,
				Team:     &kerneldomain.TeamDispatchSpec{Template: "alpha", Goal: "goal a", Wait: true},
			},
			{
				AgentID:  "team:beta",
				Prompt:   "run beta",
				Priority: 8,
				Kind:     kerneldomain.DispatchKindTeam,
				Team:     &kerneldomain.TeamDispatchSpec{Template: "beta", Goal: "goal b", Wait: true},
			},
		}, nil
	})
	cfg := KernelConfig{
		KernelID:      "multi-team-kernel",
		Schedule:      "*/10 * * * *",
		SeedState:     "# STATE\nmulti team\n",
		MaxConcurrent: 2,
	}
	engine := NewEngine(cfg, sf, store, planner, exec, logging.NewComponentLogger("e2e"))

	result, err := engine.RunCycle(context.Background())
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if result.Status != kerneldomain.CycleSuccess {
		t.Fatalf("expected success, got %s", result.Status)
	}
	if result.Dispatched != 2 || result.Succeeded != 2 {
		t.Fatalf("expected 2/2, got %d/%d", result.Dispatched, result.Succeeded)
	}
	if exec.teamCallCount() != 2 {
		t.Fatalf("expected 2 team calls, got %d", exec.teamCallCount())
	}
}
