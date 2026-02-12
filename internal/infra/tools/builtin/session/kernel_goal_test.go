package session

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"alex/internal/domain/agent/ports"
)

func TestKernelGoalSetAndGet(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	tool := NewKernelGoal()

	setResult, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:   "call-set",
		Name: "kernel_goal",
		Arguments: map[string]any{
			"action":    "set",
			"kernel_id": "default",
			"goal":      "# Goal\n- finish one real outcome",
		},
	})
	if err != nil {
		t.Fatalf("set execute returned error: %v", err)
	}
	if setResult.Error != nil {
		t.Fatalf("set result error: %v", setResult.Error)
	}
	if setResult.Content != "kernel goal updated" {
		t.Fatalf("unexpected set content: %q", setResult.Content)
	}

	path := filepath.Join(home, ".alex", "kernel", "default", "GOAL.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected goal file written: %v", err)
	}
	if got := string(data); got == "" {
		t.Fatal("goal file should not be empty")
	}

	getResult, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:   "call-get",
		Name: "kernel_goal",
		Arguments: map[string]any{
			"action":    "get",
			"kernel_id": "default",
		},
	})
	if err != nil {
		t.Fatalf("get execute returned error: %v", err)
	}
	if getResult.Error != nil {
		t.Fatalf("get result error: %v", getResult.Error)
	}
	if got := getResult.Content; got == "" || got == "(empty)" {
		t.Fatalf("unexpected get content: %q", got)
	}
}

func TestKernelGoalGetMissingReturnsEmpty(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	tool := NewKernelGoal()
	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:   "call-missing",
		Name: "kernel_goal",
		Arguments: map[string]any{
			"action": "get",
		},
	})
	if err != nil {
		t.Fatalf("execute returned error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("result error: %v", result.Error)
	}
	if result.Content != "(empty)" {
		t.Fatalf("unexpected content: %q", result.Content)
	}
}

func TestKernelGoalRejectsInvalidKernelID(t *testing.T) {
	tool := NewKernelGoal()
	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:   "call-invalid",
		Name: "kernel_goal",
		Arguments: map[string]any{
			"action":    "set",
			"kernel_id": "../escape",
			"goal":      "x",
		},
	})
	if err != nil {
		t.Fatalf("execute returned error: %v", err)
	}
	if result == nil || result.Error == nil {
		t.Fatalf("expected tool error for invalid kernel_id, got %#v", result)
	}
	if !strings.Contains(result.Content, "invalid kernel_id") {
		t.Fatalf("unexpected error content: %q", result.Content)
	}
}
