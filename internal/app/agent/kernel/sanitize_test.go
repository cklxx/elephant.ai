package kernel

import (
	"testing"
)

// TestSanitizeRuntimeSummary locks the K-05 fix:
//   - pure LLM noise lines are stripped (per-line, prefix/contains match)
//   - non-noise lines following a noisy line are preserved
//   - code fence markers (``` lines) stripped; content between fences preserved
//   - "## Execution Summary" headers are preserved (legit agent output)
//   - natural-language "tool calls" mentions are preserved
//   - precise JSON schema keys ("tool_call", "tool_calls") are stripped
func TestSanitizeRuntimeSummary(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty input returns empty",
			input: "",
			want:  "",
		},
		{
			name:  "whitespace-only returns empty",
			input: "   \n\t\n  ",
			want:  "",
		},
		{
			// "Thinking (previous):" line is stripped; subsequent lines are kept
			name:  "strips thinking prefix line only",
			input: "Thinking (previous):\nSome internal reasoning here.\nActual summary line.",
			want:  "Some internal reasoning here. Actual summary line.",
		},
		{
			// "Reasoning:" line is stripped; subsequent lines are kept
			name:  "strips reasoning prefix line only",
			input: "Reasoning:\nLet me think step by step.\nResult: done.",
			want:  "Let me think step by step. Result: done.",
		},
		{
			// ``` fence lines are stripped; content lines between fences are kept
			name:  "strips code fence markers preserves content",
			input: "```json\n{\"key\": \"value\"}\n```\nSummary: all good.",
			want:  "{\"key\": \"value\"} Summary: all good.",
		},
		{
			name:  "strips assistant to= XML artifacts",
			input: "<assistant to=functions>call here</assistant>\nLegit content.",
			want:  "Legit content.",
		},
		{
			name:  "strips recipient_name artifacts",
			input: "recipient_name: some_function\nLegit content.",
			want:  "Legit content.",
		},
		{
			name:  "strips tool_uses schema leakage",
			input: "tool_uses: [{\"name\": \"shell_exec\"}]\nLegit content.",
			want:  "Legit content.",
		},
		{
			name:  "strips \"tool_call\" JSON key",
			input: "{\"tool_call\": \"shell_exec\"}\nLegit content.",
			want:  "Legit content.",
		},
		{
			name:  "strips \"tool_calls\" JSON key",
			input: "{\"tool_calls\": []}\nLegit content.",
			want:  "Legit content.",
		},
		{
			// K-05: broad "tool call" substring must NOT strip natural-language bullets
			name:  "preserves natural-language tool calls mention",
			input: "- dispatched tasks (tool calls) to complete\n- wrote audit file",
			want:  "- dispatched tasks (tool calls) to complete - wrote audit file",
		},
		{
			// K-05: "## Execution Summary" must NOT be stripped
			name:  "preserves execution summary header",
			input: "## Execution Summary\n- K-03: fixed\n- K-04: fixed",
			want:  "## Execution Summary - K-03: fixed - K-04: fixed",
		},
		{
			name:  "preserves plain summary content",
			input: "Task completed successfully. Wrote 3 files.",
			want:  "Task completed successfully. Wrote 3 files.",
		},
		{
			// "Thinking (previous):" line stripped; content lines kept;
			// ``` fence markers stripped but inner "code block" line kept
			name:  "mixed noise markers and valid content",
			input: "Thinking (previous):\nAll internal.\n## Execution Summary\n- step 1 done\n```\ncode block\n```\n- step 2 done",
			want:  "All internal. ## Execution Summary - step 1 done code block - step 2 done",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeRuntimeSummary(tc.input)
			if got != tc.want {
				t.Errorf("\ninput: %q\ngot:   %q\nwant:  %q", tc.input, got, tc.want)
			}
		})
	}
}
