package domain

import (
	"strings"
	"testing"
)

func TestOutputFormatter_FormatForUser(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		result   string
		metadata map[string]any
		verbose  bool
		contains []string // Strings that should be present in output
		notContains []string // Strings that should NOT be present
	}{
		{
			name:     "subagent compact mode",
			toolName: "subagent",
			result:   "Long detailed output with all task results...",
			metadata: map[string]any{
				"total_tasks":      3,
				"success_count":    3,
				"failure_count":    0,
				"total_tokens":     1500,
				"total_tool_calls": 12,
			},
			verbose:  false,
			contains: []string{"3/3 tasks", "1500", "12"},
			notContains: []string{"Long detailed output"},
		},
		{
			name:     "subagent verbose mode",
			toolName: "subagent",
			result:   "Long detailed output with all task results...",
			metadata: map[string]any{
				"total_tasks":      3,
				"success_count":    3,
				"total_tokens":     1500,
				"total_tool_calls": 12,
			},
			verbose:  true,
			contains: []string{"Long detailed output"},
		},
		{
			name:     "file read compact",
			toolName: "file_read",
			result:   strings.Repeat("line content that is very long and will exceed 500 characters when repeated many times to trigger truncation\n", 10),
			metadata: map[string]any{},
			verbose:  false,
			contains: []string{"Read", "lines", "..."},
		},
		{
			name:     "bash always shows full",
			toolName: "bash",
			result:   "command output here",
			metadata: map[string]any{},
			verbose:  false,
			contains: []string{"command output here"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := NewOutputFormatter(tt.verbose)
			output := formatter.FormatForUser(tt.toolName, tt.result, tt.metadata)

			for _, want := range tt.contains {
				if !strings.Contains(output, want) {
					t.Errorf("FormatForUser() output should contain %q, got:\n%s", want, output)
				}
			}

			for _, notWant := range tt.notContains {
				if strings.Contains(output, notWant) {
					t.Errorf("FormatForUser() output should NOT contain %q, got:\n%s", notWant, output)
				}
			}
		})
	}
}

func TestOutputFormatter_FormatForLLM(t *testing.T) {
	formatter := NewOutputFormatter(false)

	result := "Full detailed output"
	metadata := map[string]any{"key": "value"}

	// LLM always gets full output regardless of tool
	output := formatter.FormatForLLM("subagent", result, metadata)

	if output != result {
		t.Errorf("FormatForLLM() should return full result, got %q, want %q", output, result)
	}
}

func TestOutputFormatter_SubagentWithFailures(t *testing.T) {
	formatter := NewOutputFormatter(false)

	metadata := map[string]any{
		"total_tasks":      5,
		"success_count":    3,
		"failure_count":    2,
		"total_tokens":     1000,
		"total_tool_calls": 8,
	}

	output := formatter.FormatForUser("subagent", "result", metadata)

	if !strings.Contains(output, "3/5") {
		t.Errorf("Should show 3/5 tasks completed, got: %s", output)
	}

	if !strings.Contains(output, "2 task(s) failed") {
		t.Errorf("Should show failure count, got: %s", output)
	}
}
