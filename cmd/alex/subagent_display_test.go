package main

import (
	"strings"
	"testing"

	"alex/internal/agent/domain"
	"alex/internal/tools/builtin"
)

func TestSubagentDisplaySuccess(t *testing.T) {
	display := NewSubagentDisplay()

	initialLines := display.Handle(&builtin.SubtaskEvent{
		OriginalEvent:  &domain.WorkflowToolStartedEvent{},
		SubtaskIndex:   0,
		TotalSubtasks:  2,
		SubtaskPreview: "Investigate login bug",
		MaxParallel:    2,
	})
	if len(initialLines) != 2 || !strings.Contains(initialLines[0], "Subagent: Running 2 tasks (max 2 parallel)") {
		t.Fatalf("expected header announcing total tasks with parallel info, got %v", initialLines)
	}
	if !strings.Contains(initialLines[1], "→ Task 1 – Investigate login bug") {
		t.Fatalf("expected start line for first subtask, got %v", initialLines)
	}

	noOutput := display.Handle(&builtin.SubtaskEvent{
		OriginalEvent: &domain.WorkflowToolCompletedEvent{},
		SubtaskIndex:  0,
	})
	if len(noOutput) != 0 {
		t.Fatalf("expected no output on tool completion, got %v", noOutput)
	}

	lines := display.Handle(&builtin.SubtaskEvent{
		OriginalEvent: &domain.WorkflowResultFinalEvent{TotalTokens: 120},
		SubtaskIndex:  0,
	})
	if len(lines) != 1 {
		t.Fatalf("expected a single completion line, got %v", lines)
	}
	if !strings.Contains(lines[0], "✓ [1/2] Task 1") {
		t.Fatalf("expected progress counter in completion line, got %q", lines[0])
	}
	if !strings.Contains(lines[0], "120 token") {
		t.Fatalf("expected token count in completion line, got %q", lines[0])
	}
	if !strings.Contains(lines[0], "Investigate login bug") {
		t.Fatalf("expected preview in completion line, got %q", lines[0])
	}

	secondStart := display.Handle(&builtin.SubtaskEvent{
		OriginalEvent:  &domain.WorkflowToolStartedEvent{},
		SubtaskIndex:   1,
		TotalSubtasks:  2,
		SubtaskPreview: "Prepare release plan",
	})
	if len(secondStart) != 1 || !strings.Contains(secondStart[0], "→ Task 2 – Prepare release plan") {
		t.Fatalf("expected start line for second task, got %v", secondStart)
	}

	display.Handle(&builtin.SubtaskEvent{
		OriginalEvent: &domain.WorkflowToolCompletedEvent{},
		SubtaskIndex:  1,
	})

	completion := display.Handle(&builtin.SubtaskEvent{
		OriginalEvent: &domain.WorkflowResultFinalEvent{TotalTokens: 80},
		SubtaskIndex:  1,
	})

	if len(completion) != 2 {
		t.Fatalf("expected completion and summary lines for second subtask, got %v", completion)
	}
	if !strings.Contains(completion[0], "✓ [2/2] Task 2") {
		t.Fatalf("expected concluded counter for second task, got %q", completion[0])
	}
	if !strings.Contains(completion[0], "Prepare release plan") {
		t.Fatalf("expected preview for second task, got %q", completion[0])
	}
	if !strings.Contains(completion[1], "All 2 tasks completed successfully") {
		t.Fatalf("expected success summary after all subtasks complete, got %q", completion[1])
	}
	if !strings.Contains(completion[1], "200 tokens") {
		t.Fatalf("expected success summary to include total tokens, got %q", completion[1])
	}
	if !strings.Contains(completion[1], "2 tool calls") {
		t.Fatalf("expected success summary to include total tool calls, got %q", completion[1])
	}
}

func TestSubagentDisplayFailure(t *testing.T) {
	display := NewSubagentDisplay()

	firstLines := display.Handle(&builtin.SubtaskEvent{
		OriginalEvent:  &domain.WorkflowToolStartedEvent{},
		SubtaskIndex:   0,
		TotalSubtasks:  1,
		SubtaskPreview: "Draft release notes",
		MaxParallel:    0,
	})
	if len(firstLines) != 2 {
		t.Fatalf("expected header and start line, got %v", firstLines)
	}
	if strings.Contains(firstLines[0], "max 0 parallel") {
		t.Fatalf("expected zero max_parallel to fall back to serial phrasing, got %q", firstLines[0])
	}
	if !strings.Contains(firstLines[1], "→ Task 1 – Draft release notes") {
		t.Fatalf("expected start line for first task, got %v", firstLines)
	}

	failure := display.Handle(&builtin.SubtaskEvent{
		OriginalEvent: &domain.WorkflowNodeFailedEvent{Error: assertError("timeout exceeded")},
		SubtaskIndex:  0,
	})
	if len(failure) != 2 {
		t.Fatalf("expected failure and summary lines, got %v", failure)
	}
	if !strings.Contains(failure[0], "✗ [1/1] Task 1") {
		t.Fatalf("expected concluded counter in failure line, got %q", failure[0])
	}
	if !strings.Contains(failure[0], "timeout exceeded") {
		t.Fatalf("expected error text in failure line, got %q", failure[0])
	}
	if !strings.Contains(failure[1], "0 of 1 task completed, 1 failure") {
		t.Fatalf("expected failure summary after error, got %q", failure[1])
	}
	if !strings.Contains(failure[1], "0 tokens") {
		t.Fatalf("expected failure summary to include token totals, got %q", failure[1])
	}
	if !strings.Contains(failure[1], "0 tool calls") {
		t.Fatalf("expected failure summary to include tool call totals, got %q", failure[1])
	}
}

func TestSubagentDisplayReprintsSummaryForAdditionalSubtasks(t *testing.T) {
	display := NewSubagentDisplay()

	header := display.Handle(&builtin.SubtaskEvent{
		OriginalEvent: &domain.WorkflowToolStartedEvent{},
		SubtaskIndex:  0,
		TotalSubtasks: 1,
	})
	if len(header) != 2 {
		t.Fatalf("expected header and start line for first subtask, got %v", header)
	}

	firstCompletion := display.Handle(&builtin.SubtaskEvent{
		OriginalEvent: &domain.WorkflowResultFinalEvent{TotalTokens: 10},
		SubtaskIndex:  0,
	})
	if len(firstCompletion) != 2 {
		t.Fatalf("expected completion and summary lines for first subtask, got %v", firstCompletion)
	}
	if !strings.Contains(firstCompletion[1], "All 1 task completed successfully") {
		t.Fatalf("expected singular success summary, got %q", firstCompletion[1])
	}

	// Introduce another subtask with an updated total count.
	secondStart := display.Handle(&builtin.SubtaskEvent{
		OriginalEvent: &domain.WorkflowToolStartedEvent{},
		SubtaskIndex:  1,
		TotalSubtasks: 2,
	})
	if len(secondStart) != 2 {
		t.Fatalf("expected updated header and start line when new subtask begins, got %v", secondStart)
	}
	if !strings.Contains(secondStart[0], "↻ Subagent: Running 2 tasks") {
		t.Fatalf("expected refreshed header reflecting updated total, got %v", secondStart[0])
	}
	if !strings.Contains(secondStart[1], "→ Task 2") {
		t.Fatalf("expected start line when new subtask begins, got %v", secondStart)
	}

	secondCompletion := display.Handle(&builtin.SubtaskEvent{
		OriginalEvent: &domain.WorkflowResultFinalEvent{TotalTokens: 20},
		SubtaskIndex:  1,
	})
	if len(secondCompletion) != 2 {
		t.Fatalf("expected completion and refreshed summary for second subtask, got %v", secondCompletion)
	}
	if !strings.Contains(secondCompletion[1], "All 2 tasks completed successfully") {
		t.Fatalf("expected summary to reflect new total count, got %q", secondCompletion[1])
	}
	if !strings.Contains(secondCompletion[1], "30 tokens") {
		t.Fatalf("expected aggregated token totals in refreshed summary, got %q", secondCompletion[1])
	}
}

func TestSubagentDisplayUpdatesHeaderWhenMaxParallelIncreases(t *testing.T) {
	display := NewSubagentDisplay()

	initial := display.Handle(&builtin.SubtaskEvent{
		OriginalEvent: &domain.WorkflowToolStartedEvent{},
		SubtaskIndex:  0,
		TotalSubtasks: 2,
		MaxParallel:   1,
	})
	if len(initial) != 2 || !strings.Contains(initial[0], "Subagent: Running 2 tasks") {
		t.Fatalf("expected initial header for two tasks, got %v", initial)
	}
	if strings.Contains(initial[0], "max 1 parallel") {
		t.Fatalf("expected serial phrasing when max parallel is one, got %q", initial[0])
	}

	update := display.Handle(&builtin.SubtaskEvent{
		OriginalEvent: &domain.WorkflowToolStartedEvent{},
		SubtaskIndex:  1,
		TotalSubtasks: 2,
		MaxParallel:   3,
	})
	if len(update) != 2 {
		t.Fatalf("expected header refresh and start line after max parallel increases, got %v", update)
	}
	if !strings.Contains(update[0], "↻ Subagent: Running 2 tasks (max 3 parallel)") {
		t.Fatalf("expected header to include updated parallel count, got %q", update[0])
	}
	if !strings.Contains(update[1], "→ Task 2") {
		t.Fatalf("expected start line for second task, got %v", update)
	}
}

type assertError string

func (e assertError) Error() string {
	return string(e)
}
