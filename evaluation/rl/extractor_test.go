package rl

import (
	"testing"
	"time"

	"alex/evaluation/swe_bench"
)

func TestExtractor_Extract_Basic(t *testing.T) {
	e := NewExtractor()

	result := swe_bench.WorkerResult{
		TaskID:     "task-1",
		InstanceID: "inst-1",
		Status:     swe_bench.StatusCompleted,
		Duration:   30 * time.Second,
		TokensUsed: 1000,
		Cost:       0.05,
		Trace: []swe_bench.TraceStep{
			{Step: 0, Action: "search", Thought: "find file", Observation: "found", Timestamp: time.Now()},
			{Step: 1, Action: "edit", Thought: "fix bug", Observation: "done", ToolCall: &swe_bench.ToolCall{Name: "file_edit"}, Timestamp: time.Now()},
			{Step: 2, Action: "submit", Thought: "submit patch", Observation: "submitted", Timestamp: time.Now()},
		},
	}

	traj, err := e.Extract(result, "eval-job-1", 85.0, "A")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if traj.ID != "traj_eval-job-1_task-1" {
		t.Errorf("unexpected ID: %s", traj.ID)
	}
	if traj.EvalJobID != "eval-job-1" {
		t.Errorf("unexpected eval job ID: %s", traj.EvalJobID)
	}
	if traj.AutoScore != 85.0 {
		t.Errorf("unexpected auto score: %f", traj.AutoScore)
	}
	if traj.Grade != "A" {
		t.Errorf("unexpected grade: %s", traj.Grade)
	}
	if len(traj.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(traj.Steps))
	}
	if traj.Metadata.TotalSteps != 3 {
		t.Errorf("expected 3 total steps, got %d", traj.Metadata.TotalSteps)
	}
	if traj.Metadata.Outcome != "completed" {
		t.Errorf("expected completed outcome, got %s", traj.Metadata.Outcome)
	}
	if traj.Metadata.TokensUsed != 1000 {
		t.Errorf("expected 1000 tokens, got %d", traj.Metadata.TokensUsed)
	}
}

func TestExtractor_Extract_EmptyTrace(t *testing.T) {
	e := NewExtractor()
	result := swe_bench.WorkerResult{
		TaskID: "task-empty",
		Status: swe_bench.StatusCompleted,
		Trace:  nil,
	}

	_, err := e.Extract(result, "job-1", 50, "C")
	if err == nil {
		t.Fatal("expected error for empty trace")
	}
}

func TestExtractor_StepRewards(t *testing.T) {
	e := NewExtractor()

	result := swe_bench.WorkerResult{
		TaskID: "task-rewards",
		Status: swe_bench.StatusCompleted,
		Trace: []swe_bench.TraceStep{
			{Step: 0, Action: "think", Timestamp: time.Now()},
			{Step: 1, Action: "tool", ToolCall: &swe_bench.ToolCall{Name: "shell", Error: "exit 1"}, Timestamp: time.Now()},
			{Step: 2, Action: "submit", Timestamp: time.Now()},
		},
	}

	traj, err := e.Extract(result, "job-1", 70, "B")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// First step: positive base reward
	if traj.Steps[0].Reward != 0.1 {
		t.Errorf("step 0: expected 0.1, got %f", traj.Steps[0].Reward)
	}
	// Second step: tool error penalty
	if traj.Steps[1].Reward != -0.2 {
		t.Errorf("step 1: expected -0.2, got %f", traj.Steps[1].Reward)
	}
	// Last step: completed terminal reward
	if traj.Steps[2].Reward != 1.0 {
		t.Errorf("step 2: expected 1.0, got %f", traj.Steps[2].Reward)
	}
}

func TestExtractor_FailedOutcome(t *testing.T) {
	e := NewExtractor()

	result := swe_bench.WorkerResult{
		TaskID: "task-fail",
		Status: swe_bench.StatusFailed,
		Trace: []swe_bench.TraceStep{
			{Step: 0, Action: "attempt", Timestamp: time.Now()},
		},
	}

	traj, err := e.Extract(result, "job-1", 20, "F")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if traj.Metadata.Outcome != "failed" {
		t.Errorf("expected failed outcome, got %s", traj.Metadata.Outcome)
	}
	// Single step that's also last + failed
	if traj.Steps[0].Reward != -0.5 {
		t.Errorf("expected -0.5 reward for failed terminal, got %f", traj.Steps[0].Reward)
	}
}

func TestExtractor_ToolsCollected(t *testing.T) {
	e := NewExtractor()

	result := swe_bench.WorkerResult{
		TaskID: "task-tools",
		Status: swe_bench.StatusCompleted,
		Trace: []swe_bench.TraceStep{
			{Step: 0, Action: "a", ToolCall: &swe_bench.ToolCall{Name: "search"}, Timestamp: time.Now()},
			{Step: 1, Action: "b", ToolCall: &swe_bench.ToolCall{Name: "edit"}, Timestamp: time.Now()},
			{Step: 2, Action: "c", ToolCall: &swe_bench.ToolCall{Name: "search"}, Timestamp: time.Now()}, // duplicate
			{Step: 3, Action: "d", Timestamp: time.Now()}, // no tool call
		},
	}

	traj, err := e.Extract(result, "job-1", 90, "A+")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(traj.Metadata.ToolsUsed) != 2 {
		t.Errorf("expected 2 unique tools, got %d: %v", len(traj.Metadata.ToolsUsed), traj.Metadata.ToolsUsed)
	}
}
